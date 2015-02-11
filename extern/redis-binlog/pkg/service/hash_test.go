// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import "testing"

func checkhash(t *testing.T, s Session, k string, expect map[string]string) {
	array := checkbytesarray(t, s, "hgetall", k)
	if expect == nil {
		checkerror(t, nil, array == nil)
	} else {
		checkerror(t, nil, len(array) == len(expect)*2)
		for i := 0; i < len(expect); i++ {
			k := string(array[i*2])
			v := string(array[i*2+1])
			checkerror(t, nil, expect[k] == v)
		}
	}
}

func TestHDel(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 0, c, "hdel", k, "key1")
	checkok(t, c, "hmset", k, "key1", "hello1", "key2", "hello2")
	checkint(t, 1, c, "hdel", k, "key1")
	checkint(t, 0, c, "hdel", k, "key1")
	checkint(t, 1, c, "hdel", k, "key2")
	checkhash(t, c, k, nil)
}

func TestHSet(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 1, c, "hset", k, "field", "value")
	checkint(t, 0, c, "hset", k, "field", "value2")
	checkhash(t, c, k, map[string]string{"field": "value2"})
}

func TestHGet(t *testing.T) {
	c := client(t)
	k := random(t)
	checknil(t, c, "hget", k, "field")
	checkint(t, 1, c, "hset", k, "field", "value")
	checkstring(t, "value", c, "hget", k, "field")
}

func TestHLen(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 0, c, "hlen", k)
	checkint(t, 1, c, "hset", k, "field", "value")
	checkint(t, 1, c, "hlen", k)
	checkint(t, 1, c, "hdel", k, "field")
	checkint(t, 0, c, "hlen", k)
}

func TestHExists(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 0, c, "hexists", k, "field")
	checkint(t, 1, c, "hset", k, "field", "value")
	checkint(t, 1, c, "hexists", k, "field")
}

func TestHMSet(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "hmset", k, "key1", "hello1", "key2", "hello2")
	checkok(t, c, "hmset", k, "key1", "world1")
	checkhash(t, c, k, map[string]string{"key1": "world1", "key2": "hello2"})
}

func TestHIncrBy(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 1, c, "hset", k, "key", "5")
	checkint(t, 6, c, "hincrby", k, "key", 1)
	checkint(t, 5, c, "hincrby", k, "key", -1)
	checkint(t, -5, c, "hincrby", k, "key", -10)
	checkint(t, 1, c, "hincrby", k, "key2", 1)
}

func TestHIncrByFloat(t *testing.T) {
	c := client(t)
	k := random(t)
	checkfloat(t, 10.5, c, "hincrbyfloat", k, "field", 10.5)
	checkfloat(t, 10.6, c, "hincrbyfloat", k, "field", 0.1)
	checkint(t, 1, c, "hset", k, "field2", 2.0e2)
	checkfloat(t, 5200, c, "hincrbyfloat", k, "field2", 5.0e3)
}

func TestHSetNX(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 1, c, "hsetnx", k, "key", "hello")
	checkint(t, 0, c, "hsetnx", k, "key", "world")
	checkhash(t, c, k, map[string]string{"key": "hello"})
	checkint(t, 1, c, "del", k)
	checkhash(t, c, k, nil)
}

func TestHMGet(t *testing.T) {
	c := client(t)
	k := random(t)
	var a [][]byte
	a = checkbytesarray(t, c, "hmget", k, "field1", "field2")
	checkerror(t, nil, len(a) == 2)
	checkerror(t, nil, a[0] == nil && a[1] == nil)

	checkint(t, 1, c, "hsetnx", k, "field1", "value")
	a = checkbytesarray(t, c, "hmget", k, "field1", "field2", "field1")
	checkerror(t, nil, len(a) == 3)
	checkerror(t, nil, string(a[0]) == "value" && a[1] == nil && string(a[0]) == string(a[2]))
}

func TestHKeys(t *testing.T) {
	c := client(t)
	k := random(t)
	var a [][]byte
	a = checkbytesarray(t, c, "hkeys", k)
	checkerror(t, nil, len(a) == 0)

	checkint(t, 1, c, "hsetnx", k, "field1", "value")
	a = checkbytesarray(t, c, "hkeys", k)
	checkerror(t, nil, len(a) == 1)
	checkerror(t, nil, string(a[0]) == "field1")
}

func TestHVals(t *testing.T) {
	c := client(t)
	k := random(t)
	var a [][]byte
	a = checkbytesarray(t, c, "hkeys", k)
	checkerror(t, nil, len(a) == 0)

	checkint(t, 1, c, "hsetnx", k, "field1", "value")
	a = checkbytesarray(t, c, "hvals", k)
	checkerror(t, nil, len(a) == 1)
	checkerror(t, nil, string(a[0]) == "value")
}
