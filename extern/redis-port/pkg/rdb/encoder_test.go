// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rdb

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/counter"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/testing/assert"
)

func toString(text string) String {
	return String([]byte(text))
}

func checkString(t *testing.T, o interface{}, text string) {
	x, ok := o.(String)
	assert.Must(t, ok)
	assert.Must(t, string(x) == text)
}

func TestEncodeString(t *testing.T) {
	docheck := func(text string) {
		p, err := EncodeDump(toString(text))
		assert.ErrorIsNil(t, err)
		o, err := DecodeDump(p)
		assert.ErrorIsNil(t, err)
		checkString(t, o, text)
	}
	docheck("hello world!!")
	docheck("2147483648")
	docheck("4294967296")
	docheck("")
	var b bytes.Buffer
	for i := 0; i < 1024; i++ {
		b.Write([]byte("01"))
	}
	docheck(b.String())
}

func toList(list ...string) List {
	o := List{}
	for _, e := range list {
		o = append(o, []byte(e))
	}
	return o
}

func checkList(t *testing.T, o interface{}, list []string) {
	x, ok := o.(List)
	assert.Must(t, ok)
	assert.Must(t, len(x) == len(list))
	for i, e := range x {
		assert.Must(t, string(e) == list[i])
	}
}

func TestEncodeList(t *testing.T) {
	docheck := func(list ...string) {
		p, err := EncodeDump(toList(list...))
		assert.ErrorIsNil(t, err)
		o, err := DecodeDump(p)
		assert.ErrorIsNil(t, err)
		checkList(t, o, list)
	}
	docheck("")
	docheck("", "a", "b", "c", "d", "e")
	list := []string{}
	for i := 0; i < 65536; i++ {
		list = append(list, strconv.Itoa(i))
	}
	docheck(list...)
}

func toHash(m map[string]string) Hash {
	o := Hash{}
	for k, v := range m {
		o = append(o, &HashElement{Field: []byte(k), Value: []byte(v)})
	}
	return o
}

func checkHash(t *testing.T, o interface{}, m map[string]string) {
	x, ok := o.(Hash)
	assert.Must(t, ok)
	assert.Must(t, len(x) == len(m))
	for _, e := range x {
		assert.Must(t, m[string(e.Field)] == string(e.Value))
	}
}

func TestEncodeHash(t *testing.T) {
	docheck := func(m map[string]string) {
		p, err := EncodeDump(toHash(m))
		assert.ErrorIsNil(t, err)
		o, err := DecodeDump(p)
		assert.ErrorIsNil(t, err)
		checkHash(t, o, m)
	}
	docheck(map[string]string{"": ""})
	docheck(map[string]string{"": "", "a": "", "b": "a", "c": "b", "d": "c"})
	hash := make(map[string]string)
	for i := 0; i < 65536; i++ {
		hash[strconv.Itoa(i)] = strconv.Itoa(i + 1)
	}
	docheck(hash)
}

func toZSet(m map[string]float64) ZSet {
	o := ZSet{}
	for k, v := range m {
		o = append(o, &ZSetElement{Member: []byte(k), Score: v})
	}
	return o
}

func checkZSet(t *testing.T, o interface{}, m map[string]float64) {
	x, ok := o.(ZSet)
	assert.Must(t, ok)
	assert.Must(t, len(x) == len(m))
	for _, e := range x {
		v := m[string(e.Member)]
		switch {
		case math.IsInf(v, 1):
			assert.Must(t, math.IsInf(e.Score, 1))
		case math.IsInf(v, -1):
			assert.Must(t, math.IsInf(e.Score, -1))
		case math.IsNaN(v):
			assert.Must(t, math.IsNaN(e.Score))
		default:
			assert.Must(t, math.Abs(e.Score-v) < 1e-10)
		}
	}
}

func TestEncodeZSet(t *testing.T) {
	docheck := func(m map[string]float64) {
		p, err := EncodeDump(toZSet(m))
		assert.ErrorIsNil(t, err)
		o, err := DecodeDump(p)
		assert.ErrorIsNil(t, err)
		checkZSet(t, o, m)
	}
	docheck(map[string]float64{"": 0})
	zset := make(map[string]float64)
	for i := -65535; i < 65536; i++ {
		zset[strconv.Itoa(i)] = float64(i)
	}
	docheck(zset)
	zset["inf"] = math.Inf(1)
	zset["-inf"] = math.Inf(-1)
	zset["nan"] = math.NaN()
	docheck(zset)
}

