// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"testing"
)

func checkset(t *testing.T, s Session, k string, expect []string) {
	array := checkbytesarray(t, s, "smembers", k)
	if expect == nil {
		checkerror(t, nil, array == nil)
	} else {
		checkerror(t, nil, len(array) == len(expect))
		m := make(map[string]bool)
		for _, s := range expect {
			m[s] = true
		}
		checkerror(t, nil, len(array) == len(m))
		for _, v := range array {
			checkerror(t, nil, m[string(v)])
		}
	}
}

func TestSAdd(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "sadd", k, "key1", "key2", "key3")
	checkint(t, 1, c, "sadd", k, "key1", "key2", "key3", "key4", "key4")
	checkset(t, c, k, []string{"key1", "key2", "key3", "key4"})
	checkint(t, -1, c, "ttl", k)
}

func TestSRem(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "sadd", k, "key1", "key2", "key3")
	checkint(t, 1, c, "srem", k, "key1", "key4")
	checkint(t, 0, c, "srem", k, "key1", "key4")
	checkset(t, c, k, []string{"key2", "key3"})
	checkint(t, 2, c, "srem", k, "key2", "key3")
	checkint(t, -2, c, "ttl", k)
	checkint(t, 0, c, "srem", k, "key1")
	checkset(t, c, k, nil)
}

func TestSCard(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "sadd", k, "key1", "key2", "key3", "key1")
	checkint(t, 3, c, "scard", k)
	checkint(t, 1, c, "srem", k, "key1", "key4")
	checkint(t, 2, c, "scard", k)
	checkint(t, 0, c, "srem", k, "key1", "key4")
	checkint(t, 2, c, "scard", k)
	checkint(t, 1, c, "del", k)
	checkint(t, 0, c, "scard", k)
}

func TestSIsMember(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "sadd", k, "key1", "key2", "key3")
	checkint(t, 1, c, "sismember", k, "key1")
	checkint(t, 0, c, "sismember", k, "key0")
	checkint(t, 1, c, "del", k)
	checkint(t, 0, c, "sismember", k, "key1")
}

func TestSPop(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "sadd", k, "key1", "key2", "key3")
	for i := 2; i >= 0; i-- {
		checkdo(t, c, "spop", k)
		checkint(t, int64(i), c, "scard", k)
	}
}

func TestRandMember(t *testing.T) {
	c := client(t)
	k := random(t)
	var a [][]byte
	checkint(t, 3, c, "sadd", k, "key1", "key2", "key3")
	a = checkbytesarray(t, c, "srandmember", k, 0)
	checkerror(t, nil, len(a) == 0)
	a = checkbytesarray(t, c, "srandmember", k, 100)
	checkerror(t, nil, len(a) == 3)
	m := make(map[string]bool)
	for _, v := range a {
		m[string(v)] = true
	}
	checkerror(t, nil, m["key1"])
	checkerror(t, nil, m["key2"])
	checkerror(t, nil, m["key3"])
}
