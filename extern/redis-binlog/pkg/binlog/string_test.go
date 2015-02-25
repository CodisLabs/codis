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

func xdel(t *testing.T, db uint32, key string, expect int64) {
	kdel(t, expect, db, key)
}

func xdump(t *testing.T, db uint32, key string, expect string) {
	kexists(t, db, key, 1)
	v, err := testbl.Dump(db, key)
	checkerror(t, err, v != nil)
	x, ok := v.(rdb.String)
	checkerror(t, nil, ok && string([]byte(x)) == expect)
	xstrlen(t, db, key, int64(len(expect)))
}

func xrestore(t *testing.T, db uint32, key string, ttlms uint64, value string) {
	var x rdb.String = []byte(value)
	dump, err := rdb.EncodeDump(x)
	checkerror(t, err, true)
	err = testbl.Restore(db, key, ttlms, dump)
	checkerror(t, err, true)
	xdump(t, db, key, value)
	if ttlms == 0 {
		kpttl(t, db, key, -1)
	} else {
		kpttl(t, db, key, int64(ttlms))
	}
}

func xset(t *testing.T, db uint32, key, value string) {
	err := testbl.Set(db, []byte(key), []byte(value))
	checkerror(t, err, true)
	kttl(t, db, key, -1)
	xget(t, db, key, value)
}

func xget(t *testing.T, db uint32, key string, expect string) {
	x, err := testbl.Get(db, []byte(key))
	if expect == "" {
		checkerror(t, err, x == nil)
		xstrlen(t, db, key, 0)
	} else {
		checkerror(t, err, string(x) == expect)
		xstrlen(t, db, key, int64(len(expect)))
	}
}

func xappend(t *testing.T, db uint32, key, value string, expect int64) {
	x, err := testbl.Append(db, key, value)
	checkerror(t, err, x == expect)
}

func xgetset(t *testing.T, db uint32, key, value string, expect string) {
	x, err := testbl.GetSet(db, key, value)
	if expect == "" {
		checkerror(t, err, x == nil)
	} else {
		checkerror(t, err, string(x) == expect)
	}
	kttl(t, db, key, -1)
}

func xpsetex(t *testing.T, db uint32, key, value string, ttlms uint64) {
	err := testbl.PSetEX(db, key, ttlms, value)
	checkerror(t, err, true)
	xdump(t, db, key, value)
	kpttl(t, db, key, int64(ttlms))
}

func xsetex(t *testing.T, db uint32, key, value string, ttls uint64) {
	err := testbl.SetEX(db, key, ttls, value)
	checkerror(t, err, true)
	xdump(t, db, key, value)
	kpttl(t, db, key, int64(ttls*1e3))
}

func xsetnx(t *testing.T, db uint32, key, value string, expect int64) {
	x, err := testbl.SetNX(db, key, value)
	checkerror(t, err, x == expect)
	if expect != 0 {
		xdump(t, db, key, value)
		kpttl(t, db, key, -1)
	}
}

func xstrlen(t *testing.T, db uint32, key string, expect int64) {
	if expect != 0 {
		kexists(t, db, key, 1)
	} else {
		kexists(t, db, key, 0)
	}
	x, err := testbl.Strlen(db, key)
	checkerror(t, err, x == expect)
}

func xincr(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.Incr(db, key)
	checkerror(t, err, x == expect)
}

func xdecr(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.Decr(db, key)
	checkerror(t, err, x == expect)
}

func xincrby(t *testing.T, db uint32, key string, delta int64, expect int64) {
	x, err := testbl.IncrBy(db, key, delta)
	checkerror(t, err, x == expect)
}

func xdecrby(t *testing.T, db uint32, key string, delta int64, expect int64) {
	x, err := testbl.DecrBy(db, key, delta)
	checkerror(t, err, x == expect)
}

func xincrbyfloat(t *testing.T, db uint32, key string, delta float64, expect float64) {
	x, err := testbl.IncrByFloat(db, key, delta)
	checkerror(t, err, math.Abs(x-expect) < 1e-9)
}

func xsetbit(t *testing.T, db uint32, key string, offset uint, value int64, expect int64) {
	x, err := testbl.SetBit(db, key, offset, value)
	checkerror(t, err, x == expect)
	xgetbit(t, db, key, offset, value)
}

