// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

func ldel(t *testing.T, db uint32, key string, expect int64) {
	kdel(t, expect, db, key)
}

func ldump(t *testing.T, db uint32, key string, expect ...string) {
	kexists(t, db, key, 1)
	v, err := testbl.Dump(db, key)
	checkerror(t, err, v != nil)
	x, ok := v.(rdb.List)
	checkerror(t, nil, ok)
	checkerror(t, nil, len(x) == len(expect))
	for i, v := range expect {
		checkerror(t, nil, v == string(x[i]))
	}
	for i, v := range expect {
		lindex(t, db, key, i, v)
	}
	llen(t, db, key, int64(len(expect)))
}

func llen(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.LLen(db, key)
	checkerror(t, err, expect == x)
	if expect == 0 {
		kexists(t, db, key, 0)
	} else {
		kexists(t, db, key, 1)
	}
}

func lindex(t *testing.T, db uint32, key string, index int, expect string) {
	x, err := testbl.LIndex(db, key, index)
	checkerror(t, err, true)
	if expect == "" {
		checkerror(t, nil, x == nil)
	} else {
		checkerror(t, nil, string(x) == expect)
	}
}

func lrange(t *testing.T, db uint32, key string, beg, end int, expect ...string) {
	x, err := testbl.LRange(db, key, beg, end)
	checkerror(t, err, len(x) == len(expect))
	for i, v := range expect {
		checkerror(t, nil, string(x[i]) == v)
	}
}

func lset(t *testing.T, db uint32, key string, index int, value string) {
	err := testbl.LSet(db, key, index, value)
	checkerror(t, err, true)
	lrange(t, db, key, index, index, value)
	lindex(t, db, key, index, value)
}

func ltrim(t *testing.T, db uint32, key string, beg, end int) {
	err := testbl.LTrim(db, key, beg, end)
	checkerror(t, err, true)
}

func lpop(t *testing.T, db uint32, key string, expect string) {
	x, err := testbl.LPop(db, key)
	checkerror(t, err, true)
	if expect == "" {
		checkerror(t, nil, x == nil)
	} else {
		checkerror(t, nil, string(x) == expect)
	}
}

func rpop(t *testing.T, db uint32, key string, expect string) {
	x, err := testbl.RPop(db, key)
	checkerror(t, err, true)
	if expect == "" {
		checkerror(t, nil, x == nil)
	} else {
		checkerror(t, nil, string(x) == expect)
	}
}

func lpush(t *testing.T, db uint32, key string, expect int64, values ...string) {
	args := []interface{}{key}
	for _, v := range values {
		args = append(args, v)
	}
	x, err := testbl.LPush(db, args...)
	checkerror(t, err, x == expect)
	llen(t, db, key, expect)
}

func rpush(t *testing.T, db uint32, key string, expect int64, values ...string) {
	args := []interface{}{key}
	for _, v := range values {
		args = append(args, v)
	}
	x, err := testbl.RPush(db, args...)
	checkerror(t, err, x == expect)
	llen(t, db, key, expect)
}

func lpushx(t *testing.T, db uint32, key string, value string, expect int64) {
	x, err := testbl.LPushX(db, key, value)
	checkerror(t, err, x == expect)
	llen(t, db, key, expect)
}

func rpushx(t *testing.T, db uint32, key string, value string, expect int64) {
	x, err := testbl.RPushX(db, key, value)
	checkerror(t, err, x == expect)
	llen(t, db, key, expect)
}

func lrestore(t *testing.T, db uint32, key string, ttlms int64, expect ...string) {
	var x rdb.List
	for _, s := range expect {
		x = append(x, []byte(s))
	}
	dump, err := rdb.EncodeDump(x)
	checkerror(t, err, true)
	err = testbl.Restore(db, key, ttlms, dump)
	checkerror(t, err, true)
	ldump(t, db, key, expect...)
	if ttlms == 0 {
		kpttl(t, db, key, -1)
	} else {
		kpttl(t, db, key, int64(ttlms))
	}
}

func TestLRestore(t *testing.T) {
	lrestore(t, 0, "list", 0, "a", "b", "c")
	ldump(t, 0, "list", "a", "b", "c")
	kpttl(t, 0, "list", -1)
	llen(t, 0, "list", 3)

	lrestore(t, 0, "list", 10, "a1", "b1", "c1")
	llen(t, 0, "list", 3)
	sleepms(20)
	llen(t, 0, "list", 0)
	kpttl(t, 0, "list", -2)
	llen(t, 0, "list", 0)
	ldel(t, 0, "list", 0)
	checkempty(t)
}

func TestLIndex(t *testing.T) {
	lindex(t, 0, "list", 0, "")
	lindex(t, 0, "list", 1, "")
	lindex(t, 0, "list", -1, "")
	llen(t, 0, "list", 0)

	lrestore(t, 0, "list", 0, "a", "b", "c")
	llen(t, 0, "list", 3)

	lindex(t, 0, "list", 0, "a")
	lindex(t, 0, "list", 1, "b")
	lindex(t, 0, "list", 2, "c")
	lindex(t, 0, "list", 3, "")

	lindex(t, 0, "list", -1, "c")
	lindex(t, 0, "list", -2, "b")
	lindex(t, 0, "list", -3, "a")
	lindex(t, 0, "list", -4, "")

	ldel(t, 0, "list", 1)
	for i := -4; i <= 4; i++ {
		lindex(t, 0, "list", i, "")
	}
	checkempty(t)
}

