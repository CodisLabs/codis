// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"strconv"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

func sdel(t *testing.T, db uint32, key string, expect int64) {
	kdel(t, expect, db, key)
}

func sdump(t *testing.T, db uint32, key string, expect ...string) {
	kexists(t, db, key, 1)
	v, err := testbl.Dump(db, key)
	checkerror(t, err, v != nil)
	x, ok := v.(rdb.Set)
	checkerror(t, nil, ok)
	m := make(map[string]bool)
	for _, s := range expect {
		m[s] = true
	}
	for _, p := range x {
		checkerror(t, nil, m[string(p)])
	}
	scard(t, db, key, int64(len(m)))
}

func srestore(t *testing.T, db uint32, key string, ttlms int64, expect ...string) {
	var x rdb.Set
	for _, s := range expect {
		x = append(x, []byte(s))
	}
	dump, err := rdb.EncodeDump(x)
	checkerror(t, err, true)
	err = testbl.Restore(db, key, ttlms, dump)
	checkerror(t, err, true)
	sdump(t, db, key, expect...)
	if ttlms == 0 {
		kpttl(t, db, key, -1)
	} else {
		kpttl(t, db, key, int64(ttlms))
	}
}

func sadd(t *testing.T, db uint32, key string, expect int64, members ...string) {
	args := []interface{}{key}
	for _, s := range members {
		args = append(args, s)
	}
	x, err := testbl.SAdd(db, args...)
	checkerror(t, err, x == expect)
	for _, s := range members {
		sismember(t, db, key, s, 1)
	}
}

func scard(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.SCard(db, key)
	checkerror(t, err, x == expect)
	if expect == 0 {
		kexists(t, db, key, 0)
	} else {
		kexists(t, db, key, 1)
	}
}

func smembers(t *testing.T, db uint32, key string, expect ...string) {
	x, err := testbl.SMembers(db, key)
	checkerror(t, err, true)
	if len(expect) == 0 {
		checkerror(t, nil, len(x) == 0)
		kexists(t, db, key, 0)
		scard(t, db, key, 0)
	} else {
		m := make(map[string]bool)
		for _, s := range expect {
			m[s] = true
		}
		checkerror(t, nil, len(m) == len(x))
		for _, b := range x {
			checkerror(t, nil, m[string(b)])
		}
		sdump(t, db, key, expect...)
	}
}

func sismember(t *testing.T, db uint32, key, member string, expect int64) {
	x, err := testbl.SIsMember(db, key, member)
	checkerror(t, err, x == expect)
}

func spop(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.SPop(db, key)
	checkerror(t, err, true)
	if expect == 0 {
		checkerror(t, err, x == nil)
		kexists(t, db, key, 0)
	} else {
		sismember(t, db, key, string(x), 0)
	}
}

func srandpop(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.SRandMember(db, key, 1)
	checkerror(t, err, true)
	if expect == 0 {
		checkerror(t, err, len(x) == 0)
		kexists(t, db, key, 0)
	} else {
		checkerror(t, err, len(x) == 1)
		member := string(x[0])
		sismember(t, db, key, member, 1)
		srem(t, db, key, 1, member)
	}
}

func srem(t *testing.T, db uint32, key string, expect int64, members ...string) {
	args := []interface{}{key}
	for _, s := range members {
		args = append(args, s)
	}
	x, err := testbl.SRem(db, args...)
	checkerror(t, err, x == expect)
	for _, s := range members {
		sismember(t, db, key, s, 0)
	}
}

func TestSRestore(t *testing.T) {
	srestore(t, 0, "set", 100, "hello", "world")
	srestore(t, 0, "set", 0, "hello", "world", "!!")
	srestore(t, 0, "set", 10, "z")
	scard(t, 0, "set", 1)
	sleepms(20)
	scard(t, 0, "set", 0)
	kpttl(t, 0, "set", -2)
	checkempty(t)
}

func TestSAdd(t *testing.T) {
	scard(t, 0, "set", 0)
	sadd(t, 0, "set", 1, "0")
	sadd(t, 0, "set", 1, "1")
	sadd(t, 0, "set", 1, "2")
	sadd(t, 0, "set", 0, "0", "1", "2")
	sdump(t, 0, "set", "1", "2", "0")

	kpexpire(t, 0, "set", 1000, 1)
	sadd(t, 0, "set", 1, "3", "2", "1", "0")
	kpttl(t, 0, "set", 1000)
	scard(t, 0, "set", 4)
	sdump(t, 0, "set", "0", "1", "2", "3")
	sadd(t, 0, "set", 0, "0", "1", "2", "3")
	sdel(t, 0, "set", 1)
	sdel(t, 0, "set", 0)
	checkempty(t)
}

func TestSMembers(t *testing.T) {
	sadd(t, 0, "set", 3, "0", "1", "2")
	smembers(t, 0, "set", "0", "1", "2")
	kpexpire(t, 0, "set", 10, 1)
	sleepms(20)
	smembers(t, 0, "set")
	kpexpire(t, 0, "set", 10, 0)
	checkempty(t)
}

func TestSRem(t *testing.T) {
	sadd(t, 0, "set", 5, "x", "y", "0", "1", "2")
	srem(t, 0, "set", 1, "y", "y", "y")
	srem(t, 0, "set", 1, "x")
	srem(t, 0, "set", 0, "x")
	srem(t, 0, "set", 1, "0", "0", "x")
	sdump(t, 0, "set", "1", "2")
	srem(t, 0, "set", 2, "1", "2")
	scard(t, 0, "set", 0)
	kpttl(t, 0, "set", -2)
	checkempty(t)
}

func TestSPop(t *testing.T) {
	for i := 0; i < 32; i++ {
		sadd(t, 0, "set", 1, strconv.Itoa(i))
	}
	for i := 0; i < 32; i++ {
		spop(t, 0, "set", 1)
	}
	spop(t, 0, "set", 0)
	checkempty(t)
}

func TestSRandMember(t *testing.T) {
	for i := 0; i < 32; i++ {
		sadd(t, 0, "set", 1, strconv.Itoa(i))
	}
	for i := 0; i < 32; i++ {
		srandpop(t, 0, "set", 1)
	}
	for i := 0; i < 32; i++ {
		sadd(t, 0, "set", 1, strconv.Itoa(i))
		srandpop(t, 0, "set", 1)
	}
	scard(t, 0, "set", 0)
	checkempty(t)
}
