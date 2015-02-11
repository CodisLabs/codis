// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"bytes"
	"math"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/testing/assert"
)

func TestEncodeSimple(t *testing.T) {
	defer func() {
		assert.Must(t, recover() == nil)
	}()
	w := NewBufWriter(nil)
	var b byte = 0xfc
	var c ObjectCode = StringCode
	var f = 3.14
	var u32 uint32 = 1 << 31
	var i64 = int64(u32 - 1)
	var u64 = uint64(1<<63 - 1)
	refs := []interface{}{b, c, &f, &u32, &i64, &u64}
	encodeRawBytes(w, refs...)

	b, c, f = 0, ObjectCode(0), 0
	u32 = 0
	i64 = 0
	u64 = 0

	r := NewBufReader(w.Bytes())
	err := decodeRawBytes(r, nil, refs...)
	assert.ErrorIsNil(t, err)
	assert.Must(t, r.Len() == 0)
	assert.Must(t, refs[0].(byte) == 0xfc)
	assert.Must(t, refs[1].(ObjectCode) == StringCode)
	assert.Must(t, math.Abs(f-3.14) < 1e-9)
	assert.Must(t, u32 == 1<<31)
	assert.Must(t, i64 == int64(u32-1))
	assert.Must(t, u64 == 1<<63-1)
}

func TestEncodeBytes(t *testing.T) {
	defer func() {
		assert.Must(t, recover() == nil)
	}()
	w := NewBufWriter(nil)
	b := make([]byte, 1024)
	for i := 0; i < len(b); i++ {
		b[i] = byte((i + 1) * i)
	}
	vb := make([]byte, len(b))
	eb := make([]byte, len(b))
	copy(vb, b)
	copy(eb, b)
	refs := []interface{}{&vb, &eb}
	encodeRawBytes(w, refs...)

	vb, eb = nil, nil

	r := NewBufReader(w.Bytes())
	err := decodeRawBytes(r, nil, refs...)
	assert.ErrorIsNil(t, err)
	assert.Must(t, r.Len() == 0)
	assert.Must(t, bytes.Equal(b, vb))
	assert.Must(t, bytes.Equal(b, eb))
}
