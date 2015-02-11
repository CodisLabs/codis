// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

func TestSlotNum(t *testing.T) {
	tests := [][]string{
		[]string{"", ""},
		[]string{"{", "{"},
		[]string{"{test", "{test"},
		[]string{"{test{0}", "test{0"},
		[]string{"test{a}", "a"},
		[]string{"{b}test", "b"},
		[]string{"}test{c}", "c"},
		[]string{"}test", "}test"},
		[]string{"}test1{test2{d}}{e}", "test2{d"},
	}
	for _, p := range tests {
		key, tag := []byte(p[0]), []byte(p[1])
		checkerror(t, nil, bytes.Equal(HashTag(key), tag))
	}
	const n = MaxSlotNum * 32
	for i := 0; i < n; i++ {
		key := []byte(fmt.Sprintf("key_%d_%d", rand.Int(), rand.Int()))
		checkerror(t, nil, bytes.Equal(HashTag(key), key))
	}
	for i := 0; i < n; i++ {
		v := rand.Int()
		tag := []byte(fmt.Sprintf("%d", v))
		key := []byte(fmt.Sprintf("key_{%d}_%d", v, rand.Int()))
		checkerror(t, nil, bytes.Equal(HashTag(key), tag))
	}
}

func xslotsrestore(t *testing.T, db uint32, args ...interface{}) {
	x := []interface{}{}
	for i, a := range args {
		switch i % 3 {
		case 0, 1:
			x = append(x, a)
		case 2:
			dump, err := rdb.EncodeDump(rdb.String([]byte(a.(string))))
			checkerror(t, err, true)
			x = append(x, dump)
		}
	}
	err := testbl.SlotsRestore(db, x...)
	checkerror(t, err, true)
}

func slotsinfo(t *testing.T, db uint32, sum int64) {
	m, err := testbl.SlotsInfo(db)
	checkerror(t, err, m != nil)
	a := int64(0)
	for _, v := range m {
		a += v
	}
	checkerror(t, nil, a == sum)
}

func TestSlotsRestore(t *testing.T) {
	xslotsrestore(t, 0, "key", 1000, "hello")
	xget(t, 0, "key", "hello")
	kpttl(t, 0, "key", 1000)

	xslotsrestore(t, 0, "key", 8000, "world")
	xget(t, 0, "key", "world")
	kpttl(t, 0, "key", 8000)

	xslotsrestore(t, 0, "key", 2000, "abc0", "key", 6000, "abc2")
	xget(t, 0, "key", "abc2")
	kpttl(t, 0, "key", 6000)

	xslotsrestore(t, 0, "key", 1000, "abc3", "key", 1000, "abc1")
	xget(t, 0, "key", "abc1")
	kpttl(t, 0, "key", 1000)

	slotsinfo(t, 0, 1)
	xdel(t, 0, "key", 1)
	slotsinfo(t, 0, 0)
	checkempty(t)
}

func checkconn(t *testing.T) (*net.TCPAddr, net.Conn) {
	l, err := net.Listen("tcp4", ":0")
	checkerror(t, err, true)
	defer l.Close()

	addr := l.Addr().(*net.TCPAddr)

	x := make(chan interface{}, 1)

	go func() {
		c, err := l.Accept()
		if err != nil {
			x <- err
		} else {
			x <- c
		}
	}()

	conn, err := getSockConn(addr.String(), time.Second)
	checkerror(t, err, true)
	putSockConn(addr.String(), conn)

	o := <-x
	if err, ok := o.(error); ok {
		checkerror(t, err, false)
	}
	return addr, o.(net.Conn)
}

