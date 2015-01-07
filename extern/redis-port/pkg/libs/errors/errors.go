// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package errors

import (
	"errors"
	"fmt"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/trace"
)

var TraceEnabled = true

type Error struct {
	Stack trace.Stack
	Cause error
}

func (e *Error) Error() string {
	return e.Cause.Error()
}

func Static(s string) error {
	return errors.New(s)
}

func New(s string) error {
	err := errors.New(s)
	if !TraceEnabled {
		return err
	}
	return &Error{
		Stack: trace.TraceN(1, 32),
		Cause: err,
	}
}

func Trace(err error) error {
	if err == nil || !TraceEnabled {
		return err
	}
	_, ok := err.(*Error)
	if ok {
		return err
	}
	return &Error{
		Stack: trace.TraceN(1, 32),
		Cause: err,
	}
}

func Errorf(format string, v ...interface{}) error {
	err := fmt.Errorf(format, v...)
	if !TraceEnabled {
		return err
	}
	return &Error{
		Stack: trace.TraceN(1, 32),
		Cause: err,
	}
}

func ErrorStack(err error) trace.Stack {
	if err == nil {
		return nil
	}
	e, ok := err.(*Error)
	if ok {
		return e.Stack
	}
	return nil
}

func ErrorCause(err error) error {
	for err != nil {
		e, ok := err.(*Error)
		if ok {
			err = e.Cause
		} else {
			return err
		}
	}
	return nil
}

func Equal(err1, err2 error) bool {
	e1 := ErrorCause(err1)
	e2 := ErrorCause(err2)
	if e1 == e2 {
		return true
	}
	if e1 == nil || e2 == nil {
		return e1 == e2
	}
	return e1.Error() == e2.Error()
}

func NotEqual(err1, err2 error) bool {
	return !Equal(err1, err2)
}