func xgetbit(t *testing.T, db uint32, key string, offset uint, expect int64) {
	x, err := testbl.GetBit(db, key, offset)
	checkerror(t, err, x == expect)
}

func xsetrange(t *testing.T, db uint32, key string, offset uint, value string, expect int64) {
	x, err := testbl.SetRange(db, key, offset, value)
	checkerror(t, err, x == expect)
	xgetrange(t, db, key, int(offset), int(offset)+len(value)-1, value)
}

func xgetrange(t *testing.T, db uint32, key string, beg, end int, expect string) {
	x, err := testbl.GetRange(db, key, beg, end)
	checkerror(t, err, string(x) == expect)
}

func xmset(t *testing.T, db uint32, pairs ...string) {
	args := make([]interface{}, len(pairs))
	for i, s := range pairs {
		args[i] = s
	}
	err := testbl.MSet(db, args...)
	checkerror(t, err, true)
	m := make(map[string]string)
	for i := 0; i < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	for key, value := range m {
		xget(t, db, key, value)
		kttl(t, db, key, -1)
	}
}

func xmsetnx(t *testing.T, expect int64, db uint32, pairs ...string) {
	args := make([]interface{}, len(pairs))
	for i, s := range pairs {
		args[i] = s
	}
	x, err := testbl.MSetNX(db, args...)
	checkerror(t, err, x == expect)
	if expect == 0 {
		return
	}
	m := make(map[string]string)
	for i := 0; i < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	for key, value := range m {
		xget(t, db, key, value)
		kttl(t, db, key, -1)
	}
}

func xmget(t *testing.T, db uint32, pairs ...string) {
	checkerror(t, nil, len(pairs)%2 == 0)
	var args []interface{}
	for i := 0; i < len(pairs); i += 2 {
		args = append(args, pairs[i])
	}
	x, err := testbl.MGet(db, args...)
	checkerror(t, err, len(x) == len(args))
	for i := 0; i < len(pairs); i += 2 {
		value := pairs[i+1]
		if value == "" {
			checkerror(t, nil, x[i/2] == nil)
		} else {
			checkerror(t, nil, string(x[i/2]) == value)
		}
	}
}

func TestXRestore(t *testing.T) {
	xrestore(t, 0, "string", 0, "hello")
	xrestore(t, 0, "string", 0, "world")
	xget(t, 0, "string", "world")
	xrestore(t, 0, "string", 10, "hello")
	sleepms(20)
	kpttl(t, 0, "string", -2)

	xrestore(t, 0, "string", 10, "test")
	xget(t, 0, "string", "test")
	sleepms(20)
	xget(t, 0, "string", "")
	checkempty(t)
}

func TestXSet(t *testing.T) {
	xset(t, 0, "string", "hello")
	xdel(t, 0, "string", 1)
	xdel(t, 0, "string", 0)
	xget(t, 0, "string", "")

	kpexpire(t, 0, "string", 100, 0)
	kpttl(t, 0, "string", -2)

	xset(t, 0, "string", "test")
	kpttl(t, 0, "string", -1)
	kpexpire(t, 0, "string", 1000, 1)
	kpexpire(t, 0, "string", 2000, 1)

	xset(t, 0, "string", "test")
	kpersist(t, 0, "string", 0)
	kpexpire(t, 0, "string", 1000, 1)
	kpersist(t, 0, "string", 1)

	xset(t, 0, "string", "test2")
	xdel(t, 0, "string", 1)
	kpttl(t, 0, "string", -2)
	checkempty(t)
}

