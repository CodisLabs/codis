// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import "testing"

func TestXAppend(t *testing.T) {
	c := client(t)
	k := random(t)
	checknil(t, c, "get", k)
	checkint(t, 5, c, "append", k, "hello")
	checkint(t, 11, c, "append", k, " world")
	checkstring(t, "hello world", c, "get", k)
}

func TestXDecr(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "set", k, 10)
	checkint(t, 9, c, "decr", k)
	checkok(t, c, "set", k, -100)
	checkint(t, -101, c, "decr", k)
}

func TestXDecrBy(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "set", k, 10)
	checkint(t, 5, c, "decrby", k, 5)
	checkint(t, 5, c, "decrby", k, 0)
}

func TestXGet(t *testing.T) {
	c := client(t)
	k := random(t)
	checknil(t, c, "get", k)
	checkok(t, c, "set", k, "hello world")
	checkstring(t, "hello world", c, "get", k)
	checkok(t, c, "set", k, "goodbye")
}

func TestXGetSet(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 1, c, "incr", k)
	checkstring(t, "1", c, "getset", k, 0)
	checkstring(t, "0", c, "get", k)
}

func TestXIncr(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "set", k, 10)
	checkint(t, 11, c, "incr", k)
	checkstring(t, "11", c, "get", k)
}

func TestXIncrBy(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "set", k, 10)
	checkint(t, 15, c, "incrby", k, 5)
}

func TestXIncrByFloat(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "set", k, 10.50)
	checkfloat(t, 10.6, c, "incrbyfloat", k, 0.1)
	checkok(t, c, "set", k, "5.0e3")
	checkfloat(t, 5200, c, "incrbyfloat", k, 2.0e2)
	checkok(t, c, "set", k, "0")
	checkfloat(t, 996945661, c, "incrbyfloat", k, 996945661)
}

func TestXSet(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "set", k, "hello")
	checkstring(t, "hello", c, "get", k)
}

func TestXPSetEX(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "psetex", k, 1000*1000, "hello")
	checkok(t, c, "psetex", k, 2000*1000, "world")
	checkstring(t, "world", c, "get", k)
	checkintapprox(t, 2000, 5, c, "ttl", k)
}

func TestXSetEX(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "setex", k, 1000, "hello")
	checkok(t, c, "setex", k, 2000, "world")
	checkstring(t, "world", c, "get", k)
	checkintapprox(t, 2000, 5, c, "ttl", k)
}

func TestXSetNX(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 1, c, "setnx", k, "hello")
	checkint(t, 0, c, "setnx", k, "world")
	checkstring(t, "hello", c, "get", k)
	checkint(t, -1, c, "ttl", k)
}

func TestXSetRange(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 7, c, "setrange", k, 2, "redis")
	checkstring(t, "\x00\x00redis", c, "get", k)
	checkint(t, 7, c, "setrange", k, 1, "redis")
	checkstring(t, "\x00rediss", c, "get", k)
	checkint(t, 11, c, "setrange", k, 0, "hello world")
	checkstring(t, "hello world", c, "get", k)
}

func TestXSetBit(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 0, c, "setbit", k, 3, 1)
	checkstring(t, "\x08", c, "get", k)
	checkint(t, 1, c, "setbit", k, 3, 1)
	checkstring(t, "\x08", c, "get", k)
	checkint(t, 1, c, "setbit", k, 3, 0)
	checkstring(t, "\x00", c, "get", k)
	checkint(t, 0, c, "setbit", k, 8, 1)
	checkstring(t, "\x00\x01", c, "get", k)
}

func TestXMSet(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "mset", k, 0, k+"1", 1, k+"2", 2, k+"3", 3)
	checkstring(t, "0", c, "get", k)
	checkstring(t, "1", c, "get", k+"1")
	checkstring(t, "2", c, "get", k+"2")
	checkstring(t, "3", c, "get", k+"3")
	checkok(t, c, "mset", k, 100, k, 1000)
	checkstring(t, "1000", c, "get", k)
}

func TestMSetNX(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "set", k, "haha")
	checkint(t, 0, c, "msetnx", k, "1", "1", "1", "2", "2")
	checkint(t, 1, c, "del", k)
	checkint(t, 1, c, "msetnx", k, "1", k, "2")
	checkstring(t, "2", c, "get", k)
}

func TestMGet(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "mset", k, 0, k+"1", 1, k+"2", 2, k+"3", 3)
	a := checkbytesarray(t, c, "mget", k, k+"1", k+"2", k+"3")
	checkerror(t, nil, len(a) == 4)
	checkerror(t, nil, string(a[0]) == "0")
	checkerror(t, nil, string(a[1]) == "1")
	checkerror(t, nil, string(a[2]) == "2")
	checkerror(t, nil, string(a[3]) == "3")
}
