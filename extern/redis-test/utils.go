// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bytes"
	"container/list"
	"flag"
	"fmt"
	"hash/crc32"
	"log"
	"net"
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

const (
	Timeout = 30 * 1000
)

type Conn struct {
	redis.Conn
	Host string
	Port int
}

func NewConn(addr string) *Conn {
	if t, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		Panic("parse tcp addr = %s, error = '%s'", addr, err)
	} else if conn, err := redis.Dial("tcp", addr); err != nil {
		Panic("connect to '%s' error = '%s'", addr, err)
	} else {
		return &Conn{conn, t.IP.String(), t.Port}
	}
	return nil
}

func (c *Conn) Addr() string {
	return c.Host + ":" + strconv.Itoa(c.Port)
}

func (c *Conn) Int(rsp interface{}) int {
	if v, err := redis.Int(rsp, nil); err != nil {
		panic(err.Error())
	} else {
		return v
	}
}

func (c *Conn) Ints(rsp interface{}, size int) []int {
	a := c.Values(rsp, size)
	r := make([]int, len(a))
	for i := 0; i < len(a); i++ {
		r[i] = c.Int(a[i])
	}
	return r
}

func (c *Conn) String(rsp interface{}) string {
	if s, err := redis.String(rsp, nil); err != nil {
		panic(err)
	} else {
		return s
	}
}

func (c *Conn) Values(rsp interface{}, size int) []interface{} {
	if a, err := redis.Values(rsp, nil); err != nil {
		panic(err)
	} else if size > 0 && len(a) != size {
		panic(fmt.Sprintf("values.len != %d", size))
	} else {
		return a
	}
}

func (c *Conn) DelSlot(slot int) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("slotsdel: c = %s, slot = %d, error = '%s', rsp = %v", c.Addr(), slot, x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("slotsdel", slot); err != nil {
		panic(err)
	}
	vs := c.Ints(c.Values(rsp, 1)[0], 2)
	if vs[0] != slot || vs[1] != 0 {
		panic("bad response")
	}
}

func (c *Conn) SlotsInfo() ([]int, []int) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("slotsinfo: c = %s, error = %s, rsp = %v", c.Addr(), x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("slotsinfo"); err != nil {
		panic(err)
	}
	as := c.Values(rsp, 0)
	s1 := make([]int, len(as))
	s2 := make([]int, len(as))
	for i := 0; i < len(as); i++ {
		vs := c.Ints(as[i], 2)
		s1[i], s2[i] = vs[0], vs[1]
	}
	return s1, s2
}

func (c *Conn) MgrtSlot(dst *Conn, slot int) (int, int) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("slotsmgrtslot: c = %s, slot = %d, dst = %s, error = '%s', rsp = %v", c.Addr(), slot, dst.Addr(), x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("slotsmgrtslot", dst.Host, dst.Port, Timeout, slot); err != nil {
		panic(err)
	}
	vs := c.Ints(rsp, 2)
	return vs[0], vs[1]
}

func (c *Conn) MgrtTagSlot(dst *Conn, key string) int {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("slotsmgrttagslot: c = %s, key = '%s', dst = %s, error = '%s', rsp = %v", c.Addr(), key, dst.Addr(), x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("slotsmgrttagslot", dst.Host, dst.Port, Timeout, key); err != nil {
		panic(err)
	}
	return c.Int(rsp)
}

var zerotagPool struct {
	next int
	sync.Mutex
}

func NewZeroTag() string {
	zerotagPool.Lock()
	defer zerotagPool.Unlock()
	for {
		v := zerotagPool.next
		zerotagPool.next++
		s := strconv.Itoa(v)
		if 0 == HashSlot(s) {
			return s
		}
	}
}

type ZeroTags struct {
	tags []string
}

func NewZeroTags(size int) *ZeroTags {
	tags := make([]string, size)
	for i := 0; i < size; i++ {
		tags[i] = NewZeroTag()
	}
	return &ZeroTags{tags}
}

func (z *ZeroTags) Get(i int) string {
	return z.tags[uint(i)%uint(len(z.tags))]
}

func HashSlot(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s)) % 1024
}

type Unit struct {
	key string
	val interface{}
}

func NewUnit(key string) *Unit {
	u := &Unit{}
	u.key = key
	u.val = nil
	return u
}

func DupUnit(u *Unit) *Unit {
	d := &Unit{}
	d.key = u.key
	d.val = u.val
	return d
}

func (u *Unit) HashKey(c *Conn) int {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("slotshashkey: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("slotshashkey", u.key); err != nil {
		panic(err)
	}
	return c.Int(c.Values(rsp, 1)[0])
}

func (u *Unit) Set(c *Conn, val interface{}) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("set: c = %s, key = '%s', v = '%s', error = '%s', rsp = %v", c.Addr(), u.key, val, x, rsp)
		}
	}()
	switch val.(type) {
	default:
		panic("invalid type")
	case int:
	case string:
	}
	var err error
	if rsp, err = c.Do("set", u.key, val); err != nil {
		panic(err)
	}
	c.String(rsp)
	u.val = val
}

