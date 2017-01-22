package main

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/assert"

	"github.com/garyburd/redigo/redis"
)

func Master() *Conn {
	c, err := redis.Dial("tcp", "127.0.0.1:2000")
	if err != nil {
		panic(err)
	}
	return &Conn{c}
}

func Slave() *Conn {
	c, err := redis.Dial("tcp", "127.0.0.1:2010")
	if err != nil {
		panic(err)
	}
	return &Conn{c}
}

type Conn struct {
	redis.Conn
}

func (c *Conn) Do(cmd string, args ...interface{}) interface{} {
	v, err := c.Conn.Do(cmd, args...)
	assert.MustNoError(err)
	return v
}

func (c *Conn) toInt(v interface{}) int {
	i, err := redis.Int(v, nil)
	assert.MustNoError(err)
	return i
}

func (c *Conn) toString(v interface{}) string {
	s, err := redis.String(v, nil)
	assert.MustNoError(err)
	return s
}

func (c *Conn) Exists(key string) bool {
	return c.toInt(c.Do("exists", key)) != 0
}

func (c *Conn) String(key string) string {
	v := c.Do("get", key)
	if v == nil {
		return ""
	}
	return c.toString(v)
}

func (c *Conn) Hash(key string) map[string]string {
	if v := c.Do("hgetall", key); v != nil {
		m, err := redis.StringMap(v, nil)
		assert.MustNoError(err)
		return m
	}
	return map[string]string{}
}

func (c *Conn) Dict(key string) map[string]bool {
	var cursor int64
	var dict = make(map[string]bool)
	for {
		values, err := redis.Values(c.Do("sscan", key, cursor), nil)
		assert.MustNoError(err)

		next, err := redis.Int64(values[0], nil)
		assert.MustNoError(err)

		eles, err := redis.Strings(values[1], nil)
		assert.MustNoError(err)
		for _, e := range eles {
			dict[e] = true
		}
		if next == 0 {
			return dict
		}
		cursor = next
	}
}

func (c *Conn) List(key string) []string {
	r := c.Do("lrange", key, 0, -1)
	if r != nil {
		s, err := redis.Strings(r, nil)
		assert.MustNoError(err)
		return s
	}
	return nil
}

func (c *Conn) ZSet(key string) map[string]float64 {
	var zset = make(map[string]float64)
	r := c.Do("zrange", key, 0, -1, "WITHSCORES")
	if r != nil {
		values, err := redis.Values(r, nil)
		assert.MustNoError(err)
		for i := 0; i < len(values); i += 2 {
			e, err := redis.String(values[i], nil)
			assert.MustNoError(err)
			f, err := redis.Float64(values[i+1], nil)
			assert.MustNoError(err)
			zset[e] = f
		}
	}
	return zset
}

func (c *Conn) SlotsRestoreAsyncProcess(args ...interface{}) {
	r := c.Do("slotsrestore-async-process", args...)
	values, err := redis.Strings(r, nil)
	assert.MustNoError(err)
	assert.Must(len(values) == 3)
	fmt.Printf("%v\n", values)
	assert.Must(strings.ToLower(values[0]) == "slotsrestore-async-ack")
	assert.Must(values[1] == "0")
}

func testHash(s1, s2 map[string]string, size int) {
	assert.Must(len(s1) == size)
	assert.Must(len(s1) == len(s2))
	for k, v := range s1 {
		assert.Must(v == s2[k])
	}
}

func testZSet(s1, s2 map[string]float64, size int) {
	assert.Must(len(s1) == size)
	assert.Must(len(s1) == len(s2))
	for k, v := range s1 {
		v2, ok := s2[k]
		assert.Must(ok && math.Abs(v-v2) < 1e-5)
	}
}

func testDict(s1, s2 map[string]bool, size int) {
	assert.Must(len(s1) == size)
	assert.Must(len(s1) == len(s2))
	for k, v := range s1 {
		assert.Must(v == s2[k])
	}
}

func testList(s1, s2 []string, size int) {
	assert.Must(len(s1) == size)
	assert.Must(len(s1) == len(s2))
	for i := range s1 {
		assert.Must(s1[i] == s2[i])
	}
}

func testString(s1, s2 string, length int) {
	assert.Must(len(s1) == length)
	assert.Must(s1 == s2)
}

func noop(n int) {
	time.Sleep(time.Millisecond * time.Duration(n))
}

func TestSlotsRestore(t *testing.T) {
	c1 := Master()
	defer c1.Close()
	c2 := Slave()
	defer c2.Close()
	c1.Do("flushall")
	c1.SlotsRestoreAsyncProcess("select", 1)
	c2.Do("select", 1)

	c1.SlotsRestoreAsyncProcess("string", "a", 0, "100")
	noop(100)

	testString(c2.String("a"), "100", 3)

	c1.SlotsRestoreAsyncProcess("del", "a")
	c1.SlotsRestoreAsyncProcess("hash", "a", 0, "a", "b")
	c1.SlotsRestoreAsyncProcess("hash", "a", 0)
	noop(100)
	testHash(c1.Hash("a"), c2.Hash("a"), 1)

	c1.SlotsRestoreAsyncProcess("del", "a")
	c1.SlotsRestoreAsyncProcess("dict", "a", 0, "a", "b", "c", "a")
	c1.SlotsRestoreAsyncProcess("dict", "a", 0, "a", "b", "c", "a")
	c1.SlotsRestoreAsyncProcess("dict", "a", 0)
	noop(100)
	testDict(c1.Dict("a"), c2.Dict("a"), 3)

	c1.SlotsRestoreAsyncProcess("del", "a")
	c1.SlotsRestoreAsyncProcess("list", "a", 0, "1", "2")
	c1.SlotsRestoreAsyncProcess("list", "a", 0, "1", "2")
	c1.SlotsRestoreAsyncProcess("list", "a", 0)
	noop(100)
	testList(c1.List("a"), c2.List("a"), 4)

	c1.SlotsRestoreAsyncProcess("del", "a")
	c1.SlotsRestoreAsyncProcess("zset", "a", 0, "a", 1, "b", 2)
	c1.SlotsRestoreAsyncProcess("zset", "a", 0, "a", 1, "b", 2)
	c1.SlotsRestoreAsyncProcess("zset", "a", 0)
	noop(100)
	testZSet(c1.ZSet("a"), c2.ZSet("a"), 2)
}