func TestXAppend(t *testing.T) {
	xset(t, 0, "string", "hello")
	xget(t, 0, "string", "hello")
	xappend(t, 0, "string", " ", 6)

	xget(t, 0, "string", "hello ")
	xappend(t, 0, "string", "world!!", 13)

	xget(t, 0, "string", "hello world!!")
	xdel(t, 0, "string", 1)
	xget(t, 0, "string", "")

	xappend(t, 0, "string", "test", 4)
	xget(t, 0, "string", "test")

	xdel(t, 0, "string", 1)

	expect := ""
	for i := 0; i < 1024; i++ {
		s := strconv.Itoa(i) + ","
		expect += s
		xappend(t, 0, "string", s, int64(len(expect)))
	}
	xdump(t, 0, "string", expect)
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestXSetEX(t *testing.T) {
	xsetex(t, 0, "string", "hello", 1)
	kpttl(t, 0, "string", 1000)

	xset(t, 0, "string", "hello")
	kpttl(t, 0, "string", -1)

	xsetex(t, 0, "string", "world", 100)
	xget(t, 0, "string", "world")
	kpttl(t, 0, "string", 100000)
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestXPSetEX(t *testing.T) {
	xpsetex(t, 0, "string", "hello", 1000)
	kpttl(t, 0, "string", 1000)
	xpsetex(t, 0, "string", "world", 2000)
	kpttl(t, 0, "string", 2000)
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestXSetNX(t *testing.T) {
	xset(t, 0, "string", "hello")

	xsetnx(t, 0, "string", "world", 0)
	xget(t, 0, "string", "hello")
	xdel(t, 0, "string", 1)

	xsetnx(t, 0, "string", "world", 1)
	xdel(t, 0, "string", 1)
	xdel(t, 0, "string", 0)
	checkempty(t)
}

func TestXGetSet(t *testing.T) {
	xgetset(t, 0, "string", "hello", "")
	xget(t, 0, "string", "hello")
	kpttl(t, 0, "string", -1)

	kpexpire(t, 0, "string", 1000, 1)
	xgetset(t, 0, "string", "world", "hello")
	kpttl(t, 0, "string", -1)

	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestXIncrDecr(t *testing.T) {
	for i := 0; i < 32; i++ {
		xincr(t, 0, "string", int64(i)+1)
	}
	xget(t, 0, "string", "32")

	kpexpire(t, 0, "string", 10000, 1)
	for i := 0; i < 32; i++ {
		xdecr(t, 0, "string", 31-int64(i))
	}
	xget(t, 0, "string", "0")
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestXIncrBy(t *testing.T) {
	sum := int64(0)
	for i := 0; i < 32; i++ {
		a := rand.Int63()
		sum += a
		xincrby(t, 0, "string", a, sum)
	}
	for i := 0; i < 32; i++ {
		a := rand.Int63()
		sum -= a
		xdecrby(t, 0, "string", a, sum)
	}
	xget(t, 0, "string", strconv.Itoa(int(sum)))
	xdel(t, 0, "string", 1)
	checkempty(t)

}

func TestXIncrByFloat(t *testing.T) {
	sum := float64(0)
	for i := 0; i < 128; i++ {
		a := rand.Float64()
		sum += a
		xincrbyfloat(t, 0, "string", a, sum)
	}
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestXSetBit(t *testing.T) {
	xsetbit(t, 0, "string", 0, 1, 0)
	xget(t, 0, "string", "\x01")
	xsetbit(t, 0, "string", 1, 1, 0)
	xget(t, 0, "string", "\x03")
	xsetbit(t, 0, "string", 2, 1, 0)
	xget(t, 0, "string", "\x07")
	xsetbit(t, 0, "string", 3, 1, 0)
	xget(t, 0, "string", "\x0f")
	xsetbit(t, 0, "string", 4, 1, 0)
	xget(t, 0, "string", "\x1f")
	xsetbit(t, 0, "string", 5, 1, 0)
	xget(t, 0, "string", "\x3f")
	xsetbit(t, 0, "string", 6, 1, 0)
	xget(t, 0, "string", "\x7f")
	xsetbit(t, 0, "string", 7, 1, 0)
	xget(t, 0, "string", "\xff")
	xsetbit(t, 0, "string", 8, 1, 0)
	xget(t, 0, "string", "\xff\x01")
	xsetbit(t, 0, "string", 0, 0, 1)
	xget(t, 0, "string", "\xfe\x01")
	xsetbit(t, 0, "string", 1, 0, 1)
	xget(t, 0, "string", "\xfc\x01")
	xsetbit(t, 0, "string", 2, 0, 1)
	xget(t, 0, "string", "\xf8\x01")
	xsetbit(t, 0, "string", 3, 0, 1)
	xget(t, 0, "string", "\xf0\x01")
	xsetbit(t, 0, "string", 4, 0, 1)
	xget(t, 0, "string", "\xe0\x01")
	xsetbit(t, 0, "string", 5, 0, 1)
	xget(t, 0, "string", "\xc0\x01")
	xsetbit(t, 0, "string", 6, 0, 1)
	xget(t, 0, "string", "\x80\x01")
	xsetbit(t, 0, "string", 7, 0, 1)
	xget(t, 0, "string", "\x00\x01")
	xsetbit(t, 0, "string", 8, 0, 1)
	xget(t, 0, "string", "\x00\x00")
	xsetbit(t, 0, "string", 16, 0, 0)
	xget(t, 0, "string", "\x00\x00\x00")
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestXSetRange(t *testing.T) {
	xsetrange(t, 0, "string", 1, "hello", 6)
	xget(t, 0, "string", "\x00hello")
	xsetrange(t, 0, "string", 7, "world", 12)
	xget(t, 0, "string", "\x00hello\x00world")
	xsetrange(t, 0, "string", 2, "test1test2test3", 17)
	xget(t, 0, "string", "\x00htest1test2test3")
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestGetBit(t *testing.T) {
	xgetbit(t, 0, "string", 0, 0)
	xgetbit(t, 0, "string", 1000, 0)
	xset(t, 0, "string", "\x01\x03")
	xgetbit(t, 0, "string", 0, 1)
	xgetbit(t, 0, "string", 1, 0)
	xgetbit(t, 0, "string", 8, 1)
	xgetbit(t, 0, "string", 9, 1)
	xdel(t, 0, "string", 1)

	for i := 0; i < 32; i += 2 {
		xsetbit(t, 0, "string", uint(i), 1, 0)
		xsetbit(t, 0, "string", uint(i), 1, 1)
	}
	for i := 0; i < 32; i++ {
		v := int64(1)
		if i%2 != 0 {
			v = 0
		}
		xgetbit(t, 0, "string", uint(i), v)
	}
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestGetRange(t *testing.T) {
	xgetrange(t, 0, "string", 0, 0, "")
	xgetrange(t, 0, "string", 100, -100, "")
	xgetrange(t, 0, "string", -100, 100, "")
	xset(t, 0, "string", "hello world!!")
	xgetrange(t, 0, "string", 0, 3, "hell")
	xgetrange(t, 0, "string", 2, 1, "")
	xgetrange(t, 0, "string", -12, 3, "ell")
	xgetrange(t, 0, "string", -100, 3, "hell")
	xgetrange(t, 0, "string", -1, 10000, "!")
	xgetrange(t, 0, "string", -1, -1, "!")
	xgetrange(t, 0, "string", -1, -2, "")
	xgetrange(t, 0, "string", -1, -1000, "")
	xgetrange(t, 0, "string", -100, 100, "hello world!!")
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestXMSet(t *testing.T) {
	xmset(t, 0, "a", "1", "b", "2", "c", "3", "a", "4", "b", "5", "c", "6")
	xget(t, 0, "a", "4")
	xget(t, 0, "b", "5")
	xget(t, 0, "c", "6")

	kpexpire(t, 0, "a", 1000, 1)
	xmset(t, 0, "a", "x")
	kpttl(t, 0, "a", -1)
	xget(t, 0, "a", "x")

	xmset(t, 0, "a", "1", "a", "2", "a", "3", "b", "1", "b", "2")
	kdel(t, 3, 0, "a", "b", "c")
	checkempty(t)
}

func TestMSetNX(t *testing.T) {
	xsetex(t, 0, "string", "hello", 100)
	xmsetnx(t, 0, 0, "string", "world", "string2", "blabla")
	xget(t, 0, "string", "hello")

	xsetex(t, 0, "string", "hello1", 1)
	kpttl(t, 0, "string", 1000)
	kpexpire(t, 0, "string", 10, 1)
	sleepms(20)
	xget(t, 0, "string", "")

	xmsetnx(t, 1, 0, "string", "world1")
	xget(t, 0, "string", "world1")
	kpttl(t, 0, "string", -1)

	kpexpire(t, 0, "string", 10, 1)
	sleepms(20)

	xmsetnx(t, 1, 0, "string", "hello", "string", "world")
	xget(t, 0, "string", "world")
	xdel(t, 0, "string", 1)
	checkempty(t)
}

func TestMGet(t *testing.T) {
	xmsetnx(t, 1, 0, "a", "1", "b", "2", "c", "3")
	kpexpire(t, 0, "a", 10, 1)
	sleepms(20)

	xmget(t, 0, "a", "", "b", "2", "c", "3", "d", "")
	kdel(t, 2, 0, "a", "b", "c")
	checkempty(t)
}
