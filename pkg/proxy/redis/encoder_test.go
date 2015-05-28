// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/wandoulabs/codis/pkg/utils/assert"
)

func TestItos(t *testing.T) {
	for i := 0; i < len(imap)*2; i++ {
		n, p := -i, i
		assert.Must(strconv.Itoa(n) == itos(int64(n)))
		assert.Must(strconv.Itoa(p) == itos(int64(p)))
	}
}

func TestEncodeString(t *testing.T) {
	resp := NewString([]byte("OK"))
	testEncodeAndCheck(t, resp, []byte("+OK\r\n"))
}

func TestEncodeError(t *testing.T) {
	resp := NewError([]byte("Error"))
	testEncodeAndCheck(t, resp, []byte("-Error\r\n"))
}

func TestEncodeInt(t *testing.T) {
	for _, v := range []int{-1, 0, 1024 * 1024} {
		s := strconv.Itoa(v)
		resp := NewInt([]byte(s))
		testEncodeAndCheck(t, resp, []byte(":"+s+"\r\n"))
	}
}

func TestEncodeBulkBytes(t *testing.T) {
	resp := NewBulkBytes(nil)
	testEncodeAndCheck(t, resp, []byte("$-1\r\n"))
	resp.Value = []byte{}
	testEncodeAndCheck(t, resp, []byte("$0\r\n\r\n"))
	resp.Value = []byte("helloworld!!")
	testEncodeAndCheck(t, resp, []byte("$12\r\nhelloworld!!\r\n"))
}

func TestEncodeArray(t *testing.T) {
	resp := NewArray(nil)
	testEncodeAndCheck(t, resp, []byte("*-1\r\n"))
	resp.Array = []*Resp{}
	testEncodeAndCheck(t, resp, []byte("*0\r\n"))
	resp.Append(NewInt([]byte(strconv.Itoa(0))))
	testEncodeAndCheck(t, resp, []byte("*1\r\n:0\r\n"))
	resp.Append(NewBulkBytes(nil))
	testEncodeAndCheck(t, resp, []byte("*2\r\n:0\r\n$-1\r\n"))
	resp.Append(NewBulkBytes([]byte("test")))
	testEncodeAndCheck(t, resp, []byte("*3\r\n:0\r\n$-1\r\n$4\r\ntest\r\n"))
}

func testEncodeAndCheck(t *testing.T, resp *Resp, expect []byte) {
	b, err := EncodeToBytes(resp)
	assert.MustNoError(err)
	assert.Must(bytes.Equal(b, expect))
}
