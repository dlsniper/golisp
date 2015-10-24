// Copyright 2014 SteelSeries ApS.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This package implements a basic LISP interpretor for embedding in a go program for scripting.
// This file tests built-in primitive functions.

package golisp

import (
	. "gopkg.in/check.v1"
)

type MacrosSuite struct {
	OldVectorizationFlag bool
}

var _ = Suite(&MacrosSuite{})

func (s *MacrosSuite) SetUpSuite(c *C) {
	InitLisp()
	s.OldVectorizationFlag = UseVectorization
	UseVectorization = true
}

func (s *MacrosSuite) TearDownSuite(c *C) {
	UseVectorization = s.OldVectorizationFlag
}

func (s *MacrosSuite) TestNoUnquoting(c *C) {
	code, _ := Parse("`(+ a 1)")
	result, err := Eval(code, Global)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	c.Assert(String(result), Equals, "(+ a 1)")
}

func (s *MacrosSuite) TestUnquotingInteger(c *C) {
	code, _ := Parse("`(+ a ,1)")
	result, err := Eval(code, Global)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	c.Assert(String(result), Equals, "(+ a 1)")
}

func (s *MacrosSuite) TestUnquotingSymbol(c *C) {
	Global.BindTo(SymbolWithName("a"), IntegerWithValue(5))
	code, _ := Parse("`(+ ,a 1)")
	result, err := Eval(code, Global)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	c.Assert(String(result), Equals, "(+ 5 1)")
}

func (s *MacrosSuite) TestUnquotingExpression(c *C) {
	Global.BindTo(SymbolWithName("a"), IntegerWithValue(5))
	code, _ := Parse("`(+ ,(+ a 1) 1)")
	result, err := Eval(code, Global)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	c.Assert(String(result), Equals, "(+ 6 1)")
}

func (s *MacrosSuite) TestUnquoteSplicing(c *C) {
	Global.BindTo(SymbolWithName("a"), IntegerWithValue(5))
	code, _ := Parse("`(+ ,@(list 1 2 3) 1)")
	result, err := Eval(code, Global)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	c.Assert(String(result), Equals, "(+ 1 2 3 1)")
}