func TestLRange(t *testing.T) {
	lrange(t, 0, "list", 0, 0)
	lrange(t, 0, "list", -1, 1)
	lrange(t, 0, "list", 1, -1)

	lrestore(t, 0, "list", 0, "a", "b", "c", "d")
	lrange(t, 0, "list", 0, 0, "a")
	lrange(t, 0, "list", 1, 2, "b", "c")
	lrange(t, 0, "list", 1, -1, "b", "c", "d")
	lrange(t, 0, "list", 2, -2, "c")
	lrange(t, 0, "list", -2, -3)
	lrange(t, 0, "list", -2, -1, "c", "d")
	lrange(t, 0, "list", -100, 2, "a", "b", "c")
	lrange(t, 0, "list", -1000, 1000, "a", "b", "c", "d")
	llen(t, 0, "list", 4)
	ldel(t, 0, "list", 1)
	checkempty(t)
}

func TestRPush(t *testing.T) {
	ss := []string{}
	for i := 0; i < 32; i++ {
		s := strconv.Itoa(i)
		ss = append(ss, s)
		rpush(t, 0, "list", int64(len(ss)), s)
	}
	for i := 0; i < 32; i++ {
		v := []string{}
		for j := 0; j < 4; j++ {
			v = append(v, strconv.Itoa(rand.Int()))
		}
		ss = append(ss, v...)
		rpush(t, 0, "list", int64(len(ss)), v...)
	}
	ldump(t, 0, "list", ss...)
	lrange(t, 0, "list", 0, -1, ss...)
	rpushx(t, 0, "list", "hello", int64(len(ss))+1)
	lindex(t, 0, "list", -1, "hello")
	ldel(t, 0, "list", 1)
	rpushx(t, 0, "list", "world", 0)
	llen(t, 0, "list", 0)
	kexists(t, 0, "list", 0)
	checkempty(t)
}

func TestLPush(t *testing.T) {
	ss := []string{}
	for i := 0; i < 32; i++ {
		s := strconv.Itoa(i)
		ss = append(ss, s)
		lpush(t, 0, "list", int64(len(ss)), s)
	}
	for i := 0; i < 32; i++ {
		v := []string{}
		for j := 0; j < 4; j++ {
			v = append(v, strconv.Itoa(rand.Int()))
		}
		ss = append(ss, v...)
		lpush(t, 0, "list", int64(len(ss)), v...)
	}
	for i, j := 0, len(ss)-1; i < j; i, j = i+1, j-1 {
		ss[i], ss[j] = ss[j], ss[i]
	}
	ldump(t, 0, "list", ss...)
	lrange(t, 0, "list", 0, -1, ss...)
	lpushx(t, 0, "list", "hello", int64(len(ss))+1)
	lindex(t, 0, "list", 0, "hello")
	ldel(t, 0, "list", 1)
	lpushx(t, 0, "list", "world", 0)
	llen(t, 0, "list", 0)
	kexists(t, 0, "list", 0)
	checkempty(t)
}

func TestLPop(t *testing.T) {
	lpop(t, 0, "list", "")
	rpush(t, 0, "list", 4, "a", "b", "c", "d")
	lpop(t, 0, "list", "a")
	lpop(t, 0, "list", "b")
	lpop(t, 0, "list", "c")
	lpush(t, 0, "list", 2, "x")
	lpop(t, 0, "list", "x")
	lpop(t, 0, "list", "d")
	lpop(t, 0, "list", "")
	checkempty(t)
}

func TestRPop(t *testing.T) {
	rpop(t, 0, "list", "")
	lpush(t, 0, "list", 4, "a", "b", "c", "d")
	rpop(t, 0, "list", "a")
	rpop(t, 0, "list", "b")
	rpop(t, 0, "list", "c")
	rpush(t, 0, "list", 2, "x")
	rpop(t, 0, "list", "x")
	rpop(t, 0, "list", "d")
	rpop(t, 0, "list", "")
	checkempty(t)
}

func TestLSet(t *testing.T) {
	ss := []string{}
	for i := 0; i < 128; i++ {
		s := strconv.Itoa(rand.Int())
		ss = append(ss, s)
		rpush(t, 0, "list", int64(len(ss)), s)
	}
	ldump(t, 0, "list", ss...)
	for i := 0; i < 128; i++ {
		ss[i] = strconv.Itoa(i * i)
		lset(t, 0, "list", i, ss[i])
	}
	ldump(t, 0, "list", ss...)
	ltrim(t, 0, "list", 1, 0)
	ldel(t, 0, "list", 0)
	checkempty(t)
}

func TestLTrim(t *testing.T) {
	ss := []string{"a", "b", "c", "d"}
	rpush(t, 0, "list", int64(len(ss)), ss...)
	ltrim(t, 0, "list", 0, -1)
	ltrim(t, 0, "list", 0, len(ss)-1)
	ldump(t, 0, "list", ss...)

	ltrim(t, 0, "list", 1, -1)
	ldump(t, 0, "list", ss[1:]...)
	ltrim(t, 0, "list", 2, 1)
	llen(t, 0, "list", 0)

	rpush(t, 0, "list", int64(len(ss)), ss...)
	ltrim(t, 0, "list", -1, -1)
	ldump(t, 0, "list", ss[len(ss)-1:]...)
	ltrim(t, 0, "list", 2, 1)
	llen(t, 0, "list", 0)

	rpush(t, 0, "list", int64(len(ss)), ss...)
	ltrim(t, 0, "list", 1, -2)
	ldump(t, 0, "list", ss[1:len(ss)-1]...)
	ltrim(t, 0, "list", 2, 1)
	llen(t, 0, "list", 0)

	rpush(t, 0, "list", int64(len(ss)), ss...)
	ltrim(t, 0, "list", -100, 1000)
	ldump(t, 0, "list", ss...)
	ltrim(t, 0, "list", 2, 1)
	llen(t, 0, "list", 0)
	checkempty(t)
}
