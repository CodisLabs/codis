// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
)

type BasicIncrTestCase struct {
	proxy string
	group int
	round int
}

func init() {
	testcase = &BasicIncrTestCase{}
}

func (tc *BasicIncrTestCase) init() {
	flag.StringVar(&tc.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&tc.group, "group", 8, "# of test players")
	flag.IntVar(&tc.round, "round", 10000, "# of incr opts per test player")
}

func (tc *BasicIncrTestCase) main() {
	tg := &TestGroup{}
	tg.Reset()
	for g := 0; g < tc.group; g++ {
		tg.AddPlayer()
		go tc.player(g, tg)
	}
	tg.Start()
	tg.Wait()
	fmt.Println("done")
}

func (tc *BasicIncrTestCase) player(gid int, tg *TestGroup) {
	tg.PlayerWait()
	defer tg.PlayerDone()
	c := NewConn(tc.proxy)
	defer c.Close()
	u := NewUnit(fmt.Sprintf("basic_incr_%d_tag{%s}", gid, NewZeroTag()))
	u.Del(c, false)
	for i := 0; i < tc.round; i++ {
		u.Incr(c)
		ops.Incr()
	}
	u.Del(c, true)
}
