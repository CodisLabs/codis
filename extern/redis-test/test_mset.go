// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"time"
)

type TestMsetTestCase struct {
	proxy string
	group int
	round int
	nkeys int
	ntags int
}

func init() {
	testcase = &TestMsetTestCase{}
}

func (tc *TestMsetTestCase) init() {
	flag.StringVar(&tc.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&tc.group, "group", 8, "# of test players")
	flag.IntVar(&tc.round, "round", 10000, "# of rounds per test player")
	flag.IntVar(&tc.nkeys, "nkeys", 10000, "# of keys per test")
	flag.IntVar(&tc.ntags, "ntags", 1000, "# of tags")
}

func (tc *TestMsetTestCase) main() {
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

func (tc *TestMsetTestCase) player(gid int, tg *TestGroup, tags *ZeroTags) {
	tg.PlayerWait()
	defer tg.PlayerDone()
	c := NewConn(tc.proxy)
	defer c.Close()
	us := UnitSlice(make([]*Unit, tc.nkeys))
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("test_mset_%d_%d_tag{%s}", gid, i, tags.Get(i))
		us[i] = NewUnit(key)
	}
	for _, u := range us {
		u.Del(c, false)
		ops.Incr()
	}
	for k := 0; k < tc.round; k++ {
		for _, u := range us {
			u.Incr(c)
		}
		const step = 16
		for i := 0; i < len(us); i++ {
			r := &Rand{time.Now().UnixNano()}
			for j := 0; j < step; j++ {
				u := us[uint(r.Next())%uint(len(us))]
				u.Incr(c)
			}
			t := make([]*Unit, step)
			for j := 0; j < step; j++ {
				u := us[uint(r.Next())%uint(len(us))]
				t[j] = u
			}
			vals := make([]interface{}, step)
			for j, k := 0, int(r.Next()); j < step; j++ {
				vals[j] = k
			}
			UnitSlice(t).Mset(c, vals...)
			for _, u := range t {
				u.Incr(c)
			}
			ops.Incr()
		}
	}
	for _, u := range us {
		u.Del(c, false)
		ops.Incr()
	}
}
