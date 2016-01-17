// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"strconv"
	"time"
)

type TestListTestCase struct {
	proxy string
	group int
	round int
	nkeys int
	nvals int
	ntags int
}

func init() {
	testcase = &TestListTestCase{}
}

func (tc *TestListTestCase) init() {
	flag.StringVar(&tc.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&tc.group, "group", 8, "# of test players")
	flag.IntVar(&tc.round, "round", 100, "# push/pop all per key")
	flag.IntVar(&tc.nkeys, "nkeys", 1000, "# of keys per test")
	flag.IntVar(&tc.nvals, "nvals", 1000, "# of push per key")
	flag.IntVar(&tc.ntags, "ntags", 1000, "# of tags")
}

func (tc *TestListTestCase) main() {
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

func (tc *TestListTestCase) player(gid int, tg *TestGroup, tags *ZeroTags) {
	tg.PlayerWait()
	defer tg.PlayerDone()
	c := NewConn(tc.proxy)
	defer c.Close()
	us := UnitSlice(make([]*Unit, tc.nkeys))
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("test_list_%d_%d_tag{%s}", gid, i, tags.Get(i))
		us[i] = NewUnit(key)
	}
	for _, u := range us {
		u.Del(c, false)
		ops.Incr()
	}
	for i := 0; i < tc.round; i++ {
		r := &Rand{time.Now().UnixNano()}
		for j := 0; j < tc.nvals; j++ {
			for _, u := range us {
				u.Lpush(c, "val_"+strconv.Itoa(r.Next()))
				ops.Incr()
			}
		}
		for j := 0; j < tc.nvals; j++ {
			for _, u := range us {
				u.Lpop(c)
				ops.Incr()
			}
		}
	}
	for _, u := range us {
		u.Del(c, false)
		ops.Incr()
	}
}
