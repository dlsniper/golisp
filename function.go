// Copyright 2014 SteelSeries ApS.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This package implements a basic LISP interpretor for embedding in a go program for scripting.
// This file implements user defined functions.

package golisp

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

type FunctonTypeSignature struct {
	ArgumentTypes []uint32
	ReturnType    uint32
}

type Function struct {
	Name             string
	Params           *Data
	VarArgs          bool
	RequiredArgCount int
	TypeSignature    *FunctonTypeSignature
	DocString        string
	Body             *Data
	Env              *SymbolTableFrame
	DebugOnEntry     bool
	SlotFunction     int32
	ParentProcess    *Process
}

var functionTypeSignatures map[string]*FunctonTypeSignature = make(map[string]*FunctonTypeSignature, 20)
var functionTypeSignaturesMutex sync.Mutex

func computeRequiredArgumentCount(args *Data) (requiredArgumentCount int, varArgs bool) {
	requiredArgumentCount = 0
	varArgs = false
	for a := args; NotNilP(a); a = Cdr(a) {
		if SymbolP(a) {
			varArgs = true
			return
		} else {
			requiredArgumentCount += 1
		}
	}
	return
}

func AddTypesForFunction(name string, argTypes []uint32, retType uint32) {
	functionTypeSignaturesMutex.Lock()
	functionTypeSignatures[name] = &FunctonTypeSignature{ArgumentTypes: argTypes, ReturnType: retType}
	functionTypeSignaturesMutex.Unlock()
}

func MakeFunction(name string, params *Data, doc string, body *Data, parentEnv *SymbolTableFrame) *Function {
	requiredArgs, varArgs := computeRequiredArgumentCount(params)
	functionTypeSignaturesMutex.Lock()
	f := &Function{Name: name, Params: params, VarArgs: varArgs, RequiredArgCount: requiredArgs, TypeSignature: functionTypeSignatures[name], DocString: doc, Body: body, Env: parentEnv, SlotFunction: 0}
	functionTypeSignatures[name] = nil
	functionTypeSignaturesMutex.Unlock()
	return f
}

func (self *Function) String() string {
	return fmt.Sprintf("<func: %s>", self.Name)
}

func countOnes(num uint32) (count int) {
	count = 0
	for i := 0; i < 32; i += 1 {
		if (num & 1) != 0 {
			count += 1
		}
		num >>= 1
	}
	return
}

func findBiggestType(types []string) (biggestCount int, biggestType string) {
	for _, name := range types {
		count := countOnes(TypeMap[name])
		if count > biggestCount {
			biggestCount = count
			biggestType = name
		}
	}
	return
}

func reduceTypes(types []string, biggestCount int, biggestType string) []string {
	minimalTypes := make([]string, 0, 3)
	minimalTypes = append(minimalTypes, biggestType)
	biggestBits := TypeMap[biggestType]
	for _, t := range types {
		if (biggestBits & TypeMap[t]) == 0 {
			minimalTypes = append(minimalTypes, t)
		}
	}
	return minimalTypes
}

func typeNameFor(value uint32, sep string) string {
	if value == 0xFFFFFFFF {
		return "anytype"
	}
	potentialTypes := make([]string, 0, 3)
	for name, mask := range TypeMap {
		if name != "anytype" && mask&value != 0 {
			potentialTypes = append(potentialTypes, name)
		}
	}

	biggestCount, biggestType := findBiggestType(potentialTypes)
	types := reduceTypes(potentialTypes, biggestCount, biggestType)
	return strings.Join(types, sep)
}

func (self *Function) MakeTypeSpec() *Data {
	// (arg1type ... argNtype -> returntype)
	argTypes := make([]*Data, 0, len(self.TypeSignature.ArgumentTypes))
	for _, argType := range self.TypeSignature.ArgumentTypes {
		t := typeNameFor(argType, "|")
		var typeObj *Data
		if strings.Contains(t, "|") {
			typeObj = StringWithValue(t)
		} else {
			typeObj = SymbolWithName(t)
		}
		argTypes = append(argTypes, typeObj)
	}
	formalsTypes := ArrayToList(argTypes)
	var returnTypeObj *Data
	returnTypeString := typeNameFor(self.TypeSignature.ReturnType, "|")
	if strings.Contains(returnTypeString, "|") {
		returnTypeObj = StringWithValue(returnTypeString)
	} else {
		returnTypeObj = SymbolWithName(returnTypeString)
	}
	result, _ := Flatten(InternalMakeList(formalsTypes, Intern("->"), returnTypeObj))
	return result
}

func (self *Function) makeLocalBindings(args *Data, argEnv *SymbolTableFrame, localEnv *SymbolTableFrame, eval bool) (err error) {
	if self.VarArgs {
		if Length(args) < self.RequiredArgCount {
			return errors.New(fmt.Sprintf("%s expected at least %d parameters, received %d.", self.Name, self.RequiredArgCount, Length(args)))
		}
	} else {
		if Length(args) != self.RequiredArgCount {
			return errors.New(fmt.Sprintf("%s expected %d parameters, received %d.", self.Name, self.RequiredArgCount, Length(args)))
		}
	}

	var argValue *Data
	var accumulatingParam *Data = nil
	accumulatedArgs := make([]*Data, 0)
	for i, p, a := 0, self.Params, args; NotNilP(a); i, a = i+1, Cdr(a) {
		if eval {
			argValue, err = Eval(Car(a), argEnv)
			if err != nil {
				return
			}
		} else {
			argValue = Car(a)
		}

		if self.TypeSignature != nil && NotNilP(argValue) && self.TypeSignature.ArgumentTypes[i]&TypeOf(argValue) == 0 {
			return errors.New(fmt.Sprintf("%s argument %d has the wrong type, expected %s but was given %s", self.Name, i, typeNameFor(self.TypeSignature.ArgumentTypes[i], " or "), typeNameFor(TypeOf(argValue), " or ")))
		}

		if SymbolP(p) {
			accumulatingParam = p
		}
		if accumulatingParam != nil {
			accumulatedArgs = append(accumulatedArgs, argValue)
		} else {
			_, err = localEnv.BindLocallyTo(Car(p), argValue)
			if err != nil {
				return
			}
		}
		if accumulatingParam == nil {
			p = Cdr(p)
		}
	}
	if accumulatingParam != nil {
		_, err = localEnv.BindLocallyTo(accumulatingParam, ArrayToList(accumulatedArgs))
		if err != nil {
			return
		}
	}
	return nil
}

