// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"testing"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/binlog"
)

func TestSelect(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "select", 128)
	checkok(t, c, "set", k, "128")
	checkstring(t, "128", c, "get", k)
	checkok(t, c, "select", 258)
	checknil(t, c, "get", k)
	checkok(t, c, "select", 128)
	checkstring(t, "128", c, "get", k)
	checkok(t, c, "select", 0)
}

func TestDel(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "set", k, 100)
	checkint(t, 1, c, "del", k)
	checkok(t, c, "set", k, 200)
	checkint(t, 1, c, "del", k, k, k, k)
	checkint(t, 0, c, "del", k)
}

func TestDump(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "set", k, "hello")
	expect := "\x00\x05\x68\x65\x6c\x6c\x6f\x06\x00\xf5\x9f\xb7\xf6\x90\x61\x1c\x99"
	checkbytes(t, []byte(expect), c, "dump", k)

	checkint(t, 1, c, "del", k)
	checkok(t, c, "restore", k, 1000, expect)
	checkstring(t, "hello", c, "get", k)
	checkintapprox(t, 1000, 50, c, "pttl", k)
}

func TestType(t *testing.T) {
	c := client(t)
	k := random(t)
	checkstring(t, "none", c, "type", k)
	checkint(t, 0, c, "exists", k)
	checkok(t, c, "set", k, "hello")
	checkstring(t, "string", c, "type", k)
	checkint(t, 1, c, "exists", k)
}

func TestExpire(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, -2, c, "ttl", k)
	checkint(t, 0, c, "expire", k, 1000)
	checkok(t, c, "set", k, 100)
	checkint(t, -1, c, "ttl", k)
	checkint(t, 1, c, "expire", k, 1000)
	checkintapprox(t, 1000, 5, c, "ttl", k)
}

func TestPExpire(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, -2, c, "pttl", k)
	checkint(t, 0, c, "pexpire", k, 100000)
	checkok(t, c, "set", k, 100)
	checkint(t, -1, c, "pttl", k)
	checkint(t, 1, c, "pexpire", k, 100000)
	checkintapprox(t, 100000, 5000, c, "pttl", k)
}

func TestExpireAt(t *testing.T) {
	c := client(t)
	k := random(t)
	expireat, _ := binlog.TTLmsToExpireAt(1000)
	checkint(t, -2, c, "ttl", k)
	checkok(t, c, "set", k, 100)
	checkint(t, 1, c, "expireat", k, expireat/1e3+1000)
	checkintapprox(t, 1000, 5, c, "ttl", k)
	checkintapprox(t, 1000000, 5000, c, "pttl", k)
	checkint(t, 1, c, "del", k)
	checkint(t, -2, c, "ttl", k)
}

func TestPExpireAt(t *testing.T) {
	c := client(t)
	k := random(t)
	expireat, _ := binlog.TTLmsToExpireAt(1000)
	checkint(t, -2, c, "pttl", k)
	checkok(t, c, "set", k, 100)
	checkint(t, 1, c, "pexpireat", k, expireat+100000)
	checkintapprox(t, 100000, 5000, c, "pttl", k)
	checkintapprox(t, 100, 5, c, "ttl", k)
	checkint(t, 1, c, "del", k)
	checkint(t, -2, c, "pttl", k)
}

func TestPersist(t *testing.T) {
	c := client(t)
	k := random(t)
	expireat, _ := binlog.TTLmsToExpireAt(1000)
	checkint(t, -2, c, "pttl", k)
	checkint(t, 0, c, "persist", k)
	checkok(t, c, "set", k, "100")
	checkint(t, 1, c, "pexpireat", k, expireat+100000)
	checkintapprox(t, 100000, 5000, c, "pttl", k)
	checkint(t, 1, c, "persist", k)
	checkint(t, 0, c, "persist", k)
	checkint(t, -1, c, "pttl", k)
}
