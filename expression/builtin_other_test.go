// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package expression

import (
	. "github.com/pingcap/check"
	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/mysql"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/hack"
	"math"
)

func (s *testEvaluatorSuite) TestInFunc(c *C) {
	fc := funcs[ast.In]
	testCases := []struct {
		args []interface{}
		res  interface{}
	}{
		{[]interface{}{1, 1, 2, 3}, int64(1)},
		{[]interface{}{1, 0, 2, 3}, int64(0)},
		{[]interface{}{1, nil, 2, 3}, nil},
		{[]interface{}{nil, nil, 2, 3}, nil},
		{[]interface{}{uint64(0), 0, 2, 3}, int64(1)},
		{[]interface{}{uint64(math.MaxUint64), uint64(math.MaxUint64), 2, 3}, int64(1)},
		{[]interface{}{-1, uint64(math.MaxUint64), 2, 3}, int64(0)},
		{[]interface{}{uint64(math.MaxUint64), -1, 2, 3}, int64(0)},
		{[]interface{}{1, 0, 2, 3}, int64(0)},
		{[]interface{}{1.1, 1.2, 1.3}, int64(0)},
		{[]interface{}{1.1, 1.1, 1.2, 1.3}, int64(1)},
		{[]interface{}{"1.1", "1.1", "1.2", "1.3"}, int64(1)},
		{[]interface{}{"1.1", hack.Slice("1.1"), "1.2", "1.3"}, int64(1)},
		{[]interface{}{hack.Slice("1.1"), "1.1", "1.2", "1.3"}, int64(1)},
	}
	for _, tc := range testCases {
		fn, err := fc.getFunction(s.ctx, s.datumsToConstants(types.MakeDatums(tc.args...)))
		c.Assert(err, IsNil)
		d, err := evalBuiltinFunc(fn, chunk.MutRowFromDatums(types.MakeDatums(tc.args...)).ToRow())
		c.Assert(err, IsNil)
		c.Assert(d.GetValue(), Equals, tc.res, Commentf("%v", types.MakeDatums(tc.args)))
	}
}

func (s *testEvaluatorSuite) TestRowFunc(c *C) {
	fc := funcs[ast.RowFunc]
	_, err := fc.getFunction(s.ctx, s.datumsToConstants(types.MakeDatums([]interface{}{"1", 1.2, true, 120}...)))
	c.Assert(err, IsNil)
}

func (s *testEvaluatorSuite) TestSetVar(c *C) {
	fc := funcs[ast.SetVar]
	testCases := []struct {
		args []interface{}
		res  interface{}
	}{
		{[]interface{}{"a", "12"}, "12"},
		{[]interface{}{"b", "34"}, "34"},
		{[]interface{}{"c", nil}, ""},
		{[]interface{}{"c", "ABC"}, "ABC"},
		{[]interface{}{"c", "dEf"}, "dEf"},
	}
	for _, tc := range testCases {
		fn, err := fc.getFunction(s.ctx, s.datumsToConstants(types.MakeDatums(tc.args...)))
		c.Assert(err, IsNil)
		d, err := evalBuiltinFunc(fn, chunk.MutRowFromDatums(types.MakeDatums(tc.args...)).ToRow())
		c.Assert(err, IsNil)
		c.Assert(d.GetString(), Equals, tc.res)
		if tc.args[1] != nil {
			key, ok := tc.args[0].(string)
			c.Assert(ok, Equals, true)
			val, ok := tc.res.(string)
			c.Assert(ok, Equals, true)
			c.Assert(s.ctx.GetSessionVars().Users[key], Equals, val)
		}
	}
}

