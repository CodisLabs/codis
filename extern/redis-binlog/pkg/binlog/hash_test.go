// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

func hdelall(t *testing.T, db uint32, key string, expect int64) {
	kdel(t, expect, db, key)
}

func hdump(t *testing.T, db uint32, key string, expect ...string) {
	kexists(t, db, key, 1)
	v, err := testbl.Dump(db, key)
	checkerror(t, err, v != nil)
	x, ok := v.(rdb.Hash)
	checkerror(t, nil, ok)
	checkerror(t, nil, len(expect)%2 == 0)
	m := make(map[string]string)
	for i := 0; i < len(expect); i += 2 {
		m[expect[i]] = expect[i+1]
	}
	for _, e := range x {
		checkerror(t, nil, m[string(e.Field)] == string(e.Value))
	}
	for k, v := range m {
		hget(t, db, key, k, v)
	}
	hlen(t, db, key, int64(len(m)))
}

func hrestore(t *testing.T, db uint32, key string, ttlms int64, expect ...string) {
	checkerror(t, nil, len(expect)%2 == 0)
	var x rdb.Hash
	for i := 0; i < len(expect); i += 2 {
		x = append(x, &rdb.HashElement{Field: []byte(expect[i]), Value: []byte(expect[i+1])})
	}
	dump, err := rdb.EncodeDump(x)
	checkerror(t, err, true)
	err = testbl.Restore(db, key, ttlms, dump)
	checkerror(t, err, true)
	hdump(t, db, key, expect...)
	if ttlms == 0 {
		kpttl(t, db, key, -1)
	} else {
		kpttl(t, db, key, int64(ttlms))
	}
}

func hlen(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.HLen(db, key)
	checkerror(t, err, x == expect)
	if expect == 0 {
		kexists(t, db, key, 0)
	} else {
		kexists(t, db, key, 1)
	}
}

func hdel(t *testing.T, db uint32, key string, expect int64, fields ...string) {
	args := []interface{}{key}
	for _, s := range fields {
		args = append(args, s)
	}
	x, err := testbl.HDel(db, args...)
	checkerror(t, err, x == expect)
	for _, s := range fields {
		hexists(t, db, key, s, 0)
	}
}

func hexists(t *testing.T, db uint32, key, field string, expect int64) {
	x, err := testbl.HExists(db, key, field)
	checkerror(t, err, x == expect)
}

func hgetall(t *testing.T, db uint32, key string, expect ...string) {
	x, err := testbl.HGetAll(db, key)
	checkerror(t, err, true)
	if len(expect) == 0 {
		checkerror(t, nil, len(x) == 0)
		kexists(t, db, key, 0)
	} else {
		checkerror(t, nil, len(expect)%2 == 0)
		m := make(map[string]string)
		for i := 0; i < len(expect); i += 2 {
			m[expect[i]] = expect[i+1]
		}
		checkerror(t, nil, len(x) == len(expect))
		fields, values := []string{}, []string{}
		for i := 0; i < len(x); i += 2 {
			f, v := string(x[i]), string(x[i+1])
			checkerror(t, nil, m[f] == v)
			hget(t, db, key, f, v)
			fields = append(fields, f)
			values = append(values, v)
		}
		hdump(t, db, key, expect...)
		hkeys(t, db, key, fields...)
		hvals(t, db, key, values...)
	}
}

func hget(t *testing.T, db uint32, key, field string, expect string) {
	x, err := testbl.HGet(db, key, field)
	checkerror(t, err, true)
	if expect == "" {
		checkerror(t, nil, x == nil)
		hexists(t, db, key, field, 0)
	} else {
		checkerror(t, nil, expect == string(x))
		hexists(t, db, key, field, 1)
	}
}

func hkeys(t *testing.T, db uint32, key string, expect ...string) {
	x, err := testbl.HKeys(db, key)
	checkerror(t, err, true)
	if len(expect) == 0 {
		checkerror(t, nil, len(x) == 0)
		kexists(t, db, key, 0)
		hlen(t, db, key, 0)
	} else {
		checkerror(t, nil, len(expect) == len(x))
		m := make(map[string]bool)
		for _, s := range expect {
			m[s] = true
		}
		for _, b := range x {
			checkerror(t, nil, m[string(b)])
		}
		for _, s := range expect {
			hexists(t, db, key, s, 1)
		}
		hlen(t, db, key, int64(len(expect)))
	}
}

