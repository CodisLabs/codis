// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"math"
	"testing"
)

func kdel(t *testing.T, expect int64, db uint32, keys ...string) {
	args := make([]interface{}, len(keys))
	for i, key := range keys {
		args[i] = key
	}
	n, err := testbl.Del(db, args...)
	checkerror(t, err, n == expect)
	for _, key := range keys {
		kexists(t, db, key, 0)
	}
}

func ktype(t *testing.T, db uint32, key string, expect ObjectCode) {
	c, err := testbl.Type(db, key)
	checkerror(t, err, c == expect)
	if expect == 0 {
		kexists(t, db, key, 0)
	} else {
		kexists(t, db, key, 1)
	}
}

func kexists(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.Exists(db, key)
	checkerror(t, err, x == expect)
}

func kttl(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.TTL(db, key)
	switch expect {
	case -1, -2, 0:
		checkerror(t, err, x == expect)
	default:
		checkerror(t, err, math.Abs(float64(expect-x)) < 5)
	}
}

func kpttl(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.PTTL(db, key)
	switch expect {
	case -1, -2, 0:
		checkerror(t, err, x == expect)
	default:
		checkerror(t, err, math.Abs(float64(expect-x)) < 50)
	}
}

func kpersist(t *testing.T, db uint32, key string, expect int64) {
	x, err := testbl.Persist(db, key)
	checkerror(t, err, x == expect)
	if expect != 0 {
		kpttl(t, db, key, -1)
	}
}

func kexpire(t *testing.T, db uint32, key string, ttls uint64, expect int64) {
	x, err := testbl.Expire(db, key, ttls)
	checkerror(t, err, x == expect)
	if expect != 0 {
		if ttls == 0 {
			kpttl(t, db, key, -2)
		} else {
			kpttl(t, db, key, int64(ttls*1e3))
		}
	}
}

func kpexpire(t *testing.T, db uint32, key string, ttlms uint64, expect int64) {
	x, err := testbl.PExpire(db, key, ttlms)
	checkerror(t, err, x == expect)
	if expect != 0 {
		if ttlms == 0 {
			kpttl(t, db, key, -2)
		} else {
			kpttl(t, db, key, int64(ttlms))
		}
	}
}

func kexpireat(t *testing.T, db uint32, key string, timestamp uint64, expect int64) {
	x, err := testbl.ExpireAt(db, key, timestamp)
	checkerror(t, err, x == expect)
	if expect != 0 {
		expireat := timestamp * 1e3
		if now := nowms(); expireat < now {
			kpttl(t, db, key, -2)
		} else {
			kpttl(t, db, key, int64(expireat-now))
		}
	}
}

func kpexpireat(t *testing.T, db uint32, key string, expireat uint64, expect int64) {
	x, err := testbl.PExpireAt(db, key, expireat)
	checkerror(t, err, x == expect)
	if expect != 0 {
		if now := nowms(); expireat < now {
			kpttl(t, db, key, -2)
		} else {
			kpttl(t, db, key, int64(expireat-now))
		}
	}
}

func TestDel(t *testing.T) {
	kdel(t, 0, 0, "a", "b", "c", "d")
	xset(t, 0, "a", "a")
	xset(t, 0, "b", "b")
	kdel(t, 2, 0, "a", "b", "c", "d")
	checkempty(t)
}

func TestExists(t *testing.T) {
	kexists(t, 0, "a", 0)
	xset(t, 0, "a", "a")
	kexists(t, 0, "a", 1)
	kdel(t, 1, 0, "a")
	kexists(t, 0, "a", 0)
	checkempty(t)
}

func TestTTL(t *testing.T) {
	kttl(t, 0, "a", -2)
	xset(t, 0, "a", "a")
	kttl(t, 0, "a", -1)

	kexpireat(t, 0, "a", nowms()/1e3+100, 1)
	kttl(t, 0, "a", 100)

	kpexpireat(t, 0, "a", nowms()+100, 1)
	kttl(t, 0, "a", 0)

	kpexpireat(t, 0, "a", nowms()+100000, 1)
	kttl(t, 0, "a", 100)

	kpexpireat(t, 0, "a", nowms()+10, 1)
	kttl(t, 0, "a", 0)
	kexists(t, 0, "a", 1)
	sleepms(20)
	kttl(t, 0, "a", -2)
	kexists(t, 0, "a", 0)
	checkempty(t)
}