func checkslotsmgrt(t *testing.T, r *bufio.Reader, w *bufio.Writer, c chan error, expect ...interface{}) {
	if len(expect) != 0 {
		req1, err := redis.Decode(r)
		checkerror(t, err, true)
		cmd1, args1, err := redis.ParseArgs(req1)
		checkerror(t, err, cmd1 == "select" && len(args1) == 1)

		checkerror(t, redis.Encode(w, redis.NewString("OK")), true)
		checkerror(t, w.Flush(), true)

		req2, err := redis.Decode(r)
		cmd2, args2, err := redis.ParseArgs(req2)
		checkerror(t, err, cmd2 == "slotsrestore" && len(args2) == len(expect))

		m := make(map[string]*struct {
			key, value string
			ttlms      uint64
		})
		for i := 0; i < len(expect)/3; i++ {
			v := &struct {
				key, value string
				ttlms      uint64
			}{key: expect[i*3].(string), value: expect[i*3+2].(string)}
			v.ttlms, err = ParseUint(expect[i*3+1])
			checkerror(t, err, true)
			m[v.key] = v
		}

		for i := 0; i < len(expect)/3; i++ {
			key := args2[i*3]
			ttlms := args2[i*3+1]
			value := args2[i*3+2]

			v := m[string(key)]
			checkerror(t, nil, v != nil)
			checkerror(t, nil, string(key) == v.key)
			b, err := rdb.DecodeDump(value)
			checkerror(t, err, string(b.(rdb.String)) == v.value)
			x, err := strconv.Atoi(string(ttlms))
			checkerror(t, err, true)
			if v.ttlms == 0 {
				checkerror(t, nil, x == 0)
			} else {
				checkerror(t, nil, x != 0)
				checkerror(t, nil, math.Abs(float64(x)-float64(v.ttlms)) < 500)
			}
		}

		checkerror(t, redis.Encode(w, redis.NewString("OK")), true)
		checkerror(t, w.Flush(), true)
	}

	select {
	case err := <-c:
		checkerror(t, err, true)
	case <-time.After(time.Second):
		checkerror(t, nil, false)
	}
}

func slotsmgrtslot(addr *net.TCPAddr, db uint32, tag string, expect int64) chan error {
	c := make(chan error, 1)
	go func() {
		host, port := addr.IP.String(), addr.Port
		fmt.Println(host, port)
		n, err := testbl.SlotsMgrtSlot(db, host, port, 1000, HashTagToSlot([]byte(tag)))
		if err != nil {
			c <- err
		} else if n != expect {
			c <- errors.Errorf("n = %d, expect = %d", n, expect)
		} else {
			c <- nil
		}
	}()
	return c
}

func slotsmgrttagslot(addr *net.TCPAddr, db uint32, tag string, expect int64) chan error {
	c := make(chan error, 1)
	go func() {
		host, port := addr.IP.String(), addr.Port
		n, err := testbl.SlotsMgrtTagSlot(db, host, port, 1000, HashTagToSlot([]byte(tag)))
		if err != nil {
			c <- err
		} else if n != expect {
			c <- errors.Errorf("n = %d, expect = %d", n, expect)
		} else {
			c <- nil
		}
	}()
	return c
}

func slotsmgrtone(addr *net.TCPAddr, db uint32, key string, expect int64) chan error {
	c := make(chan error, 1)
	go func() {
		host, port := addr.IP.String(), addr.Port
		n, err := testbl.SlotsMgrtOne(db, host, port, 1000, []byte(key))
		if err != nil {
			c <- err
		} else if n != expect {
			c <- errors.Errorf("n = %d, expect = %d", n, expect)
		} else {
			c <- nil
		}
	}()
	return c
}

func slotsmgrttagone(addr *net.TCPAddr, db uint32, key string, expect int64) chan error {
	c := make(chan error, 1)
	go func() {
		host, port := addr.IP.String(), addr.Port
		n, err := testbl.SlotsMgrtTagOne(db, host, port, 1000, []byte(key))
		if err != nil {
			c <- err
		} else if n != expect {
			c <- errors.Errorf("n = %d, expect = %d", n, expect)
		} else {
			c <- nil
		}
	}()
	return c
}

