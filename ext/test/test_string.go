package main

import (
	"flag"
	"fmt"
	"time"
)

var args struct {
	proxy  string
	maxlen int
}

func test_init() {
	flag.StringVar(&args.proxy, "proxy", "", "redis# host:port")
	flag.IntVar(&args.maxlen, "maxlen", 10000, "# bytes of test string")
}

func test_main() {
	c := NewConn(args.proxy)
	defer c.Close()
	u := NewUnit("test_{string}_string")
	u.Del(c, false)
	r := &Rand{time.Now().UnixNano()}
	n := 0
	if step := args.maxlen / 1000; step != 0 {
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
	for ; n < args.maxlen; n++ {
		u.Append(c, string(byte(uint64(r.Next())%(127-32)+32)))
		u.GetString(c)
		ops.Incr()
	}
	u.Del(c, true)
	fmt.Println("done")
}
