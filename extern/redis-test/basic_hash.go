// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"time"
)

type BasicHashTestCase struct {
	proxy string
	nkeys int
}

func init() {
	testcase = &BasicHashTestCase{}
}

func (tc *BasicHashTestCase) init() {
	flag.StringVar(&tc.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&tc.nkeys, "nkeys", 10000, "# of nkeys")
}

func (tc *BasicHashTestCase) main() {
	c := NewConn(tc.proxy)
	defer c.Close()
	r := &Rand{time.Now().UnixNano()}
	for i := 0; i < tc.nkeys; i++ {
		u := NewUnit(fmt.Sprintf("basic_hash_%d_%d", r.Next(), r.Next()))
		h, e := uint32(u.HashKey(c)), HashSlot(u.key)
		if h != e {
			Panic("checksum key = '%s': return = %d, expect = %d", u.key, h, e)
		}
		u.key = fmt.Sprintf("%d_{%s}_%d", r.Next(), u.key, r.Next())
		h = uint32(u.HashKey(c))
		if h != e {
			Panic("checksum key = '%s': return = %d, expect = %d", u.key, h, e)
		}
		ops.Incr()
	}
}
