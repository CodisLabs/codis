package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"time"
)

var args struct {
	proxy string
	nkeys int
}

func test_init() {
	flag.StringVar(&args.proxy, "proxy", "", "redis host:port")
	flag.IntVar(&args.nkeys, "nkeys", 10000, "# of nkeys")
}

func test_main() {
	c := NewConn(args.proxy)
	defer c.Close()
	r := &Rand{time.Now().UnixNano()}
	for i := 0; i < args.nkeys; i++ {
		u := NewUnit(fmt.Sprintf("basic_hash_%d_%d", r.Next(), r.Next()))
		h, e := uint32(u.HashKey(c)), crc32.ChecksumIEEE([]byte(u.key))%1024
		if h != e {
			Panic("checksum key = '%s': return = %d, expect = %d", u.key, h, e)
		}
		u.key = fmt.Sprintf("%d_{%s}_%d", r.Next(), u.key, r.Next())
		h = uint32(u.HashKey(c))
		if h != e {
			Panic("checksum key = '%s': return = %d, expect = %d", u.key, h, e)
		}
		ops.Incr()
	}
}