func TestPTTL(t *testing.T) {
	kpttl(t, 0, "a", -2)
	xset(t, 0, "a", "a")
	kpttl(t, 0, "a", -1)

	kpexpireat(t, 0, "a", nowms()+100, 1)
	kpttl(t, 0, "a", 100)

	kpexpireat(t, 0, "a", nowms()+100, 1)
	kpttl(t, 0, "a", 100)
	kexists(t, 0, "a", 1)

	kpexpireat(t, 0, "a", nowms()+10, 1)
	sleepms(20)
	kpttl(t, 0, "a", -2)
	kexists(t, 0, "a", 0)
	checkempty(t)
}

func TestPersist(t *testing.T) {
	kpersist(t, 0, "a", 0)
	xset(t, 0, "a", "a")
	kpexpireat(t, 0, "a", nowms()+100, 1)
	kpersist(t, 0, "a", 1)

	kpexpireat(t, 0, "a", nowms()+10, 1)
	sleepms(20)
	kpersist(t, 0, "a", 0)
	kexists(t, 0, "a", 0)
	checkempty(t)
}

func TestExpire(t *testing.T) {
	kexpire(t, 0, "a", 10, 0)
	xset(t, 0, "a", "a")
	kexpire(t, 0, "a", 10, 1)
	kttl(t, 0, "a", 10)

	kexpire(t, 0, "a", 1, 1)
	kpttl(t, 0, "a", 1000)
	kpersist(t, 0, "a", 1)

	kexpireat(t, 0, "a", 0, 1)
	ktype(t, 0, "a", 0)
	checkempty(t)
}

func TestPExpire(t *testing.T) {
	kpexpire(t, 0, "a", 10, 0)
	xset(t, 0, "a", "a")
	kpexpire(t, 0, "a", 100, 1)
	kttl(t, 0, "a", 0)
	kpttl(t, 0, "a", 100)

	kpexpire(t, 0, "a", 10, 1)
	sleepms(20)
	ktype(t, 0, "a", 0)
	checkempty(t)
}

func TestExpireAt(t *testing.T) {
	kexpireat(t, 0, "a", nowms()/1e3+100, 0)
	xset(t, 0, "a", "a")
	kexpireat(t, 0, "a", nowms()/1e3+100, 1)
	ktype(t, 0, "a", StringCode)

	kexpireat(t, 0, "a", nowms()/1e3-20, 1)
	kexists(t, 0, "a", 0)

	xset(t, 0, "a", "a")
	kexpireat(t, 0, "a", 0, 1)
	kexists(t, 0, "a", 0)
	checkempty(t)
}

func TestPExpireAt(t *testing.T) {
	kpexpireat(t, 0, "a", nowms()+100, 0)
	xset(t, 0, "a", "a")
	kpexpireat(t, 0, "a", nowms()+100, 1)
	ktype(t, 0, "a", StringCode)

	kpexpireat(t, 0, "a", nowms()-20, 1)
	kexists(t, 0, "a", 0)

	xset(t, 0, "a", "a")
	kpexpireat(t, 0, "a", 0, 1)
	kexists(t, 0, "a", 0)
	checkempty(t)
}

func TestRestore(t *testing.T) {
	lpush(t, 0, "key", 1, "a")
	xrestore(t, 0, "key", 1000, "hello")
	hrestore(t, 0, "key", 2000, "a", "b", "b", "a")
	zrestore(t, 0, "key", 3000, "z0", 100, "z1", 1000)
	lrestore(t, 0, "key", 4000, "l0", "l1", "l2")
	srestore(t, 0, "key", 10, "a", "b", "c", "d")
	sleepms(20)
	kexists(t, 0, "key", 0)
	checkempty(t)
}

/*
// TODO
func (b *Binlog) Restore(db uint32, args ...interface{}) error {
*/
