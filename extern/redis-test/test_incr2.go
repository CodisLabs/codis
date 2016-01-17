// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"time"
)

type TestIncr2TestCase struct {
	proxy1 string
	proxy2 string
	group  int
	round  int
	nkeys  int
	ntags  int
}

func init() {
	testcase = &TestIncr2TestCase{}
}

func (tc *TestIncr2TestCase) init() {
	flag.StringVar(&tc.proxy1, "proxy1", "", "redis#1 host:port")
	flag.StringVar(&tc.proxy2, "proxy2", "", "redis#2 host:port")
	flag.IntVar(&tc.group, "group", 8, "# of test players")
	flag.IntVar(&tc.round, "round", 10, "# of incr opts per key")
	flag.IntVar(&tc.nkeys, "nkeys", 10000, "# of keys per test")
	flag.IntVar(&tc.ntags, "ntags", 1000, "# of tags")
}

func (tc *TestIncr2TestCase) main() {
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

func (tc *TestIncr2TestCase) player(gid int, tg *TestGroup, tags *ZeroTags) {
	tg.PlayerWait()
	defer tg.PlayerDone()
	c1 := NewConn(tc.proxy1)
	defer c1.Close()
	c2 := NewConn(tc.proxy2)
	defer c2.Close()
	us := UnitSlice(make([]*Unit, tc.nkeys))
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("test_incr2_%d_%d_tag{%s}", gid, i, tags.Get(i))
		us[i] = NewUnit(key)
	}
	for _, u := range us {
		u.Del(c1, false)
		ops.Incr()
	}
	for i := 0; i < tc.round; i++ {
		r := &Rand{time.Now().UnixNano()}
		for _, u := range us {
			u.Incr(c1)
			if r.Next()%2 != 0 {
				c1, c2 = c2, c1
			}
			ops.Incr()
		}
	}
	for _, u := range us {
		u.Del(c2, true)
		ops.Incr()
	}
}
