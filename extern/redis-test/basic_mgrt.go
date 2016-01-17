// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
)

type BasicMgrtTestCase struct {
	master1 string
	master2 string
	round   int
}

func init() {
	testcase = &BasicMgrtTestCase{}
}

func (tc *BasicMgrtTestCase) init() {
	flag.StringVar(&tc.master1, "master1", "", "redis#1 host:port")
	flag.StringVar(&tc.master2, "master2", "", "redis#2 host:port")
	flag.IntVar(&tc.round, "round", 10000, "# of opts")
}

func (tc *BasicMgrtTestCase) main() {
	c1 := NewConn(tc.master1)
	defer c1.Close()
	c2 := NewConn(tc.master2)
	defer c2.Close()
	u := NewUnit(fmt.Sprintf("basic_mgrt_tag{%s}", NewZeroTag()))
	u.Del(c1, false)
	u.Del(c2, false)
	for i := 0; i < tc.round; i++ {
		u.Incr(c1)
		u.Mgrt(c1, c2, true)
		c1, c2 = c2, c1
		ops.Incr()
	}
	u.Del(c1, false)
	u.Del(c2, false)
	fmt.Println("done")
}
