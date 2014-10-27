package main

import (
	"flag"
	"fmt"
)

var args struct {
	master1 string
	master2 string
	round   int
}

func test_init() {
	flag.StringVar(&args.master1, "master1", "", "redis#1 host:port")
	flag.StringVar(&args.master2, "master2", "", "redis#2 host:port")
	flag.IntVar(&args.round, "round", 10000, "# of opts")
}

func test_main() {
	c1 := NewConn(args.master1)
	defer c1.Close()
	c2 := NewConn(args.master2)
	defer c2.Close()
	u := NewUnit("basic_mgrt")
	u.Del(c1, false)
	u.Del(c2, false)
	for i := 0; i < args.round; i++ {
		u.Incr(c1)
		u.Mgrt(c1, c2, true)
		c1, c2 = c2, c1
		ops.Incr()
	}
	u.Del(c1, false)
	u.Del(c2, false)
	fmt.Println("done")
}
