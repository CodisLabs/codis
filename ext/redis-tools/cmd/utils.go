package cmd

import (
	"bufio"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

import (
	"github.com/garyburd/redigo/redis"
	"github.com/spinlock/redis-tools/rdb"
	"github.com/spinlock/redis-tools/utils"
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

func openFileReader(name string) (f *os.File, nsize int64) {
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

func openFileWriter(name string) *os.File {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		utils.Panic("cannot open file-writer '%s', error = %s", name, err)
	}
	return f
}

func goClockTicker(wg *sync.WaitGroup, onTick, onKick func()) chan int {
	kick := make(chan int)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer onKick()
		for {
			select {
			case <-kick:
				return
			case <-time.After(time.Second):
				onTick()
			}
		}
	}()
	return kick
}

func goRdbLoader(wg *sync.WaitGroup, size int, reader *bufio.Reader, nread *AtomicInt64) chan *rdb.Entry {
	wg.Add(1)
	pipe := make(chan *rdb.Entry, size)
	go func() {
		defer close(pipe)
		defer wg.Done()
		l := rdb.NewLoader(reader)
		if err := l.LoadHeader(); err != nil {
			utils.Panic("parse rdb header error = '%s'", err)
		}
		for {
			if entry, offset, err := l.LoadEntry(); err != nil {
				utils.Panic("parse rdb entry error = '%s'", err)
			} else {
				if entry != nil {
					nread.Set(offset)
					pipe <- entry
				} else {
					if err := l.LoadChecksum(); err != nil {
						utils.Panic("parse rdb checksum error = '%s'", err)
					}
					return
				}
			}
		}
	}()
	return pipe
}

func goBufWriter(wg *sync.WaitGroup, size int, writer *bufio.Writer, nwrite *AtomicInt64) chan string {
	wg.Add(1)
	pipe := make(chan string, size)
	go func() {
		defer wg.Done()
		for s := range pipe {
			if _, err := writer.WriteString(s); err != nil {
				utils.Panic("buf-writer write error = '%s'", err)
			}
			if err := writer.Flush(); err != nil {
				utils.Panic("buf-writer flush error = '%s'", err)
			}
			nwrite.Add(int64(len(s)))
		}
	}()
	return pipe
}

func openSyncConn(target string) (net.Conn, int64) {
	c := openNetConn(target)
	for cmd := []byte("sync\r\n"); len(cmd) != 0; {
		n, err := c.Write(cmd)
		if err != nil {
			utils.Panic("write sync command error = %s", err)
		}
		cmd = cmd[n:]
	}
	var rsp string
	for {
		b := []byte{0}
		if _, err := c.Read(b); err != nil {
			utils.Panic("read sync response = '%s', error = %s", rsp, err)
		}
		if len(rsp) == 0 && b[0] == '\n' {
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
	if err != nil {
		utils.Panic("invalid sync response = '%s', error = %s", rsp, err)
	}
	return c, int64(n)
}

func goReaderWriterPipe(wg *sync.WaitGroup, r io.Reader, w io.Writer, nread, nwrite *AtomicInt64, total int64) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for total != 0 {
			p := make([]byte, 1024)
			if total > 0 && int64(len(p)) > total {
				p = p[:total]
			}
			if n, err := r.Read(p); err != nil {
				utils.Panic("read full error = '%s'", err)
			} else {
				p = p[:n]
			}
			delta := int64(len(p))
			nread.Add(delta)
			for len(p) != 0 {
				n, err := w.Write(p)
				if err != nil {
					utils.Panic("write error = '%s'", err)
				}
				p = p[n:]
			}
			nwrite.Add(delta)
			if total > 0 {
				total -= delta
			}
		}
	}()
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
