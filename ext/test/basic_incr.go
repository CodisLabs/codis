package main

import (
	"flag"
	"fmt"
)

var args struct {
	proxy string
	group int
	round int
}

func test_init() {
	flag.StringVar(&args.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&args.group, "group", 8, "# of test players")
	flag.IntVar(&args.round, "round", 10000, "# of incr opts per test player")
}

func test_main() {
	t := &Test{}
	t.Reset()
	for g := 0; g < args.group; g++ {
		t.AddPlayer()
		go test_player(g, t)
	}
	t.Start()
	t.Wait()
	fmt.Println("done")
}

func test_player(gid int, t *Test) {
	t.PlayerWait()
	defer t.PlayerDone()
	c := NewConn(args.proxy)
	defer c.Close()
	u := NewUnit(fmt.Sprintf("basic_incr_%d", gid))
	u.Del(c, false)
	for i := 0; i < args.round; i++ {
		u.Incr(c)
		ops.Incr()
	}
	u.Del(c, true)
}
