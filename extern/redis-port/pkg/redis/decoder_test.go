// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bytes"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/tests"
)

func TestDecodeInvalidRequests(t *testing.T) {
	test := []string{
		"",
		"*hello\r\n",
		"*-100\r\n",
		"*3\r\nhi",
		"*3\r\nhi\r\n",
		"*4\r\n$1",
		"*4\r\n$1\r",
		"*4\r\n$1\n",
		"*2\r\n$3\r\nget\r\n$what?\r\nx\r\n",
		"*4\r\n$3\r\nget\r\n$1\r\nx\r\n",
		"*2\r\n$3\r\nget\r\n$1\r\nx",
		"*2\r\n$3\r\nget\r\n$1\r\nx\r",
		"*2\r\n$3\r\nget\r\n$100\r\nx\r\n",
		"$6\r\nfoobar\r",
		"$0\rn\r\n",
		"$-1\n",
		"*0",
		"*2n$3\r\nfoo\r\n$3\r\nbar\r\n",
		"3\r\n:1\r\n:2\r\n:3\r\n",
		"*-\r\n",
		"+OK\n",
		"-Error message\r",
	}
	for _, s := range test {
		_, err := DecodeFromBytes([]byte(s))
		tests.Assert(t, err != nil)
	}
}

func TestDecodeBulkBytes(t *testing.T) {
	test := "*2\r\n$4\r\nLLEN\r\n$6\r\nmylist\r\n"
	resp, err := DecodeFromBytes([]byte(test))
	tests.AssertNoError(t, err)
	x, ok := resp.(*Array)
	tests.Assert(t, ok)
	tests.Assert(t, len(x.Value) == 2)
	var s1, s2 *BulkBytes
	s1, ok = x.Value[0].(*BulkBytes)
	tests.Assert(t, ok)
	tests.Assert(t, bytes.Equal(s1.Value, []byte("LLEN")))
	s2, ok = x.Value[1].(*BulkBytes)
	tests.Assert(t, ok)
	tests.Assert(t, bytes.Equal(s2.Value, []byte("mylist")))
}

func TestDecoder(t *testing.T) {
	test := []string{
		"$6\r\nfoobar\r\n",
		"$0\r\n\r\n",
		"$-1\r\n",
		"*0\r\n",
		"*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n",
		"*3\r\n:1\r\n:2\r\n:3\r\n",
		"*-1\r\n",
		"+OK\r\n",
		"-Error message\r\n",
		"*2\r\n$1\r\n0\r\n*0\r\n",
		"*3\r\n$4\r\nEVAL\r\n$31\r\nreturn {1,2,{3,'Hello World!'}}\r\n$1\r\n0\r\n",
	}
	for _, s := range test {
		_, err := DecodeFromBytes([]byte(s))
		tests.AssertNoError(t, err)
	}
}