func (s *testEvaluatorSuite) TestGetVar(c *C) {
	fc := funcs[ast.GetVar]

	sessionVars := []struct {
		key string
		val string
	}{
		{"a", "???"},
		{"b", "?????????chuan"},
		{"c", ""},
	}
	for _, kv := range sessionVars {
		s.ctx.GetSessionVars().Users[kv.key] = kv.val
	}

	testCases := []struct {
		args []interface{}
		res  interface{}
	}{
		{[]interface{}{"a"}, "???"},
		{[]interface{}{"b"}, "?????????chuan"},
		{[]interface{}{"c"}, ""},
		{[]interface{}{"d"}, ""},
	}
	for _, tc := range testCases {
		fn, err := fc.getFunction(s.ctx, s.datumsToConstants(types.MakeDatums(tc.args...)))
		c.Assert(err, IsNil)
		d, err := evalBuiltinFunc(fn, chunk.MutRowFromDatums(types.MakeDatums(tc.args...)).ToRow())
		c.Assert(err, IsNil)
		c.Assert(d.GetString(), Equals, tc.res)
	}
}

func (s *testEvaluatorSuite) TestValues(c *C) {
	origin := s.ctx.GetSessionVars().StmtCtx.InInsertStmt
	s.ctx.GetSessionVars().StmtCtx.InInsertStmt = false
	defer func() {
		s.ctx.GetSessionVars().StmtCtx.InInsertStmt = origin
	}()

	fc := &valuesFunctionClass{baseFunctionClass{ast.Values, 0, 0}, 1, types.NewFieldType(mysql.TypeVarchar)}
	_, err := fc.getFunction(s.ctx, s.datumsToConstants(types.MakeDatums("")))
	c.Assert(err, ErrorMatches, "*Incorrect parameter count in the call to native function 'values'")

	sig, err := fc.getFunction(s.ctx, s.datumsToConstants(types.MakeDatums()))
	c.Assert(err, IsNil)

	ret, err := evalBuiltinFunc(sig, chunk.Row{})
	c.Assert(err, IsNil)
	c.Assert(ret.IsNull(), IsTrue)

	s.ctx.GetSessionVars().CurrInsertValues = chunk.MutRowFromDatums(types.MakeDatums("1")).ToRow()
	ret, err = evalBuiltinFunc(sig, chunk.Row{})
	c.Assert(err, IsNil)
	c.Assert(ret.IsNull(), IsTrue)

	currInsertValues := types.MakeDatums("1", "2")
	s.ctx.GetSessionVars().StmtCtx.InInsertStmt = true
	s.ctx.GetSessionVars().CurrInsertValues = chunk.MutRowFromDatums(currInsertValues).ToRow()
	ret, err = evalBuiltinFunc(sig, chunk.Row{})
	c.Assert(err, IsNil)

	cmp, err := ret.CompareDatum(nil, &currInsertValues[1])
	c.Assert(err, IsNil)
	c.Assert(cmp, Equals, 0)
}

func (s *testEvaluatorSuite) TestSetVarFromColumn(c *C) {
	// Construct arguments.
	argVarName := &Constant{
		Value:   types.NewStringDatum("a"),
		RetType: &types.FieldType{Tp: mysql.TypeVarString, Flen: 20},
	}
	argCol := &Column{
		RetType: &types.FieldType{Tp: mysql.TypeVarString, Flen: 20},
		Index:   0,
	}

	// Construct SetVar function.
	funcSetVar, err := NewFunction(
		s.ctx,
		ast.SetVar,
		&types.FieldType{Tp: mysql.TypeVarString, Flen: 20},
		[]Expression{argVarName, argCol}...,
	)
	c.Assert(err, IsNil)

	// Construct input and output Chunks.
	inputChunk := chunk.NewChunkWithCapacity([]*types.FieldType{argCol.RetType}, 1)
	inputChunk.AppendString(0, "a")
	outputChunk := chunk.NewChunkWithCapacity([]*types.FieldType{argCol.RetType}, 1)

	// Evaluate the SetVar function.
	err = evalOneCell(s.ctx, funcSetVar, inputChunk.GetRow(0), outputChunk, 0)
	c.Assert(err, IsNil)
	c.Assert(outputChunk.GetRow(0).GetString(0), Equals, "a")

	// Change the content of the underlying Chunk.
	inputChunk.Reset()
	inputChunk.AppendString(0, "b")

	// Check whether the user variable changed.
	sessionVars := s.ctx.GetSessionVars()
	sessionVars.UsersLock.RLock()
	defer sessionVars.UsersLock.RUnlock()
	c.Assert(sessionVars.Users["a"], Equals, "a")
}
