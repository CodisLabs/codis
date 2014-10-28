package cmd

import (
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

import (
	"github.com/garyburd/redigo/redis"
	"github.com/wandoulabs/codis/ext/redis-port/rdb"
	"github.com/wandoulabs/codis/ext/redis-port/utils"
)

type AtomicInt64 int64

func (a *AtomicInt64) Get() int64 {
	return atomic.LoadInt64((*int64)(a))
}

func (a *AtomicInt64) Set(v int64) {
	atomic.StoreInt64((*int64)(a), v)
}

func (a *AtomicInt64) Reset() int64 {
	return atomic.SwapInt64((*int64)(a), 0)
}

func (a *AtomicInt64) Add(v int64) int64 {
	return atomic.AddInt64((*int64)(a), v)
}

func (a *AtomicInt64) Sub(v int64) int64 {
	return a.Add(-v)
}

func openRedisConn(target string) redis.Conn {
	return redis.NewConn(openNetConn(target), 0, 0)
}

func openNetConn(target string) net.Conn {
	c, err := net.Dial("tcp", target)
	if err != nil {
		utils.Panic("cannot connect to '%s', error = '%s'", target, err)
	}
	return c
}

func openReadFile(name string) (f *os.File, nsize int64) {
	var err error
	if f, err = os.Open(name); err != nil {
		utils.Panic("cannot open file-reader '%s', error = '%s'", name, err)
	}
	if fi, err := f.Stat(); err != nil {
		utils.Panic("cannot stat file-reader '%s', error = '%s'", name, err)
	} else {
		nsize = fi.Size()
	}
	return
}

func openWriteFile(name string) *os.File {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		utils.Panic("cannot open file-writer '%s', error = %s", name, err)
	}
	return f
}

func openSyncConn(target string) (net.Conn, chan int64) {
	c := openNetConn(target)
	for cmd := []byte("sync\r\n"); len(cmd) != 0; {
		n, err := c.Write(cmd)
		if err != nil {
			utils.Panic("write sync command error = %s", err)
		}
		cmd = cmd[n:]
	}
	size := make(chan int64)
	go func() {
		var rsp string
		for {
			b := []byte{0}
			if _, err := c.Read(b); err != nil {
				utils.Panic("read sync response = '%s', error = %s", rsp, err)
			}
			if len(rsp) == 0 && b[0] == '\n' {
				size <- 0
				continue
			}
			rsp += string(b)
			if strings.HasSuffix(rsp, "\r\n") {
				break
			}
		}
		if rsp[0] != '$' {
			utils.Panic("invalid sync response, rsp = '%s'", rsp)
		}
		n, err := strconv.Atoi(rsp[1 : len(rsp)-2])
		if err != nil || n <= 0 {
			utils.Panic("invalid sync response = '%s', error = %s, n = %d", rsp, err, n)
		}
		size <- int64(n)
	}()
	return c, size
}

func restoreRdbEntry(c redis.Conn, e *rdb.Entry) {
	var ttl uint64
	if e.Expire != 0 {
		if now := uint64(time.Now().UnixNano() / int64(time.Millisecond)); now >= e.Expire {
			ttl = 1
		} else {
			ttl = e.Expire - now
		}
	}
	s, err := redis.String(c.Do("slotsrestore", e.Key, ttl, e.Val))
	if err != nil {
		utils.Panic("restore command error = '%s'", err)
	}
	if s != "OK" {
		utils.Panic("restore command response = '%s', should be 'OK'", s)
	}
}
