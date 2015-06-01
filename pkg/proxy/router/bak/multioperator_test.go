// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"strings"
	"testing"

	"github.com/alicebob/miniredis"
)

var redisrv *miniredis.Miniredis

func TestMgetResults(t *testing.T) {
	redisrv, err := miniredis.Run()
	if err != nil {
		t.Fatal("can not run miniredis")
	}
	defer redisrv.Close()

	moper := NewMultiOperator(redisrv.Addr())
	redisrv.Set("a", "a")
	redisrv.Set("b", "b")
	redisrv.Set("c", "c")
	buf, err := moper.mgetResults(&MulOp{
		op: "mget",
		keys: [][]byte{[]byte("a"),
			[]byte("b"), []byte("c"), []byte("x")}})
	if err != nil {
		t.Error(err)
	}

	res := string(buf)
	if !strings.Contains(res, "a") || !strings.Contains(res, "b") || !strings.Contains(res, "c") {
		t.Error("not match", res)
	}

	buf, err = moper.mgetResults(&MulOp{
		op: "mget",
		keys: [][]byte{[]byte("x"),
			[]byte("c"), []byte("x")}})
	if err != nil {
		t.Error(err)
	}

	buf, err = moper.mgetResults(&MulOp{
		op: "mget",
		keys: [][]byte{[]byte("x"),
			[]byte("y"), []byte("x")}})
	if err != nil {
		t.Error(err)
	}
}

func TestDeltResults(t *testing.T) {
	redisrv, err := miniredis.Run()
	if err != nil {
		t.Fatal("can not run miniredis")
	}
	defer redisrv.Close()

	moper := NewMultiOperator(redisrv.Addr())
	redisrv.Set("a", "a")
	redisrv.Set("b", "b")
	redisrv.Set("c", "c")
	buf, err := moper.delResults(&MulOp{
		op: "del",
		keys: [][]byte{[]byte("a"),
			[]byte("b"), []byte("c")}})
	if err != nil {
		t.Error(err)
	}

	res := string(buf)
	if !strings.Contains(res, "3") {
		t.Error("not match", res)
	}
}
