package main

import (
	"flag"
	"fmt"
	"time"
)

var args struct {
	proxy string
	group int
	nkeys int
}

func test_init() {
	flag.StringVar(&args.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&args.group, "group", 8, "# of test players")
	flag.IntVar(&args.nkeys, "nkeys", 1000, "# of keys per test")
}

func test_main() {
	fmt.Println(`
!! PLEASE MAKE SURE !!
- compile : make MALLOC=libc -j4
- run     : valgrind --leak-check=full
`)
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
	r := &Rand{time.Now().UnixNano()}
	us := UnitSlice(make([]*Unit, args.nkeys))
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("extra_memleak_%d_%d_%d", gid, i, r.Next())
		us[i] = NewUnit(key)
	}
	us.Del(c, false)
	for _, u := range us {
		u.Lpush(c, fmt.Sprintf("val_%d", r.Next()))
		ops.Incr()
	}
	us.Del(c, false)
}