func hvals(t *testing.T, db uint32, key string, expect ...string) {
	x, err := testbl.HVals(db, key)
	checkerror(t, err, true)
	if len(expect) == 0 {
		checkerror(t, nil, len(x) == 0)
		kexists(t, db, key, 0)
		hlen(t, db, key, 0)
	} else {
		checkerror(t, nil, len(expect) == len(x))
		m1 := make(map[string]int)
		for _, s := range expect {
			m1[s]++
		}
		m2 := make(map[string]int)
		for _, b := range x {
			m2[string(b)]++
		}
		checkerror(t, nil, len(m1) == len(m2))
		for k, v := range m2 {
			checkerror(t, nil, m1[k] == v)
		}
		hlen(t, db, key, int64(len(expect)))
	}
}

func hincrby(t *testing.T, db uint32, key, field string, delta int64, expect int64) {
	x, err := testbl.HIncrBy(db, key, field, delta)
	checkerror(t, err, x == expect)
}

func hincrbyfloat(t *testing.T, db uint32, key, field string, delta float64, expect float64) {
	x, err := testbl.HIncrByFloat(db, key, field, delta)
	checkerror(t, err, math.Abs(x-expect) < 1e-9)
}

func hset(t *testing.T, db uint32, key, field, value string, expect int64) {
	x, err := testbl.HSet(db, key, field, value)
	checkerror(t, err, x == expect)
	hget(t, db, key, field, value)
}

func hsetnx(t *testing.T, db uint32, key, field, value string, expect int64) {
	x, err := testbl.HSetNX(db, key, field, value)
	checkerror(t, err, expect == x)
	hexists(t, db, key, field, 1)
	if expect != 0 {
		hget(t, db, key, field, value)
	}
}

func hmset(t *testing.T, db uint32, key string, pairs ...string) {
	checkerror(t, nil, len(pairs)%2 == 0)
	args := []interface{}{key}
	for i := 0; i < len(pairs); i++ {
		args = append(args, pairs[i])
	}
	err := testbl.HMSet(db, args...)
	checkerror(t, err, true)
	for i := 0; i < len(pairs); i += 2 {
		hget(t, db, key, pairs[i], pairs[i+1])
	}
}

func hmget(t *testing.T, db uint32, key string, pairs ...string) {
	checkerror(t, nil, len(pairs)%2 == 0)
	args := []interface{}{key}
	for i := 0; i < len(pairs); i += 2 {
		args = append(args, pairs[i])
	}
	x, err := testbl.HMGet(db, args...)
	checkerror(t, err, len(x) == len(pairs)/2)
	for i, b := range x {
		v := pairs[i*2+1]
		if len(v) == 0 {
			checkerror(t, nil, b == nil)
		} else {
			checkerror(t, nil, string(b) == v)
		}
	}
}

func TestHSet(t *testing.T) {
	ss := []string{}
	ks := []string{}
	vs := []string{}
	for i := 0; i < 32; i++ {
		k := strconv.Itoa(i)
		v := strconv.Itoa(rand.Int())
		ss = append(ss, k, v)
		ks = append(ks, k)
		vs = append(vs, v)
		hset(t, 0, "hash", k, strconv.Itoa(rand.Int()), 1)
		hset(t, 0, "hash", k, v, 0)
		hget(t, 0, "hash", k, v)
	}
	hkeys(t, 0, "hash", ks...)
	hvals(t, 0, "hash", vs...)
	hgetall(t, 0, "hash", ss...)
	hdelall(t, 0, "hash", 1)
	hgetall(t, 0, "hash")
	checkempty(t)
}

func TestHDel(t *testing.T) {
	ss := []string{}
	for i := 0; i < 32; i++ {
		k := strconv.Itoa(i)
		v := strconv.Itoa(rand.Int())
		ss = append(ss, k, v)
	}
	hmset(t, 0, "hash", ss...)
	hgetall(t, 0, "hash", ss...)

	hdel(t, 0, "hash", 2, "0", "1")
	hdel(t, 0, "hash", 1, "2", "2", "2")
	hdel(t, 0, "hash", 0, "0", "1", "2", "0", "1", "2")

	hlen(t, 0, "hash", int64(len(ss)/2)-3)
	hgetall(t, 0, "hash", ss[6:]...)
	kpexpire(t, 0, "hash", 10, 1)
	sleepms(20)
	hdelall(t, 0, "hash", 0)

	for i := 0; i < 10; i++ {
		hset(t, 0, "hash", strconv.Itoa(i), strconv.Itoa(rand.Int()), 1)
	}
	for i := 0; i < 10; i++ {
		hdel(t, 0, "hash", 1, strconv.Itoa(i))
		hdel(t, 0, "hash", 0, strconv.Itoa(i))
	}
	hgetall(t, 0, "hash")
	checkempty(t)
}

