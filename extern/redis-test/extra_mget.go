// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"time"
)

type ExtraMgetTestCase struct {
	proxy string
}

func init() {
	testcase = &ExtraMgetTestCase{}
}

func (tc *ExtraMgetTestCase) init() {
	flag.StringVar(&tc.proxy, "proxy", "", "redis host:port")
}

func (tc *ExtraMgetTestCase) main() {
	c := NewConn(tc.proxy)
	defer c.Close()
	const max = 1000 * 1000 * 100
	var tags = NewZeroTags(10000)
	for n := 10; n <= max; n *= 10 {
		for k := 100; k <= max/n; k *= 10 {
			b := make([]byte, k)
			for i := 0; i < len(b); i++ {
				b[i] = byte(i%26 + 'a')
			}
			s := string(b)
			us := UnitSlice(make([]*Unit, n))
			for i := 0; i < len(us); i++ {
				key := fmt.Sprintf("extra_del_%d_tag{%s}", i, tags.Get(i))
				us[i] = NewUnit(key)
			}
			for _, u := range us {
				u.Set(c, s)
				ops.Incr()
			}
			beg := time.Now().UnixNano()
			us.Mget(c)
			avg := float64(time.Now().UnixNano()-beg) / float64(time.Millisecond)
			fmt.Printf("len = %-8d   key = %-6d        %8dms    avg=%.2fms\n", k, n, int64(avg), avg/float64(n))
			us.Del(c, true)
		}
	}
	fmt.Println("done")
}
