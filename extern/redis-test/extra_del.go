// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"time"
)

type ExtraDelTestCase struct {
	proxy string
}

func init() {
	testcase = &ExtraDelTestCase{}
}

func (tc *ExtraDelTestCase) init() {
	flag.StringVar(&tc.proxy, "proxy", "", "redis host:port")
}

func (tc *ExtraDelTestCase) main() {
	c := NewConn(tc.proxy)
	defer c.Close()
	var tags = NewZeroTags(10000)
	for n := 10; n <= 100*10000; n *= 10 {
		us := UnitSlice(make([]*Unit, n))
		for i := 0; i < len(us); i++ {
			key := fmt.Sprintf("extra_del_%d_tag{%s}", i, tags.Get(i))
			us[i] = NewUnit(key)
		}
		for _, u := range us {
			u.Set(c, u.key)
			ops.Incr()
		}
		beg := time.Now().UnixNano()
		us.Del(c, true)
		avg := float64(time.Now().UnixNano()-beg) / float64(time.Millisecond)
		fmt.Printf("n = %-10d %8dms    avg=%.2fms\n", n, int64(avg), avg/float64(n))
	}
	fmt.Println("done")
}
