package main

import (
	"flag"
	"fmt"
	"time"
)

var args struct {
	proxy string
	group int
	round int
	nkeys int
}

func test_init() {
	flag.StringVar(&args.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&args.group, "group", 8, "# of test players")
	flag.IntVar(&args.round, "round", 100, "# of incr opts per key")
	flag.IntVar(&args.nkeys, "nkeys", 10000, "# of keys per test")
}

func test_main() {
	go func() {
		c := NewConn(args.proxy)
		for {
			time.Sleep(time.Second * 5)
			c.Check()
		}
	}()
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
	us := make([]*Unit, args.nkeys)
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("test_incr1_%d_{%d}", gid, i)
		us[i] = NewUnit(key)
		us[i].Del(c, false)
		ops.Incr()
	}
	for i := 0; i < args.round; i++ {
		for _, u := range us {
			u.Incr(c)
			ops.Incr()
		}
	}
	for _, u := range us {
		u.Del(c, true)
		ops.Incr()
	}
}
