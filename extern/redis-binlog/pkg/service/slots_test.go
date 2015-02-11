// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"bufio"
	"net"
	"os"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/binlog"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store/rocksdb"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

var (
	testbl2 *binlog.Binlog
	port    int
)

type fakeSession2 struct {
	db uint32
}

func (s *fakeSession2) DB() uint32 {
	return s.db
}

func (s *fakeSession2) SetDB(db uint32) {
	s.db = db
}

func (s *fakeSession2) Binlog() *binlog.Binlog {
	return testbl2
}

func init() {
	const path = "/tmp/testdb2-rocksdb"
	if err := os.RemoveAll(path); err != nil {
		log.PanicErrorf(err, "remove '%s' failed", path)
	} else {
		conf := rocksdb.NewDefaultConfig()
		if testdb, err := rocksdb.Open(path, conf, true, false); err != nil {
			log.PanicError(err, "open rocksdb failed")
		} else {
			testbl2 = binlog.New(testdb)
		}
	}
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.PanicError(err, "open listen port failed")
	}
	port = l.Addr().(*net.TCPAddr).Port
	go func() {
		server := redis.MustServer(&Handler{})
		for {
			c, err := l.Accept()
			if err != nil {
				log.PanicError(err, "accept socket failed")
			}
			go func() {
				defer c.Close()
				r, w := bufio.NewReader(c), bufio.NewWriter(c)
				s := &fakeSession2{}
				for {
					if req, err := redis.Decode(r); err != nil {
						return
					} else {
						if rsp, err := server.Dispatch(s, req); err != nil {
							return
						} else if rsp != nil {
							if err := redis.Encode(w, rsp); err != nil {
								return
							}
						}
					}
				}
			}()
		}
	}()
}

func xcheck2(t *testing.T, db uint32, key string, expect string) {
	x, err := testbl2.Get(db, []byte(key))
	checkerror(t, err, x != nil && string(x) == expect)
}

func TestSlotsHashKey(t *testing.T) {
	c := client(t)
	checkintarray(t, []int64{579, 1017, 879}, c, "slotshashkey", "a", "b", "c")
}

func TestSlotsMgrtOne(t *testing.T) {
	c := client(t)
	k1 := "{tag}" + random(t)
	k2 := "{tag}" + random(t)
	checkok(t, c, "mset", k1, "1", k2, "2")
	checkint(t, 1, c, "slotsmgrtone", "127.0.0.1", port, 1000, k1)
	checkint(t, 0, c, "slotsmgrtone", "127.0.0.1", port, 1000, k1)
	xcheck2(t, 0, k1, "1")

	checkint(t, 1, c, "slotsmgrtone", "127.0.0.1", port, 1000, k2)
	checkint(t, 0, c, "slotsmgrtone", "127.0.0.1", port, 1000, k2)
	xcheck2(t, 0, k2, "2")

	checkok(t, c, "set", k1, "3")

	checkint(t, 1, c, "slotsmgrtone", "127.0.0.1", port, 1000, k1)
	checkint(t, 0, c, "slotsmgrtone", "127.0.0.1", port, 1000, k1)
	xcheck2(t, 0, k1, "3")
}

func TestSlotsMgrtTagOne(t *testing.T) {
	c := client(t)
	k1 := "{tag}" + random(t)
	k2 := "{tag}" + random(t)
	k3 := "{tag}" + random(t)
	checkok(t, c, "mset", k1, "1", k2, "2")
	checkint(t, 2, c, "slotsmgrttagone", "127.0.0.1", port, 1000, k1)
	checkint(t, 0, c, "slotsmgrttagone", "127.0.0.1", port, 1000, k1)
	xcheck2(t, 0, k1, "1")

	checkint(t, 0, c, "slotsmgrtone", "127.0.0.1", port, 1000, k2)
	xcheck2(t, 0, k2, "2")

	checkok(t, c, "mset", k1, "0", k3, "100")

	checkint(t, 2, c, "slotsmgrttagone", "127.0.0.1", port, 1000, k1)
	checkint(t, 0, c, "slotsmgrtone", "127.0.0.1", port, 1000, k1)
	checkint(t, 0, c, "slotsmgrtone", "127.0.0.1", port, 1000, k3)
	xcheck2(t, 0, k1, "0")
	xcheck2(t, 0, k3, "100")
}

func TestSlotsMgrtSlot(t *testing.T) {
	c := client(t)
	k1 := "{tag}" + random(t)
	k2 := "{tag}" + random(t)
	checkok(t, c, "mset", k1, "1", k2, "2")
	checkintarray(t, []int64{1, 1}, c, "slotsmgrtslot", "127.0.0.1", port, 1000, 899)
	checkintarray(t, []int64{1, 1}, c, "slotsmgrtslot", "127.0.0.1", port, 1000, 899)
	checkintarray(t, []int64{0, 0}, c, "slotsmgrtslot", "127.0.0.1", port, 1000, 899)

	xcheck2(t, 0, k1, "1")
	xcheck2(t, 0, k2, "2")
}

func TestSlotsMgrtTagSlot(t *testing.T) {
	c := client(t)
	k1 := "{tag}" + random(t)
	k2 := "{tag}" + random(t)
	k3 := "{tag}" + random(t)
	checkok(t, c, "mset", k1, "1", k2, "2", k3, "3")
	checkintarray(t, []int64{3, 1}, c, "slotsmgrttagslot", "127.0.0.1", port, 1000, 899)
	checkintarray(t, []int64{0, 0}, c, "slotsmgrttagslot", "127.0.0.1", port, 1000, 899)

	xcheck2(t, 0, k1, "1")
	xcheck2(t, 0, k2, "2")

	checkok(t, c, "mset", k1, "0", k3, "100")
	checkintarray(t, []int64{2, 1}, c, "slotsmgrttagslot", "127.0.0.1", port, 1000, 899)
	checkintarray(t, []int64{0, 0}, c, "slotsmgrttagslot", "127.0.0.1", port, 1000, 899)
	xcheck2(t, 0, k1, "0")
	xcheck2(t, 0, k3, "100")
}
