// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/binlog"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store/rocksdb"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/testing/assert"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

var (
	testbl *binlog.Binlog
	server = redis.MustServer(&Handler{})
	keymap = make(map[string]bool)
)

type fakeSession struct {
	db uint32
}

func (s *fakeSession) DB() uint32 {
	return s.db
}

func (s *fakeSession) SetDB(db uint32) {
	s.db = db
}

func (s *fakeSession) Binlog() *binlog.Binlog {
	return testbl
}

func reinit() {
	if testbl != nil {
		testbl.Close()
		testbl = nil
	}
	const path = "/tmp/testdb-rocksdb"
	if err := os.RemoveAll(path); err != nil {
		log.PanicErrorf(err, "remove '%s' failed", path)
	} else {
		conf := rocksdb.NewDefaultConfig()
		if testdb, err := rocksdb.Open(path, conf, true, false); err != nil {
			log.PanicError(err, "open rocksdb failed")
		} else {
			testbl = binlog.New(testdb)
		}
	}
}

func init() {
	reinit()
}

func client(t *testing.T) *fakeSession {
	return &fakeSession{}
}

func random(t *testing.T) string {
	for i := 0; ; i++ {
		p := make([]byte, 16)
		for j := 0; j < len(p); j++ {
			p[j] = 'a' + byte(rand.Intn(26))
		}
		s := "key_" + string(p)
		if _, ok := keymap[s]; !ok {
			keymap[s] = true
			return s
		}
		assert.Must(t, i < 32)
	}
}

func checkerror(t *testing.T, err error, exp bool) {
	if err != nil || !exp {
		reinit()
	}
	assert.ErrorIsNil(t, err)
	assert.Must(t, exp)
}

func request(cmd string, args ...interface{}) redis.Resp {
	resp := redis.NewArray()
	resp.AppendBulkBytes([]byte(cmd))
	for _, v := range args {
		resp.AppendBulkBytes([]byte(fmt.Sprintf("%v", v)))
	}
	return resp
}

func checkok(t *testing.T, s Session, cmd string, args ...interface{}) {
	checkstring(t, "OK", s, cmd, args...)
}

func checkdo(t *testing.T, s Session, cmd string, args ...interface{}) {
	rsp, err := server.Dispatch(s, request(cmd, args...))
	checkerror(t, err, rsp != nil)
}

func checknil(t *testing.T, s Session, cmd string, args ...interface{}) {
	rsp, err := server.Dispatch(s, request(cmd, args...))
	checkerror(t, err, rsp != nil)
	switch x := rsp.(type) {
	case *redis.BulkBytes:
		checkerror(t, nil, x.Value == nil)
	case *redis.Array:
		checkerror(t, nil, x.Value == nil)
	default:
		checkerror(t, nil, false)
	}
}

func checkstring(t *testing.T, expect string, s Session, cmd string, args ...interface{}) {
	rsp, err := server.Dispatch(s, request(cmd, args...))
	checkerror(t, err, rsp != nil)
	switch x := rsp.(type) {
	case *redis.String:
		checkerror(t, nil, x.Value == expect)
	case *redis.BulkBytes:
		checkerror(t, nil, string(x.Value) == expect)
	default:
		checkerror(t, nil, false)
	}
}

func checkint(t *testing.T, expect int64, s Session, cmd string, args ...interface{}) {
	rsp, err := server.Dispatch(s, request(cmd, args...))
	checkerror(t, err, rsp != nil)
	x, ok := rsp.(*redis.Int)
	checkerror(t, nil, ok)
	checkerror(t, nil, x.Value == expect)
}

func checkintapprox(t *testing.T, expect, delta int64, s Session, cmd string, args ...interface{}) {
	rsp, err := server.Dispatch(s, request(cmd, args...))
	checkerror(t, err, rsp != nil)
	x, ok := rsp.(*redis.Int)
	checkerror(t, nil, ok)
	checkerror(t, nil, math.Abs(float64(x.Value-expect)) <= float64(delta))
}

func checkbytes(t *testing.T, expect []byte, s Session, cmd string, args ...interface{}) {
	rsp, err := server.Dispatch(s, request(cmd, args...))
	checkerror(t, err, rsp != nil)
	x, ok := rsp.(*redis.BulkBytes)
	checkerror(t, nil, ok)
	checkerror(t, nil, bytes.Equal(x.Value, expect))
}

func checkfloat(t *testing.T, expect float64, s Session, cmd string, args ...interface{}) {
	rsp, err := server.Dispatch(s, request(cmd, args...))
	checkerror(t, err, rsp != nil)
	var v string
	switch x := rsp.(type) {
	case *redis.String:
		v = x.Value
	case *redis.BulkBytes:
		v = string(x.Value)
	default:
		checkerror(t, nil, false)
	}
	f, err := strconv.ParseFloat(v, 64)
	checkerror(t, err, math.Abs(f-expect) < 1e-10)
}

func checkbytesarray(t *testing.T, s Session, cmd string, args ...interface{}) [][]byte {
	rsp, err := server.Dispatch(s, request(cmd, args...))
	checkerror(t, err, rsp != nil)
	x, ok := rsp.(*redis.Array)
	checkerror(t, nil, ok)
	if x.Value == nil {
		return nil
	}
	array := make([][]byte, len(x.Value))
	for i, v := range x.Value {
		x, ok := v.(*redis.BulkBytes)
		checkerror(t, nil, ok)
		array[i] = x.Value
	}
	return array
}

func checkintarray(t *testing.T, expect []int64, s Session, cmd string, args ...interface{}) {
	rsp, err := server.Dispatch(s, request(cmd, args...))
	checkerror(t, err, rsp != nil)
	x, ok := rsp.(*redis.Array)
	checkerror(t, nil, ok && x.Value != nil)
	array := make([]int64, len(x.Value))
	for i, v := range x.Value {
		x, ok := v.(*redis.Int)
		checkerror(t, nil, ok)
		array[i] = x.Value
	}
	checkerror(t, nil, len(array) == len(expect))
	for i := 0; i < len(array); i++ {
		checkerror(t, nil, array[i] == expect[i])
	}
}
