package main

import (
	"flag"
	"fmt"
	"strconv"
	"time"
)

var args struct {
	proxy string
	group int
	round int
	nkeys int
	nvals int
	ntags int
}

func test_init() {
	flag.StringVar(&args.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&args.group, "group", 8, "# of test players")
	flag.IntVar(&args.round, "round", 100, "# push/pop all per key")
	flag.IntVar(&args.nkeys, "nkeys", 1000, "# of keys per test")
	flag.IntVar(&args.nvals, "nvals", 1000, "# of push per key")
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
	c := NewConn(args.proxy)
	defer c.Close()
	us := UnitSlice(make([]*Unit, args.nkeys))
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("test_list_%d_{%d}_%d", gid, i%args.ntags, i)
		us[i] = NewUnit(key)
	}
	for _, u := range us {
		u.Del(c, false)
		ops.Incr()
	}
	for i := 0; i < args.round; i++ {
		r := &Rand{time.Now().UnixNano()}
		for j := 0; j < args.nvals; j++ {
			for _, u := range us {
				u.Lpush(c, "val_"+strconv.Itoa(r.Next()))
				ops.Incr()
			}
		}
		for j := 0; j < args.nvals; j++ {
			for _, u := range us {
				u.Lpop(c)
				ops.Incr()
			}
		}
	}
	for _, u := range us {
		u.Del(c, false)
		ops.Incr()
	}
}
