// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
)

type TestIncr1TestCase struct {
	proxy string
	group int
	round int
	nkeys int
}

func init() {
	testcase = &TestIncr1TestCase{}
}

func (tc *TestIncr1TestCase) init() {
	flag.StringVar(&tc.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&tc.group, "group", 8, "# of test players")
	flag.IntVar(&tc.round, "round", 100, "# of incr opts per key")
	flag.IntVar(&tc.nkeys, "nkeys", 10000, "# of keys per test")
}

func (tc *TestIncr1TestCase) main() {
	tg := &TestGroup{}
	tg.Reset()
	var tags = NewZeroTags(10000)
	for g := 0; g < tc.group; g++ {
		tg.AddPlayer()
		go tc.player(g, tg, tags)
	}
	tg.Start()
	tg.Wait()
	fmt.Println("done")
}

func (tc *TestIncr1TestCase) player(gid int, tg *TestGroup, tags *ZeroTags) {
	tg.PlayerWait()
	defer tg.PlayerDone()
	c := NewConn(tc.proxy)
	defer c.Close()
	us := make([]*Unit, tc.nkeys)
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("test_incr1_%d_%d_tag{%s}", gid, i, tags.Get(i))
		us[i] = NewUnit(key)
		us[i].Del(c, false)
		ops.Incr()
	}
	for i := 0; i < tc.round; i++ {
		for _, u := range us {
			u.Incr(c)
			ops.Incr()
		}
	}
	for _, u := range us {
		u.Del(c, true)
		ops.Incr()
	}
}
