// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

import (
	"github.com/garyburd/redigo/redis"
)

var args struct {
	time     int
	keys     []string
	connlist []redis.Conn
}

func main() {
	var ncpu int
	var test, proxy string
	flag.IntVar(&ncpu, "ncpu", 0, "# of cpus")
	flag.StringVar(&proxy, "proxy", "", "# proxy list, separated by ','")
	flag.IntVar(&args.time, "time", 0, "wait of seconds")
	flag.StringVar(&test, "test", "", "set of tests, separated by ',', set,get,lpush,lpop are supported")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	runtime.GOMAXPROCS(ncpu)
	ncpu = runtime.GOMAXPROCS(0)

	rand.Seed(time.Now().UnixNano())

	l, m := 10, make(map[string]bool, 0)
	for len(m) < 1024*128 {
		ac, bs := 0, make([]byte, l)
		for i := 0; i < 100; i++ {
			for j := 0; j < l; j++ {
				bs[j] = byte(uint(rand.Int())%('z'-'a') + 'a')
			}
			s := string(bs)
			if _, ok := m[s]; ok {
				continue
			}
			m[s] = true
			ac++
		}
		if ac < 50 {
			l++
		}
	}
	for s, _ := range m {
		args.keys = append(args.keys, s)
	}

	for _, addr := range strings.Split(proxy, ",") {
		if len(addr) == 0 {
			continue
		}
		for i := 0; i < ncpu*8; i++ {
			conn, err := redis.Dial("tcp", addr)
			if err != nil {
				panic(fmt.Sprintf("connect to '%s', error = %s", addr, err))
			}
			args.connlist = append(args.connlist, conn)
		}
	}

	for _, name := range strings.Split(test, ",") {
		if len(name) == 0 {
			continue
		}
		switch name {
		default:
			panic(fmt.Sprintf("unsupported command = '%s'", name))
		case "set":
			benchSet()
		case "lpush":
			benchLpush()
		case "mget":
			benchMget()
		}
	}
}

type BenchCtrl struct {
	prefix  string
	sig     chan int
	players sync.WaitGroup
	running int64
	count   int64
	keys    []string
	prepare func(b *BenchCtrl, r *Rand, c redis.Conn, i int)
	cleanup func(b *BenchCtrl, r *Rand, c redis.Conn, i int)
	testing func(b *BenchCtrl, r *Rand, c redis.Conn)
}

func NewBenchCtrl(prefix string) *BenchCtrl {
	keys := make([]string, len(args.keys))
	for i := 0; i < len(args.keys); i++ {
		keys[i] = prefix + "{" + args.keys[i] + "}"
	}
	b := &BenchCtrl{}
	b.sig = make(chan int, 0)
	b.running = 1
	b.keys = keys
	b.prefix = prefix
	return b
}

func (b *BenchCtrl) Run() {
	if b.prepare != nil {
		fmt.Printf("%s: prepare\n", b.prefix)
		var wg sync.WaitGroup
		for j, conn := range args.connlist {
			wg.Add(1)
			r := NewRand()
			go func(c redis.Conn, j int) {
				for i := j; i < len(b.keys); i += len(args.connlist) {
					b.prepare(b, r, c, i)
				}
				wg.Done()
			}(conn, j)
		}
		wg.Wait()
	}

	for _, conn := range args.connlist {
		b.players.Add(1)
		go b.testing(b, NewRand(), conn)
	}
	close(b.sig)
	for i := 0; i < args.time; i++ {
		now := time.Now().UnixNano()
		count1 := atomic.LoadInt64(&b.count)
		time.Sleep(time.Second)
		dlt := time.Now().UnixNano() - now
		count2 := atomic.LoadInt64(&b.count)
		if dlt <= 0 {
			dlt = 1
		}
		fmt.Printf("%s: %d ops/s\n", b.prefix, (count2-count1)*int64(time.Second)/dlt)
	}
	atomic.StoreInt64(&b.running, 0)
	b.players.Wait()

	if b.cleanup != nil {
		fmt.Printf("%s: cleanup\n", b.prefix)
		var wg sync.WaitGroup
		for j, conn := range args.connlist {
			wg.Add(1)
			r := NewRand()
			go func(c redis.Conn, j int) {
				for i := j; i < len(b.keys); i += len(args.connlist) {
					b.cleanup(b, r, c, i)
				}
				wg.Done()
			}(conn, j)
		}
		wg.Wait()
	}
}