func (self *Function) internalApply(args *Data, argEnv *SymbolTableFrame, frame *FrameMap, eval bool) (result *Data, err error) {
	localEnv := NewSymbolTableFrameBelowWithFrame(self.Env, frame, self.Name)
	localEnv.Previous = argEnv
	selfSym := Intern("self")
	if frame != nil {
		_, err = localEnv.BindLocallyTo(selfSym, FrameWithValue(frame))
		if err != nil {
			return
		}
	} else if atomic.LoadInt32(&self.SlotFunction) == 1 {
		selfBinding, found := argEnv.findBindingInLocalFrameFor(selfSym)
		if found {
			_, err = localEnv.BindLocallyTo(selfSym, selfBinding.Val)
			if err != nil {
				return
			}
		}
	}

	parentProcSym := Intern("parentProcess")
	if self.ParentProcess != nil {
		procObj := ObjectWithTypeAndValue("Process", unsafe.Pointer(self.ParentProcess))
		_, err = localEnv.BindLocallyTo(parentProcSym, procObj)
		if err != nil {
			return
		}
	}

	err = self.makeLocalBindings(args, argEnv, localEnv, eval)
	if err != nil {
		return
	}

	localGuid := atomic.AddInt64(&ProfileGUID, 1) - 1

	ProfileEnter("func", self.Name, localGuid)

	for s := self.Body; NotNilP(s); s = Cdr(s) {
		result, err = Eval(Car(s), localEnv)
		if err != nil {
			result, err = nil, errors.New(fmt.Sprintf("In '%s': %s", self.Name, err))
			break
		}
	}

	if err == nil {
		if self.TypeSignature != nil && self.TypeSignature.ReturnType&TypeOf(result) == 0 {
			result, err = nil, errors.New(fmt.Sprintf("%s returns the wrong type, expected %s but returned %s", self.Name, typeNameFor(self.TypeSignature.ReturnType, " or "), typeNameFor(TypeOf(result), " or ")))
		}
	}

	ProfileExit("func", self.Name, localGuid)

	return
}

func (self *Function) Apply(args *Data, argEnv *SymbolTableFrame) (result *Data, err error) {
	return self.internalApply(args, argEnv, nil, true)
}

func (self *Function) ApplyWithFrame(args *Data, argEnv *SymbolTableFrame, frame *FrameMap) (result *Data, err error) {
	return self.internalApply(args, argEnv, frame, true)
}

func (self *Function) ApplyWithoutEval(args *Data, argEnv *SymbolTableFrame) (result *Data, err error) {
	return self.internalApply(args, argEnv, nil, false)
}

func (self *Function) ApplyWithoutEvalWithFrame(args *Data, argEnv *SymbolTableFrame, frame *FrameMap) (result *Data, err error) {
	return self.internalApply(args, argEnv, frame, false)
}

func (self *Function) ApplyOveriddingEnvironment(args *Data, argEnv *SymbolTableFrame) (result *Data, err error) {
	localEnv := NewSymbolTableFrameBelow(argEnv, self.Name)
	err = self.makeLocalBindings(args, argEnv, localEnv, true)
	if err != nil {
		return
	}

	localGuid := atomic.AddInt64(&ProfileGUID, 1) - 1

	ProfileEnter("func", self.Name, localGuid)

	for s := self.Body; NotNilP(s); s = Cdr(s) {
		result, err = Eval(Car(s), localEnv)
		if err != nil {
			result, err = nil, errors.New(fmt.Sprintf("In '%s': %s", self.Name, err))
			break
		}
	}

	ProfileExit("func", self.Name, localGuid)

	return
}

func (self *Function) ExtendEnv(args *Data, argEnv *SymbolTableFrame, frame *FrameMap) (localEnv *SymbolTableFrame, err error) {
	localEnv = NewSymbolTableFrameBelowWithFrame(self.Env, frame, self.Name)
	localEnv.Previous = argEnv
	selfSym := Intern("self")
	if frame != nil {
		localEnv.BindLocallyTo(selfSym, FrameWithValue(frame))
	} else if atomic.LoadInt32(&self.SlotFunction) == 1 {
		selfBinding, found := argEnv.findBindingInLocalFrameFor(selfSym)
		if found {
			localEnv.BindLocallyTo(selfSym, selfBinding.Val)
		}
	}

	parentProcSym := Intern("parentProcess")
	if self.ParentProcess != nil {
		procObj := ObjectWithTypeAndValue("Process", unsafe.Pointer(self.ParentProcess))
		localEnv.BindLocallyTo(parentProcSym, procObj)
	}

	err = self.makeLocalBindings(args, argEnv, localEnv, false)

	return
}