func (u *Unit) Del(c *Conn, exists bool) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("del: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("del", u.key); err != nil {
		panic(err)
	}
	v := c.Int(rsp)
	if exists && v != 1 {
		panic("not exists")
	}
	u.val = nil
}

func (u *Unit) Mgrt(c, dst *Conn, exists bool) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("slotsmgrtone: c = %s, key = '%s', dst = %s, error = '%s', rsp = %v", c.Addr(), u.key, dst.Addr(), x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("slotsmgrtone", dst.Host, dst.Port, Timeout, u.key); err != nil {
		panic(err)
	}
	v := c.Int(rsp)
	if exists && v != 1 {
		panic("not exists")
	}
}

func (u *Unit) MgrtTag(c, dst *Conn, exists bool) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("slotsmgrttagone: c = %s, key = '%s', dst = %s, error = '%s', rsp = %v", c.Addr(), u.key, dst.Addr(), x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("slotsmgrttagone", dst.Host, dst.Port, Timeout, u.key); err != nil {
		panic(err)
	}
	v := c.Int(rsp)
	if exists && v == 0 {
		panic("not exists")
	}
}

func (u *Unit) Incr(c *Conn) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("incr: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	switch u.val.(type) {
	default:
		panic("not an integer")
	case nil:
		u.val = 0
	case int:
	}
	var err error
	if rsp, err = c.Do("incr", u.key); err != nil {
		panic(err)
	}
	v := c.Int(rsp)
	if x := u.val.(int) + 1; v != x {
		panic(fmt.Sprintf("return = %d, expect = %d", v, x))
	}
	u.val = v
}

func (u *Unit) Lpush(c *Conn, s string) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("lpush: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	switch u.val.(type) {
	default:
		panic("not a string list")
	case nil:
		u.val = list.New()
	case *list.List:
	}
	l := u.val.(*list.List)
	var err error
	if rsp, err = c.Do("lpush", u.key, s); err != nil {
		panic(err)
	}
	v := c.Int(rsp)
	if x := l.Len() + 1; x != v {
		panic(fmt.Sprintf("return = %d, except = %d", s, v, x))
	}
	l.PushFront(s)
}

func (u *Unit) Lpop(c *Conn) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("lpop: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	switch u.val.(type) {
	default:
		panic("not a string list")
	case *list.List:
	}
	l := u.val.(*list.List)
	if l.Len() == 0 {
		panic("list is empty")
	}
	var err error
	if rsp, err = c.Do("lpop", u.key); err != nil {
		panic(err)
	}
	v := c.String(rsp)
	if s := l.Remove(l.Front()).(string); s != v {
		panic(fmt.Sprintf("return = %s, expect = %s", v, s))
	}
}

func (u *Unit) Append(c *Conn, a string) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("append: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	switch u.val.(type) {
	default:
		panic("not a string")
	case nil:
		u.val = ""
	case string:
	}
	var err error
	if rsp, err = c.Do("append", u.key, a); err != nil {
		panic(err)
	}
	v := c.Int(rsp)
	s := u.val.(string)
	if x := len(s) + len(a); x != v {
		panic(fmt.Sprintf("return = %d, expect = %d", v, x))
	}
	u.val = s + a
}

func (u *Unit) GetString(c *Conn) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("get: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	switch u.val.(type) {
	default:
		panic("not a string")
	case string:
	}
	var err error
	if rsp, err = c.Do("get", u.key); err != nil {
		panic(err)
	}
	v := c.String(rsp)
	s := u.val.(string)
	if v != s {
		panic(fmt.Sprintf("return = %s, expect = %s", v, s))
	}
}

func (u *Unit) Hset(c *Conn, k, v string) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("hset: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	switch u.val.(type) {
	default:
		panic("not a hset")
	case nil:
		u.val = make(map[string]string)
	case map[string]string:
	}
	var err error
	if rsp, err = c.Do("hset", u.key, k, v); err != nil {
		panic(err)
	}
	if x := c.Int(rsp); x != 0 && x != 1 {
		panic(fmt.Sprintf("return = %d, expect = 0 or 1", x))
	}
	u.val.(map[string]string)[k] = v
}

func (u *Unit) GetHset(c *Conn) {
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("hgetall: c = %s, key = '%s', error = '%s', rsp = %v", c.Addr(), u.key, x, rsp)
		}
	}()
	switch u.val.(type) {
	default:
		panic("not a hset")
	case map[string]string:
	}
	var err error
	if rsp, err = c.Do("hgetall", u.key); err != nil {
		panic(err)
	}
	m := u.val.(map[string]string)
	as := c.Values(rsp, len(m)*2)
	r := make(map[string]string)
	for i := 0; i < len(as); i += 2 {
		r[c.String(as[i])] = c.String(as[i+1])
	}
	for k, v := range m {
		if r[k] != v {
			panic(fmt.Sprintf("key = %s, return = %s, expect = %s", k, r[k], v))
		}
	}
}

type UnitSlice []*Unit