type Rand rand.Rand

func NewRand() *Rand {
	r := rand.New(rand.NewSource(time.Now().UnixNano() * int64(rand.Int())))
	return (*Rand)(r)
}

func (r *Rand) NextKey(keys []string) string {
	idx := uint((*rand.Rand)(r).Int()) % uint(len(keys))
	return keys[idx]
}

func (r *Rand) Int() int {
	return (*rand.Rand)(r).Int()
}

func benchSet() {
	b := NewBenchCtrl("set")
	b.testing = func(b *BenchCtrl, r *Rand, c redis.Conn) {
		<-b.sig
		for atomic.LoadInt64(&b.running) != 0 {
			key := r.NextKey(b.keys)
			_, err := c.Do("set", key, strconv.Itoa(r.Int()))
			if err != nil {
				panic(fmt.Sprintf("set '%s' error = %s", key, err))
			}
			atomic.AddInt64(&b.count, 1)
		}
		b.players.Done()
	}
	b.cleanup = func(b *BenchCtrl, r *Rand, c redis.Conn, i int) {
		key := b.keys[i]
		_, err := c.Do("del", key)
		if err != nil {
			panic(fmt.Sprintf("del '%s' error = %s", key, err))
		}
	}
	b.Run()
}

func benchLpush() {
	b := NewBenchCtrl("lpush")
	b.testing = func(b *BenchCtrl, r *Rand, c redis.Conn) {
		<-b.sig
		for atomic.LoadInt64(&b.running) != 0 {
			key := r.NextKey(b.keys)
			_, err := c.Do("lpush", key, strconv.Itoa(r.Int()))
			if err != nil {
				panic(fmt.Sprintf("lpush '%s' error = %s", key, err))
			}
			atomic.AddInt64(&b.count, 1)
		}
		b.players.Done()
	}
	b.cleanup = func(b *BenchCtrl, r *Rand, c redis.Conn, i int) {
		key := b.keys[i]
		_, err := c.Do("del", key)
		if err != nil {
			panic(fmt.Sprintf("del '%s' error = %s", key, err))
		}
	}
	b.Run()
}

func benchMget() {
	b := NewBenchCtrl("mget")
	b.testing = func(b *BenchCtrl, r *Rand, c redis.Conn) {
		<-b.sig
		keys := make([]interface{}, 16)
		for atomic.LoadInt64(&b.running) != 0 {
			for i := 0; i < len(keys); i++ {
				keys[i] = r.NextKey(b.keys)
			}
			_, err := c.Do("mget", keys...)
			if err != nil {
				panic(fmt.Sprintf("mget '%v' error = %s", keys, err))
			}
			atomic.AddInt64(&b.count, 1)
		}
		b.players.Done()
	}
	b.prepare = func(b *BenchCtrl, r *Rand, c redis.Conn, i int) {
		key := b.keys[i]
		_, err := c.Do("set", key, strconv.Itoa(r.Int()))
		if err != nil {
			panic(fmt.Sprintf("set '%s' error = %s", key, err))
		}
	}
	b.cleanup = func(b *BenchCtrl, r *Rand, c redis.Conn, i int) {
		key := b.keys[i]
		_, err := c.Do("del", key)
		if err != nil {
			panic(fmt.Sprintf("del '%s' error = %s", key, err))
		}
	}
	b.Run()
}
