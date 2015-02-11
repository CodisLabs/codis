// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/testing/assert"
)

type testHandler struct {
	c map[string]int
}

func (h *testHandler) count(args ...[]byte) (Resp, error) {
	for _, arg := range args {
		h.c[string(arg)]++
	}
	return nil, nil
}

func (h *testHandler) Get(arg0 interface{}, args ...[]byte) (Resp, error) {
	return h.count(args...)
}

func (h *testHandler) Set(arg0 interface{}, args [][]byte) (Resp, error) {
	return h.count(args...)
}

func testmapcount(t *testing.T, m1, m2 map[string]int) {
	assert.Must(t, len(m1) == len(m2))
	for k, _ := range m1 {
		assert.Must(t, m1[k] == m2[k])
	}
}

func TestHandlerFunc(t *testing.T) {
	h := &testHandler{make(map[string]int)}
	s, err := NewServer(h)
	assert.ErrorIsNil(t, err)
	key1, key2, key3, key4 := "key1", "key2", "key3", "key4"
	s.t["get"](nil)
	testmapcount(t, h.c, map[string]int{})
	s.t["get"](nil, []byte(key1), []byte(key2))
	testmapcount(t, h.c, map[string]int{key1: 1, key2: 1})
	s.t["get"](nil, [][]byte{[]byte(key1), []byte(key3)}...)
	testmapcount(t, h.c, map[string]int{key1: 2, key2: 1, key3: 1})
	s.t["set"](nil)
	testmapcount(t, h.c, map[string]int{key1: 2, key2: 1, key3: 1})
	s.t["set"](nil, []byte(key1), []byte(key4))
	testmapcount(t, h.c, map[string]int{key1: 3, key2: 1, key3: 1, key4: 1})
	s.t["set"](nil, [][]byte{[]byte(key1), []byte(key2), []byte(key3)}...)
	testmapcount(t, h.c, map[string]int{key1: 4, key2: 2, key3: 2, key4: 1})
}

func TestServerServe(t *testing.T) {
	h := &testHandler{make(map[string]int)}
	s, err := NewServer(h)
	assert.ErrorIsNil(t, err)
	resp, err := Decode(bufio.NewReader(bytes.NewReader([]byte("*2\r\n$3\r\nset\r\n$3\r\nfoo\r\n"))))
	assert.ErrorIsNil(t, err)
	_, err = s.Dispatch(nil, resp)
	assert.ErrorIsNil(t, err)
	testmapcount(t, h.c, map[string]int{"foo": 1})
}
