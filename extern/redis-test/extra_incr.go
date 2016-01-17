// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
)

type ExtraIncrTestCase struct {
	proxy   string
	master1 string
	slave1  string
	master2 string
	slave2  string
	group   int
	round   int
	nkeys   int
	ntags   int
}

func init() {
	testcase = &ExtraIncrTestCase{}
}

func (tc *ExtraIncrTestCase) init() {
	flag.StringVar(&tc.proxy, "proxy", "", "redis host:port")
	flag.StringVar(&tc.master1, "master1", "", "redis host:port")
	flag.StringVar(&tc.slave1, "slave1", "", "redis host:port")
	flag.StringVar(&tc.master2, "master2", "", "redis host:port")
	flag.StringVar(&tc.slave2, "slave2", "", "redis host:port")
	flag.IntVar(&tc.group, "group", 8, "# of test players")
	flag.IntVar(&tc.round, "round", 100, "# of incr opts per key")
	flag.IntVar(&tc.nkeys, "nkeys", 10000, "# of keys per test")
	flag.IntVar(&tc.ntags, "ntags", 1000, "# tags")
}

func (tc *ExtraIncrTestCase) main() {
	tg := &TestGroup{}
	tg.Reset()
	var tags = NewZeroTags(tc.ntags)
	for g := 0; g < tc.group; g++ {
		tg.AddPlayer()
		go tc.player(g, tg, tags)
	}
	tg.Start()
	tg.Wait()
	fmt.Println("done")
}

func (tc *ExtraIncrTestCase) player(gid int, tg *TestGroup, tags *ZeroTags) {
	tg.PlayerWait()
	defer tg.PlayerDone()
	c := NewConn(tc.proxy)
	defer c.Close()
	us := UnitSlice(make([]*Unit, tc.nkeys))
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("extra_incr_%d_%d_tag{%s}", gid, i, tags.Get(i))
		us[i] = NewUnit(key)
	}
	for _, u := range us {
		u.Del(c, false)
		ops.Incr()
	}
	for i := 0; i < tc.round; i++ {
		for _, u := range us {
			u.Incr(c)
			ops.Incr()
		}
	}
	time.Sleep(time.Second * 5)
	c1s, c1m := NewConn(tc.slave1), NewConn(tc.master1)
	c2s, c2m := NewConn(tc.slave2), NewConn(tc.master2)
	defer c1s.Close()
	defer c1m.Close()
	defer c2s.Close()
	defer c2m.Close()
	for _, u := range us {
		s := tc.groupfetch(c1s, c2s, u.key)
		m := tc.groupfetch(c1m, c2m, u.key)
		if s != m || s != u.val {
			Panic("check failed, key = %s, val = %d, master = %d, slave = %d", u.key, u.val, s, m)
		}
	}
	for _, u := range us {
		u.Del(c, true)
		ops.Incr()
	}
}

func (tc *ExtraIncrTestCase) groupfetch(c1, c2 redis.Conn, key string) int {
	r1, e1 := c1.Do("get", key)
	r2, e2 := c2.Do("get", key)
	if e1 != nil || e2 != nil {
		Panic("groupfetch key = %s, e1 = %s, e2 = %s", key, e1, e2)
	}
	if r1 == nil && r2 == nil {
		Panic("groupfetch key = %s, r1 == nil && r2 == nil", key)
	}
	if r1 != nil && r2 != nil {
		Panic("groupfetch key = %s, r1 != nil && r2 != nil, %v %v", key, r1, r2)
	}
	if r1 != nil {
		if v, err := redis.Int(r1, nil); err != nil {
			Panic("groupfetch key = %s, error = %s", key, err)
		} else {
			return v
		}
	}
	if r2 != nil {
		if v, err := redis.Int(r2, nil); err != nil {
			Panic("groupfetch key = %s, error = %s", key, err)
		} else {
			return v
		}
	}
	return -1
}
