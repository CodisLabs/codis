// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bytes"
	"math"
	"strconv"
	"testing"

	"github.com/wandoulabs/codis/pkg/utils/assert"
)

func TestItox(t *testing.T) {
	for i := 0; i < len(itoamap)*2; i++ {
		n, p := -i, i
		assert.Must(strconv.Itoa(n) == itoa(int64(n)))
		assert.Must(strconv.Itoa(p) == itoa(int64(p)))
		assert.Must(strconv.Itoa(n) == string(itob(int64(n))))
		assert.Must(strconv.Itoa(p) == string(itob(int64(p))))
	}
	tests := []int64{
		math.MaxInt32, math.MinInt32,
		math.MaxInt64, math.MinInt64,
	}
	for _, v := range tests {
		assert.Must(strconv.FormatInt(v, 10) == itoa(int64(v)))
		assert.Must(strconv.FormatInt(v, 10) == string(itob(int64(v))))
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