func toSet(set ...string) Set {
	o := Set{}
	for _, e := range set {
		o = append(o, []byte(e))
	}
	return o
}

func checkSet(t *testing.T, o interface{}, set []string) {
	x, ok := o.(Set)
	assert.Must(t, ok)
	assert.Must(t, len(x) == len(set))
	for i, e := range x {
		assert.Must(t, string(e) == set[i])
	}
}

func TestEncodeSet(t *testing.T) {
	docheck := func(set ...string) {
		p, err := EncodeDump(toSet(set...))
		assert.ErrorIsNil(t, err)
		o, err := DecodeDump(p)
		assert.ErrorIsNil(t, err)
		checkSet(t, o, set)
	}
	docheck("")
	docheck("", "a", "b", "c")
	set := []string{}
	for i := 0; i < 65536; i++ {
		set = append(set, strconv.Itoa(i))
	}
	docheck(set...)
}

func TestEncodeRdb(t *testing.T) {
	objs := make([]struct {
		db       uint32
		expireat uint64
		key      []byte
		obj      interface{}
		typ      string
	}, 128)
	var b bytes.Buffer
	enc := NewEncoder(&b)
	assert.ErrorIsNil(t, enc.EncodeHeader())
	for i := 0; i < len(objs); i++ {
		db := uint32(i + 32)
		expireat := uint64(i)
		key := []byte(strconv.Itoa(i))
		var obj interface{}
		var typ string
		switch i % 5 {
		case 0:
			s := strconv.Itoa(i)
			obj = s
			typ = "string"
			assert.ErrorIsNil(t, enc.EncodeObject(db, key, expireat, toString(s)))
		case 1:
			list := []string{}
			for j := 0; j < 32; j++ {
				list = append(list, fmt.Sprintf("l%d_%d", i, rand.Int()))
			}
			obj = list
			typ = "list"
			assert.ErrorIsNil(t, enc.EncodeObject(db, key, expireat, toList(list...)))
		case 2:
			hash := make(map[string]string)
			for j := 0; j < 32; j++ {
				hash[strconv.Itoa(j)] = fmt.Sprintf("h%d_%d", i, rand.Int())
			}
			obj = hash
			typ = "hash"
			assert.ErrorIsNil(t, enc.EncodeObject(db, key, expireat, toHash(hash)))
		case 3:
			zset := make(map[string]float64)
			for j := 0; j < 32; j++ {
				zset[strconv.Itoa(j)] = rand.Float64()
			}
			obj = zset
			typ = "zset"
			assert.ErrorIsNil(t, enc.EncodeObject(db, key, expireat, toZSet(zset)))
		case 4:
			set := []string{}
			for j := 0; j < 32; j++ {
				set = append(set, fmt.Sprintf("s%d_%d", i, rand.Int()))
			}
			obj = set
			typ = "set"
			assert.ErrorIsNil(t, enc.EncodeObject(db, key, expireat, toSet(set...)))
		}
		objs[i].db = db
		objs[i].expireat = expireat
		objs[i].key = key
		objs[i].obj = obj
		objs[i].typ = typ
	}
	assert.ErrorIsNil(t, enc.EncodeFooter())
	rdb := b.Bytes()
	var c counter.Int64
	l := NewLoader(ioutils.NewCountReader(bytes.NewReader(rdb), &c))
	assert.ErrorIsNil(t, l.Header())
	var i int = 0
	for {
		e, err := l.NextBinEntry()
		assert.ErrorIsNil(t, err)
		if e == nil {
			break
		}
		assert.Must(t, objs[i].db == e.DB)
		assert.Must(t, objs[i].expireat == e.ExpireAt)
		assert.Must(t, bytes.Equal(objs[i].key, e.Key))
		o, err := DecodeDump(e.Value)
		assert.ErrorIsNil(t, err)
		switch objs[i].typ {
		case "string":
			checkString(t, o, objs[i].obj.(string))
		case "list":
			checkList(t, o, objs[i].obj.([]string))
		case "hash":
			checkHash(t, o, objs[i].obj.(map[string]string))
		case "zset":
			checkZSet(t, o, objs[i].obj.(map[string]float64))
		case "set":
			checkSet(t, o, objs[i].obj.([]string))
		}
		i++
	}
	assert.Must(t, i == len(objs))
	assert.ErrorIsNil(t, l.Footer())
	assert.Must(t, c.Get() == int64(len(rdb)))
}