func TestSlotsMgrtSlot(t *testing.T) {
	xslotsrestore(t, 0, "key", 1000, "hello", "key", 8000, "world")

	addr, c := checkconn(t)
	defer c.Close()

	r, w := bufio.NewReader(c), bufio.NewWriter(c)

	slotsinfo(t, 0, 1)
	checkslotsmgrt(t, r, w, slotsmgrtslot(addr, 0, "key", 1), "key", 8000, "world")
	slotsinfo(t, 0, 0)
	checkslotsmgrt(t, r, w, slotsmgrtslot(addr, 0, "key", 0))
	slotsinfo(t, 0, 0)

	slotsinfo(t, 1, 0)
	checkslotsmgrt(t, r, w, slotsmgrtslot(addr, 1, "key", 0))

	xslotsrestore(t, 1, "key", 0, "world2")
	slotsinfo(t, 1, 1)

	checkslotsmgrt(t, r, w, slotsmgrtslot(addr, 1, "key", 1), "key", 0, "world2")
	slotsinfo(t, 1, 0)

	checkempty(t)
}

func TestSlotsMgrtTagSlot(t *testing.T) {
	args := []interface{}{}
	for i := 0; i < 32; i++ {
		key := "{}_" + strconv.Itoa(i)
		value := "test_" + strconv.Itoa(i)
		xset(t, 0, key, value)
		kpexpire(t, 0, key, 1000, 1)
		args = append(args, key, 1000, value)
	}

	addr, c := checkconn(t)
	defer c.Close()

	r, w := bufio.NewReader(c), bufio.NewWriter(c)

	slotsinfo(t, 0, 1)
	checkslotsmgrt(t, r, w, slotsmgrttagslot(addr, 0, "tag", 0))
	checkslotsmgrt(t, r, w, slotsmgrttagslot(addr, 0, "", 32), args...)
	checkslotsmgrt(t, r, w, slotsmgrttagslot(addr, 0, "", 0))
	slotsinfo(t, 0, 0)

	checkempty(t)
}

func TestSlotsMgrtOne(t *testing.T) {
	xset(t, 0, "key{tag}", "hello")
	xset(t, 1, "key{tag}1", "hello")
	xset(t, 1, "key{tag}2", "world")

	addr, c := checkconn(t)
	defer c.Close()

	r, w := bufio.NewReader(c), bufio.NewWriter(c)

	slotsinfo(t, 0, 1)
	checkslotsmgrt(t, r, w, slotsmgrtone(addr, 0, "key{tag}", 1), "key{tag}", 0, "hello")
	slotsinfo(t, 0, 0)

	slotsinfo(t, 1, 1)
	checkslotsmgrt(t, r, w, slotsmgrtone(addr, 1, "key{tag}1", 1), "key{tag}1", 0, "hello")
	slotsinfo(t, 1, 1)
	checkslotsmgrt(t, r, w, slotsmgrtone(addr, 1, "key{tag}2", 1), "key{tag}2", 0, "world")
	slotsinfo(t, 1, 0)

	checkempty(t)
}

func TestSlotsMgrtTagOne(t *testing.T) {
	xset(t, 0, "tag", "xxxx")
	xset(t, 0, "key{tag}", "hello")
	xset(t, 1, "key{tag}1", "hello")
	xset(t, 1, "key{tag}2", "world")

	addr, c := checkconn(t)
	defer c.Close()

	r, w := bufio.NewReader(c), bufio.NewWriter(c)

	slotsinfo(t, 0, 1)
	checkslotsmgrt(t, r, w, slotsmgrttagone(addr, 0, "tag", 1), "tag", 0, "xxxx")
	slotsinfo(t, 0, 1)
	checkslotsmgrt(t, r, w, slotsmgrttagone(addr, 0, "key{tag}", 1), "key{tag}", 0, "hello")
	slotsinfo(t, 0, 0)

	slotsinfo(t, 1, 1)
	checkslotsmgrt(t, r, w, slotsmgrttagone(addr, 1, "key{tag}1", 2), "key{tag}1", 0, "hello", "key{tag}2", 0, "world")
	slotsinfo(t, 1, 0)
	checkslotsmgrt(t, r, w, slotsmgrttagone(addr, 1, "key{tag}2", 0))

	xset(t, 2, "key{tag3}", "test")
	kpexpire(t, 2, "key{tag3}", 10, 1)
	sleepms(20)
	checkslotsmgrt(t, r, w, slotsmgrttagone(addr, 2, "key{tag}3", 0))
	xdel(t, 2, "key{tag3}", 0)

	checkempty(t)
}
