// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"time"
)

type TestStringTestCase struct {
	proxy  string
	maxlen int
}

func init() {
	testcase = &TestStringTestCase{}
}

func (tc *TestStringTestCase) init() {
	flag.StringVar(&tc.proxy, "proxy", "", "redis# host:port")
	flag.IntVar(&tc.maxlen, "maxlen", 10000, "# bytes of test string")
}

func (tc *TestStringTestCase) main() {
	c := NewConn(tc.proxy)
	defer c.Close()
	u := NewUnit(fmt.Sprintf("test_string_tag{%s}", NewZeroTag()))
	u.Del(c, false)
	r := &Rand{time.Now().UnixNano()}
	n := 0
	if step := tc.maxlen / 1000; step != 0 {
		buf := make([]byte, step)
		for i := 0; i < 1000; i++ {
			for j := 0; j < step; j++ {
				buf[j] = byte(uint64(r.Next())%(127-32) + 32)
			}
			u.Append(c, string(buf))
			u.GetString(c)
			n += step
			ops.Incr()
		}
	}
	for ; n < tc.maxlen; n++ {
		u.Append(c, string(byte(uint64(r.Next())%(127-32)+32)))
		u.GetString(c)
		ops.Incr()
	}
	u.Del(c, true)
	fmt.Println("done")
}