func (us UnitSlice) Del(c *Conn, exists bool) {
	keys := make([]interface{}, len(us))
	for i := 0; i < len(us); i++ {
		keys[i] = us[i].key
	}
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("del: c = %s, keys = %v, error = '%s', rsp = %v", c.Addr(), keys, x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("del", keys...); err != nil {
		panic(err)
	}
	v := c.Int(rsp)
	if exists && v != len(us) {
		panic("not exists")
	}
	for _, u := range us {
		u.val = nil
	}
}

func (us UnitSlice) Mget(c *Conn) {
	keys := make([]interface{}, len(us))
	for i := 0; i < len(us); i++ {
		keys[i] = us[i].key
	}
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("mget: c = %s, keys = %v, error = '%s', rsp = %v", c.Addr(), keys, x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("mget", keys...); err != nil {
		panic(err)
	}
	as := c.Values(rsp, len(us))
	for i := 0; i < len(us); i++ {
		u := us[i]
		switch u.val.(type) {
		default:
			panic(fmt.Sprintf("key = %s, invalid type", u.key))
		case int:
			v := c.Int(as[i])
			if x := u.val.(int); x != v {
				panic(fmt.Sprintf("key = %s, return = %d, expect = %d", u.key, v, x))
			}
		case string:
			v := c.String(as[i])
			if x := u.val.(string); x != v {
				panic(fmt.Sprintf("key = %s, return = %s, expect = %s", u.key, v, x))
			}
		}
	}
}

func (us UnitSlice) Mset(c *Conn, vals ...interface{}) {
	if len(us) != len(vals) {
		Panic("mset: len(keys) = %d, len(vals) = %d", len(us), len(vals))
	}
	args := make([]interface{}, len(us)*2)
	for i := 0; i < len(us); i++ {
		if vals[i] == nil {
			Panic("mset: with nil argument, please use del instead")
		}
		args[i*2], args[i*2+1] = us[i].key, vals[i]
	}
	var rsp interface{}
	defer func() {
		if x := recover(); x != nil {
			Panic("mset: c = %s, args = %v, error = '%s', rsp = %v", c.Addr(), args, x, rsp)
		}
	}()
	var err error
	if rsp, err = c.Do("mset", args...); err != nil {
		panic(err)
	}
	for i := 0; i < len(us); i++ {
		us[i].val = vals[i]
	}
}

func Trace() (r string, ss []string, full bool) {
	bs := make([]byte, 16*1024)
	if n := runtime.Stack(bs, false); n != len(bs) {
		bs, full = bs[:n], true
	} else {
		bs = append(bs, []byte(" ...\n")...)
	}
	ss = strings.Split(string(bs), "\n")
	for i := 0; i < len(ss); i++ {
		ss[i] = strings.TrimSpace(ss[i])
	}
	r, ss = ss[0], ss[1:]
	return
}

func Panic(format string, args ...interface{}) {
	const tab = "    "
	var b bytes.Buffer
	r, ss, _ := Trace()
	fmt.Fprintf(&b, "[panic] ")
	fmt.Fprintf(&b, format, args...)
	fmt.Fprintf(&b, "\n"+tab+"%s\n", r)
	for i := 0; i < len(ss); i++ {
		for j := i % 2; j >= 0; j-- {
			fmt.Fprintf(&b, tab)
		}
		fmt.Fprintf(&b, "%s\n", ss[i])
	}
	log.Printf("%s", b.String())
	os.Exit(1)
}

type TestGroup struct {
	sig chan int
	wg  *sync.WaitGroup
}

func (t *TestGroup) Reset() {
	t.sig = make(chan int)
	t.wg = &sync.WaitGroup{}
}

func (t *TestGroup) Start() {
	close(t.sig)
}

func (t *TestGroup) Wait() {
	t.wg.Wait()
}

func (t *TestGroup) AddPlayer() {
	t.wg.Add(1)
}

func (t *TestGroup) PlayerWait() {
	<-t.sig
}

func (t *TestGroup) PlayerDone() {
	t.wg.Done()
}

type Rand struct {
	seed int64
}

func NewRand(seed int64) *Rand {
	return &Rand{seed}
}

func (r *Rand) Next() int {
	r.seed *= 1103515245
	r.seed += 12345
	return int(r.seed)
}

type Counter int64

func (i *Counter) Incr() {
	atomic.AddInt64((*int64)(i), 1)
}

func (i *Counter) Reset() int64 {
	return atomic.SwapInt64((*int64)(i), 0)
}

var ops Counter

type TestCase interface {
	init()
	main()
}

var (
	testcase TestCase
)

func main() {
	if testcase == nil {
		Panic("please set testcase in init function")
	}
	var ncpu int
	flag.IntVar(&ncpu, "ncpu", 0, "# of cpus")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		flag.PrintDefaults()
	}

	testcase.init()

	flag.Parse()
	runtime.GOMAXPROCS(ncpu)

	go func() {
		for {
			time.Sleep(time.Second)
			log.Printf("%6d ops/s\n", ops.Reset())
		}
	}()

	testcase.main()
}