func TestHRestore(t *testing.T) {
	ss := []string{}
	for i := 0; i < 32; i++ {
		k := strconv.Itoa(i)
		v := strconv.Itoa(rand.Int())
		ss = append(ss, k, v)
	}

	hrestore(t, 0, "hash", 0, ss...)
	hgetall(t, 0, "hash", ss...)
	kpttl(t, 0, "hash", -1)

	for i := 0; i < len(ss); i++ {
		ss[i] = strconv.Itoa(rand.Int())
	}

	hrestore(t, 0, "hash", 10, ss...)
	hgetall(t, 0, "hash", ss...)
	sleepms(20)
	hlen(t, 0, "hash", 0)
	kpttl(t, 0, "hash", -2)
	hdelall(t, 0, "hash", 0)
	checkempty(t)
}

func TestHIncrBy(t *testing.T) {
	hincrby(t, 0, "hash", "a", 100, 100)
	hincrby(t, 0, "hash", "a", -100, 0)
	hset(t, 0, "hash", "a", "1000", 0)
	hincrby(t, 0, "hash", "a", -1000, 0)
	hgetall(t, 0, "hash", "a", "0")
	hdelall(t, 0, "hash", 1)
	checkempty(t)
}

func TestHIncrFloat(t *testing.T) {
	hincrbyfloat(t, 0, "hash", "a", 100.5, 100.5)
	hincrbyfloat(t, 0, "hash", "a", 10000, 10100.5)
	hset(t, 0, "hash", "a", "300", 0)
	hincrbyfloat(t, 0, "hash", "a", 3.14, 303.14)
	hincrbyfloat(t, 0, "hash", "a", -303.14, 0)
	hdelall(t, 0, "hash", 1)
	checkempty(t)
}

func TestHSetNX(t *testing.T) {
	for i := 0; i < 16; i++ {
		k := strconv.Itoa(i)
		v := strconv.Itoa(rand.Int())
		hsetnx(t, 0, "hash", k, v, 1)
	}
	hsetnx(t, 0, "hash", "0", "0", 0)
	hsetnx(t, 0, "hash", "128", "128", 1)
	hsetnx(t, 0, "hash", "129", "129", 1)
	hsetnx(t, 0, "hash", "129", "129", 0)
	hlen(t, 0, "hash", 18)

	kpexpire(t, 0, "hash", 10, 1)
	sleepms(20)
	hsetnx(t, 0, "hash", "0", "1", 1)
	hsetnx(t, 0, "hash", "0", "2", 0)
	hdel(t, 0, "hash", 1, "0")
	hsetnx(t, 0, "hash", "0", "3", 1)
	hdel(t, 0, "hash", 1, "0")
	hlen(t, 0, "hash", 0)

	hsetnx(t, 0, "hash", "0", "a", 1)
	hsetnx(t, 0, "hash", "0", "b", 0)
	hsetnx(t, 0, "hash", "1", "c", 1)
	hsetnx(t, 0, "hash", "1", "d", 0)
	hsetnx(t, 0, "hash", "2", "a", 1)
	hsetnx(t, 0, "hash", "2", "c", 0)
	hvals(t, 0, "hash", "a", "a", "c")
	hkeys(t, 0, "hash", "0", "1", "2")

	hdel(t, 0, "hash", 1, "0")
	hsetnx(t, 0, "hash", "0", "x", 1)
	hlen(t, 0, "hash", 3)
	hmget(t, 0, "hash", "0", "x", "1", "c", "2", "a")
	hdelall(t, 0, "hash", 1)
	checkempty(t)
}

func TestHMSet(t *testing.T) {
	hset(t, 0, "hash", "a", "0", 1)
	hmset(t, 0, "hash", "b", "1", "c", "2")
	hmget(t, 0, "hash", "a", "0", "a", "0", "x", "")
	hdel(t, 0, "hash", 1, "a")
	hmget(t, 0, "hash", "a", "", "b", "1")
	hgetall(t, 0, "hash", "b", "1", "c", "2")
	hdelall(t, 0, "hash", 1)
	hmget(t, 0, "hash", "a", "")
	checkempty(t)
}
