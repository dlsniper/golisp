// Copyright 2014 SteelSeries ApS.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This package implements a basic LISP interpretor for embedding in a go program for scripting.
// This file contains Raspberry Pi related primitive functions.

package golisp

import (
	"fmt"
	"github.com/mrmorphic/hwio"
)

func RegisterPiPrimitives() {
	Global.BindTo(Intern("gpio:INPUT"), IntegerWithValue(int64(hwio.INPUT)))
	Global.BindTo(Intern("gpio:OUTPUT"), IntegerWithValue(int64(hwio.OUTPUT)))
	Global.BindTo(Intern("gpio:INPUT_PULLUP"), IntegerWithValue(int64(hwio.INPUT_PULLUP)))
	Global.BindTo(Intern("gpio:INPUT_PULLDOWN"), IntegerWithValue(int64(hwio.INPUT_PULLDOWN)))
	Global.BindTo(Intern("gpio:HIGH"), IntegerWithValue(int64(hwio.HIGH)))
	Global.BindTo(Intern("gpio:LOW"), IntegerWithValue(int64(hwio.LOW)))

	MakePrimitiveFunction("gpio:get-pin", "1|2", GPIOGetPinImpl)
	MakePrimitiveFunction("gpio:set-pin-mode", "2", GPIOSetPinModeImpl)
	MakePrimitiveFunction("gpio:close-pin", "1", GPIOClosePinImpl)
	MakePrimitiveFunction("gpio:digital-write", "2", GPIODigitalWriteImpl)
	MakePrimitiveFunction("gpio:digital-read", "1", GPIODigitalReadImpl)
}

func GPIOGetPinImpl(args *Data, env *SymbolTableFrame) (result *Data, err error) {
	if !StringP(First(args)) {
		err = ProcessError(fmt.Sprintf("gpio:get-pin expected a pin name as its first argument but received %s.", String(First(args))), env)
		return
	}
	pinName := StringValue(First(args))

	var pin hwio.Pin

	if Length(args) == 1 {
		pin, err = hwio.GetPin(pinName)
		if err != nil {
			return
		}
	} else {
		modeObj := Second(args)
		if !IntegerP(modeObj) {
			err = ProcessError(fmt.Sprintf("gpio:get-pin expects an integer as it's second argument but received %s.", String(modeObj)), env)
			return
		}
		mode := hwio.PinIOMode(int(IntegerValue(modeObj)))
		if mode < hwio.INPUT || mode > hwio.INPUT_PULLDOWN {
			err = ProcessError(fmt.Sprintf("gpio:get-pin expected a valid pin mode as its second argument but received %d.", mode), env)
		}
		pin, err = hwio.GetPinWithMode(pinName, mode)
		if err != nil {
			return
		}
	}

	result = IntegerWithValue(int64(pin))
	return
}

func GPIOSetPinModeImpl(args *Data, env *SymbolTableFrame) (result *Data, err error) {
	pinObj := First(args)
	if !IntegerP(pinObj) {
		err = ProcessError(fmt.Sprintf("gpio:set-pin-mode expected a pin number as its first argument but received %s.", String(pinObj)), env)
		return
	}
	pin := hwio.Pin(int(IntegerValue(pinObj)))

	modeObj := Second(args)
	if !IntegerP(modeObj) {
		err = ProcessError(fmt.Sprintf("gpio:set-pin-mode expects an integer as it's second argument but received %s.", String(modeObj)), env)
		return
	}
	mode := hwio.PinIOMode(int(IntegerValue(modeObj)))
	if mode < hwio.INPUT || mode > hwio.INPUT_PULLDOWN {
		err = ProcessError(fmt.Sprintf("gpio:set-pin-mode expected a valid pin mode as its second argument but received %d.", mode), env)
	}

	hwio.PinMode(pin, mode)
	result = LispTrue
	return
}

func GPIOClosePinImpl(args *Data, env *SymbolTableFrame) (result *Data, err error) {
	pinObj := First(args)
	if !IntegerP(pinObj) {
		err = ProcessError(fmt.Sprintf("gpio:close-pin expected a pin number as its first argument but received %s.", String(pinObj)), env)
		return
	}
	pin := hwio.Pin(int(IntegerValue(pinObj)))
	err = hwio.ClosePin(pin)
	if err != nil {
		return
	}
	result = LispTrue
	return
}

func GPIODigitalWriteImpl(args *Data, env *SymbolTableFrame) (result *Data, err error) {
	pinObj := First(args)
	if !IntegerP(pinObj) {
		err = ProcessError(fmt.Sprintf("gpio:digital-write expected a pin number as its first argument but received %s.", String(pinObj)), env)
		return
	}
	pin := hwio.Pin(int(IntegerValue(pinObj)))

	valueObj := Second(args)
	value := 0
	if IntegerP(valueObj) {
		value = int(IntegerValue(valueObj))
		if value != hwio.HIGH && value != hwio.LOW {
			err = ProcessError(fmt.Sprintf("gpio:digital-write expected a valid value as its second argument but received %d.", value), env)
			return
		}
	} else if BooleanP(valueObj) {
		if BooleanValue(valueObj) {
			value = 1
		} else {
			value = 0
		}
	} else {
		err = ProcessError(fmt.Sprintf("gpio:digital-write expected %d, %d, #f, or #t as its second argument but received %s.", hwio.LOW, hwio.HIGH, String(valueObj)), env)
		return
	}

	hwio.DigitalWrite(pin, int(value))
	result = LispTrue
	return
}

func GPIODigitalReadImpl(args *Data, env *SymbolTableFrame) (result *Data, err error) {
	pinObj := First(args)
	if !IntegerP(pinObj) {
		err = ProcessError(fmt.Sprintf("gpio:digital-read expected a pin number as its first argument but received %s.", String(pinObj)), env)
		return
	}
	pin := hwio.Pin(int(IntegerValue(pinObj)))

	value, err := hwio.DigitalRead(pin)
	if err != nil {
		return
	}

	result = BooleanWithValue(value == hwio.HIGH)
	return
}