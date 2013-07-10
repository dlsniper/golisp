// Copyright 2013 SteelSeries ApS. All rights reserved.
// No license is given for the use of this source code.

// This package impliments a basic LISP interpretor for embedding in a go program for scripting.
// This file provides a repl

package golisp

import (
    "bufio"
    "fmt"
    "os"
)

func Repl() {
    for true {
        for true {
            input := *ReadLine(">")
            if input != "" {
                AddHistory(input)
                code, err := Parse(input)
                if err != nil {
                    fmt.Printf("Error: %s\n", err)
                } else {
                    d, err := Eval(code, Global)
                    if err != nil {
                        fmt.Printf("Error in evaluation: %s\n", err)
                    } else {
                        fmt.Printf("==> %s\n", String(d))
                    }
                }
            }
        }
    }
}