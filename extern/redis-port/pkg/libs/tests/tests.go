// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package tests

import (
	"fmt"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/trace"
)

func Fatalf(t *testing.T, format string, args ...interface{}) {
	t.Fatalf("%s\n%s", fmt.Sprintf(format, args...), trace.TraceN(1, 32))
}

func AssertNoError(t *testing.T, err error) {
	if err == nil {
		return
	}
	stack := errors.ErrorStack(err)
	if stack == nil {
		stack = trace.TraceN(1, 32)
	}
	t.Fatalf("%s\n%s", err, stack)
}

func Assert(t *testing.T, b bool) {
	if b {
		return
	}
	t.Fatalf("assertion failed\n%s", trace.TraceN(1, 32))
}
