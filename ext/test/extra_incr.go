package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
)

var args struct {
	proxy   string
	master1 string
	slave1  string
	master2 string
	slave2  string
	group   int
	round   int
	nkeys   int
	ntags   int
}

func test_init() {
	flag.StringVar(&args.proxy, "proxy", "", "redis host:port")
	flag.StringVar(&args.master1, "master1", "", "redis host:port")
	flag.StringVar(&args.slave1, "slave1", "", "redis host:port")
	flag.StringVar(&args.master2, "master2", "", "redis host:port")
	flag.StringVar(&args.slave2, "slave2", "", "redis host:port")
	flag.IntVar(&args.group, "group", 8, "# of test players")
	flag.IntVar(&args.round, "round", 100, "# of incr opts per key")
	flag.IntVar(&args.nkeys, "nkeys", 10000, "# of keys per test")
	flag.IntVar(&args.ntags, "ntags", 1000, "# tags")
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
	us := UnitSlice(make([]*Unit, args.nkeys))
	for i := 0; i < len(us); i++ {
		key := fmt.Sprintf("extra_incr_%d_{%d}_%d", gid, i%args.ntags, i)
		us[i] = NewUnit(key)
	}
	for _, u := range us {
		u.Del(c, false)
		ops.Incr()
	}
	for i := 0; i < args.round; i++ {
		for _, u := range us {
			u.Incr(c)
			ops.Incr()
		}
	}
	time.Sleep(time.Second * 5)
	c1s, c1m := NewConn(args.slave1), NewConn(args.master1)
	c2s, c2m := NewConn(args.slave2), NewConn(args.master2)
	defer c1s.Close()
	defer c1m.Close()
	defer c2s.Close()
	defer c2m.Close()
	for _, u := range us {
		s := groupfetch(c1s, c2s, u.key)
		m := groupfetch(c1m, c2m, u.key)
		if s != m || s != u.val {
			Panic("check failed, key = %s, val = %d, master = %d, slave = %d", u.key, u.val, s, m)
		}
	}
	for _, u := range us {
		u.Del(c, true)
		ops.Incr()
	}
	c.Check()
}

func groupfetch(c1, c2 redis.Conn, key string) int {
	r1, e1 := c1.Do("get", key)
	r2, e2 := c2.Do("get", key)
	if e1 != nil || e2 != nil {
		Panic("groupfetch key = %s, e1 = %s, e2 = %s", key, e1, e2)
	}
	if r1 == nil && r2 == nil {
		Panic("groupfetch key = %s, r1 == nil && r2 == nil", key)
	}
	if r1 != nil && r2 != nil {
		Panic("groupfetch key = %s, r1 != nil && r2 != nil, %v %v", key, r1, r2)
	}
	if r1 != nil {
		if v, err := redis.Int(r1, nil); err != nil {
			Panic("groupfetch key = %s, error = %s", key, err)
		} else {
			return v
		}
	}
	if r2 != nil {
		if v, err := redis.Int(r2, nil); err != nil {
			Panic("groupfetch key = %s, error = %s", key, err)
		} else {
			return v
		}
	}
	return -1
}
