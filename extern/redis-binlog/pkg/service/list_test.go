// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import "testing"

func checklist(t *testing.T, s Session, k string, expect []string) {
	array := checkbytesarray(t, s, "lrange", k, 0, -1)
	if expect == nil {
		checkerror(t, nil, array == nil)
	} else {
		checkerror(t, nil, len(array) == len(expect))
		for i := 0; i < len(expect); i++ {
			checkerror(t, nil, string(array[i]) == expect[i])
		}
	}
}

func TestLPush(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 1, c, "lpush", k, "key1")
	checkint(t, 2, c, "lpush", k, "key2")
	checkint(t, 4, c, "lpush", k, "key3", "key4")
	checklist(t, c, k, []string{"key4", "key3", "key2", "key1"})
}

func TestLPushX(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 0, c, "lpushx", k, "key1")
	checklist(t, c, k, nil)
	checkint(t, 1, c, "lpush", k, "key1")
	checkint(t, 2, c, "lpushx", k, "key2")
	checklist(t, c, k, []string{"key2", "key1"})
}

func TestLPop(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 4, c, "lpush", k, "key1", "key2", "key3", "key4")
	checkstring(t, "key4", c, "lpop", k)
	checkstring(t, "key3", c, "lpop", k)
	checklist(t, c, k, []string{"key2", "key1"})
	checkstring(t, "key2", c, "lpop", k)
	checkstring(t, "key1", c, "lpop", k)
	checklist(t, c, k, nil)
	checknil(t, c, "lpop", k)
}

func TestRPush(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 1, c, "rpush", k, "key1")
	checkint(t, 2, c, "rpush", k, "key2")
	checkint(t, 4, c, "rpush", k, "key3", "key4")
	checklist(t, c, k, []string{"key1", "key2", "key3", "key4"})
}

func TestRPushX(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 1, c, "rpush", k, "key1")
	checkint(t, 2, c, "rpushx", k, "key2")
	checklist(t, c, k, []string{"key1", "key2"})
	checkint(t, 1, c, "del", k)
	checkint(t, 0, c, "rpushx", k, "key3")
	checklist(t, c, k, nil)
}

func TestRPop(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "lpush", k, "key1", "key2", "key3")
	checkstring(t, "key1", c, "rpop", k)
	checkstring(t, "key2", c, "rpop", k)
	checkstring(t, "key3", c, "rpop", k)
	checklist(t, c, k, nil)
}

func TestLSet(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "lpush", k, "key1", "key2", "key3")
	checkok(t, c, "lset", k, 0, "one")
	checkok(t, c, "lset", k, 1, "two")
	checkok(t, c, "lset", k, 2, "three")
	checklist(t, c, k, []string{"one", "two", "three"})
	checkok(t, c, "lset", k, -1, "3")
	checkok(t, c, "lset", k, -2, "2")
	checkok(t, c, "lset", k, -3, "1")
	checklist(t, c, k, []string{"1", "2", "3"})
}

func TestLIndex(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "lpush", k, "key1", "key2", "key3")
	checkstring(t, "key1", c, "lindex", k, -1)
	checkstring(t, "key2", c, "lindex", k, -2)
	checkstring(t, "key3", c, "lindex", k, -3)
}

func TestLLen(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 0, c, "llen", k)
	checkint(t, 3, c, "lpush", k, "key1", "key2", "key3")
	checkint(t, 3, c, "llen", k)
}

func TestTrim(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "lpush", k, "key1", "key2", "key3")
	checkok(t, c, "ltrim", k, 0, -1)
	checkint(t, 3, c, "llen", k)
	checkok(t, c, "ltrim", k, 0, -2)
	checklist(t, c, k, []string{"key3", "key2"})
	checkok(t, c, "ltrim", k, 0, 0)
	checklist(t, c, k, []string{"key3"})
	checkok(t, c, "ltrim", k, 1, 0)
	checkint(t, 0, c, "llen", k)
}
