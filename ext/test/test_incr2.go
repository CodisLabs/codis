package main

import (
	"flag"
	"fmt"
	"time"
)

var args struct {
	proxy1 string
	proxy2 string
	group  int
	round  int
	nkeys  int
	ntags  int
}

func test_init() {
	flag.StringVar(&args.proxy1, "proxy1", "", "redis#1 host:port")
	flag.StringVar(&args.proxy2, "proxy2", "", "redis#2 host:port")
	flag.IntVar(&args.group, "group", 8, "# of test players")
	flag.IntVar(&args.round, "round", 10, "# of incr opts per key")
	flag.IntVar(&args.nkeys, "nkeys", 10000, "# of keys per test")
	flag.IntVar(&args.ntags, "ntags", 1000, "# of tags")
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
	c1 := NewConn(args.proxy1)
	defer c1.Close()
	c2 := NewConn(args.proxy2)
	defer c2.Close()
	us := UnitSlice(make([]*Unit, args.nkeys))
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("test_incr2_%d_{%d}_%d", gid, i%args.ntags, i)
		us[i] = NewUnit(key)
	}
	for _, u := range us {
		u.Del(c1, false)
		ops.Incr()
	}
	for i := 0; i < args.round; i++ {
		r := &Rand{time.Now().UnixNano()}
		for _, u := range us {
			u.Incr(c1)
			if r.Next()%2 != 0 {
				c1, c2 = c2, c1
			}
			ops.Incr()
		}
	}
	for _, u := range us {
		u.Del(c2, true)
		ops.Incr()
	}
}
