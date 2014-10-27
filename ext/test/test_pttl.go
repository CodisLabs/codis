package main

import (
	"flag"
	"fmt"
	"time"
)

var args struct {
	proxy  string
	group  int
	round  int
	nkeys  int
	ntags  int
	expire int
}

func test_init() {
	flag.StringVar(&args.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&args.group, "group", 8, "# of test players")
	flag.IntVar(&args.round, "round", 100, "# push/pop all per key")
	flag.IntVar(&args.nkeys, "nkeys", 1000, "# of keys per test")
	flag.IntVar(&args.ntags, "ntags", 1000, "# tags")
	flag.IntVar(&args.expire, "expire", 1, "expire seconds")
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
		key := fmt.Sprintf("test_pttl_%d_{%d}", gid, i)
		us[i] = NewUnit(key)
	}
	ttls := make(map[string]*TTL)
	for _, u := range us {
		u.Del(c, false)
		ttls[u.key] = &TTL{}
		ops.Incr()
	}
	for i := 0; i < args.round; i++ {
		for _, u := range us {
			u.Set(c, u.key)
			Expire(c, u, ttls[u.key], args.expire)
			ops.Incr()
		}
		for {
			nothing := true
			for _, u := range us {
				if Pttl(c, u, ttls[u.key]) {
					nothing = false
				}
				ops.Incr()
			}
			if nothing {
				break
			}
		}
	}
}

type TTL struct {
	beg  int64
	end  int64
	done bool
}

func Expire(c *Conn, u *Unit, ttl *TTL, expire int) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("expire: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	if expire <= 0 {
		panic(fmt.Sprintf("invalid expire = %d", expire))
	}
	var err error
	if rsp, err = c.Do("expire", u.key, expire); err != nil {
		panic(err)
	}
	if v := c.Int(rsp); v != 1 {
		panic(fmt.Sprintf("return = %d, expect = 1", v))
	}
	ttl.beg = time.Now().UnixNano()
	ttl.end = int64(time.Second)*int64(expire) + ttl.beg
	ttl.done = false
}

func Pttl(c *Conn, u *Unit, ttl *TTL) bool {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("pttl: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("pttl", u.key); err != nil {
		panic(err)
	}
	if v := c.Int(rsp); v == -1 {
		panic("return = -1")
	} else if v == -2 {
		if !ttl.done {
			now := time.Now().UnixNano()
			dlt := ttl.end - now
			if dlt > int64(time.Second) {
				panic(fmt.Sprintf("beg = %d, end = %d, dlt = %d", ttl.beg, ttl.end, dlt))
			}
			ttl.done = true
		}
		return false
	} else {
		now := time.Now().UnixNano()
		dlt := now + int64(v)*int64(time.Millisecond) - ttl.end
		if dlt < 0 {
			dlt = -dlt
		}
		if dlt > int64(time.Second) {
			panic(fmt.Sprintf("beg = %d, end = %d, dlt = %d, now = %d, return = %d", ttl.beg, ttl.end, dlt, now, v))
		}
		return true
	}
}
