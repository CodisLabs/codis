// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/trace"
)

func Panic(format string, v ...interface{}) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "[panic]: "+format, v...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintf(&b, "\n")
	}
	fmt.Fprintf(&b, trace.TraceN(1, 32).StringWithIndent(1))
	log.Fatal(b.String())
}

func ErrorPanic(err error, format string, v ...interface{}) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "[panic]: "+format, v...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintf(&b, "\n")
	}
	fmt.Fprintf(&b, trace.TraceN(1, 32).StringWithIndent(1))
	fmt.Fprintf(&b, "[error]: %s\n", err.Error())
	if stack := errors.ErrorStack(err); stack != nil {
		fmt.Fprintf(&b, stack.StringWithIndent(1))
	}
	log.Fatal(b.String())
}

func AssertNoError(err error) {
	if err == nil {
		return
	}
	ErrorPanic(err, "error happens, assertion failed")
}

func Assert(b bool) {
	if b {
		return
	}
	Panic("assertion failed")
}
