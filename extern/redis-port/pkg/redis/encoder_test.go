// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/tests"
)

func TestEncodeString(t *testing.T) {
	resp := &String{"OK"}
	testEncodeAndCheck(t, resp, []byte("+OK\r\n"))
}

func TestEncodeError(t *testing.T) {
	resp := &Error{"Error"}
	testEncodeAndCheck(t, resp, []byte("-Error\r\n"))
}

func TestEncodeInt(t *testing.T) {
	resp := &Int{}
	for _, v := range []int{-1, 0, 1024 * 1024} {
		resp.Value = int64(v)
		testEncodeAndCheck(t, resp, []byte(":"+strconv.FormatInt(int64(v), 10)+"\r\n"))
	}
}

func TestEncodeBulkBytes(t *testing.T) {
	resp := &BulkBytes{}
	resp.Value = nil
	testEncodeAndCheck(t, resp, []byte("$-1\r\n"))
	resp.Value = []byte{}
	testEncodeAndCheck(t, resp, []byte("$0\r\n\r\n"))
	resp.Value = []byte("helloworld!!")
	testEncodeAndCheck(t, resp, []byte("$12\r\nhelloworld!!\r\n"))
}

func TestEncodeArray(t *testing.T) {
	resp := &Array{}
	resp.Value = nil
	testEncodeAndCheck(t, resp, []byte("*-1\r\n"))
	resp.Value = []Resp{}
	testEncodeAndCheck(t, resp, []byte("*0\r\n"))
	resp.Append(&Int{0})
	testEncodeAndCheck(t, resp, []byte("*1\r\n:0\r\n"))
	resp.Append(&BulkBytes{nil})
	testEncodeAndCheck(t, resp, []byte("*2\r\n:0\r\n$-1\r\n"))
	resp.Append(&BulkBytes{[]byte("test")})
	testEncodeAndCheck(t, resp, []byte("*3\r\n:0\r\n$-1\r\n$4\r\ntest\r\n"))
}

func testEncodeAndCheck(t *testing.T, resp Resp, expect []byte) {
	b, err := EncodeToBytes(resp)
	tests.AssertNoError(t, err)
	tests.Assert(t, bytes.Equal(b, expect))
}
