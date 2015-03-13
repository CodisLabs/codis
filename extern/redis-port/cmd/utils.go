// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	redigo "github.com/garyburd/redigo/redis"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

func openRedisConn(target string) redigo.Conn {
	return redigo.NewConn(openNetConn(target), 0, 0)
}

func openNetConn(target string) net.Conn {
	c, err := net.Dial("tcp", target)
	if err != nil {
		log.PanicErrorf(err, "cannot connect to '%s'", target)
	}
	return c
}

func openReadFile(name string) (*os.File, int64) {
	f, err := os.Open(name)
	if err != nil {
		log.PanicErrorf(err, "cannot open file-reader '%s'", name)
	}
	s, err := f.Stat()
	if err != nil {
		log.PanicErrorf(err, "cannot stat file-reader '%s'", name)
	}
	return f, s.Size()
}

func openWriteFile(name string) *os.File {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.PanicErrorf(err, "cannot open file-writer '%s'", name)
	}
	return f
}

func openSyncConn(target string, authCode string) (net.Conn, chan int64) {
	c := openNetConn(target)

	// send auth to master
	if len(authCode) > 0 {
		cmd := fmt.Sprintf("*2\r\n$4\r\nauth\r\n$%d\r\n%s\r\n", len(authCode), authCode)
		if _, err := ioutils.WriteFull(c, []byte(cmd)); err != nil {
			log.PanicError(err, "write auth command failed")
		}

		resp := make([]byte, 5)
		if _, err := io.ReadFull(c, resp); err != nil {
			log.PanicError(err, "read auth response failed")
		}

		if string(resp) != "+OK\r\n" {
			log.Panic("auth failed")
		}

	}

	if _, err := ioutils.WriteFull(c, []byte("*1\r\n$4\r\nsync\r\n")); err != nil {
		log.PanicError(err, "write sync command failed")
	}
	size := make(chan int64)
	go func() {
		var rsp string
		for {
			b := []byte{0}
			if _, err := c.Read(b); err != nil {
				log.PanicErrorf(err, "read sync response = '%s'", rsp)
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
			log.Panicf("invalid sync response, rsp = '%s'", rsp)
		}
		n, err := strconv.Atoi(rsp[1 : len(rsp)-2])
		if err != nil || n <= 0 {
			log.PanicErrorf(err, "invalid sync response = '%s', n = %d", rsp, n)
		}
		size <- int64(n)
	}()
	return c, size
}

func selectDB(c redigo.Conn, db uint32) {
	s, err := redigo.String(c.Do("select", db))
	if err != nil {
		log.PanicError(err, "select command error")
	}
	if s != "OK" {
		log.Panicf("select command response = '%s', should be 'OK'", s)
	}
}

func restoreRdbEntry(c redigo.Conn, e *rdb.BinEntry) {
	var ttlms uint64
	if e.ExpireAt != 0 {
		now := uint64(time.Now().Add(args.shift).UnixNano())
		now /= uint64(time.Millisecond)
		if now >= e.ExpireAt {
			ttlms = 1
		} else {
			ttlms = e.ExpireAt - now
		}
	}
	s, err := redigo.String(c.Do("slotsrestore", e.Key, ttlms, e.Value))
	if err != nil {
		log.PanicError(err, "restore command error")
	}
	if s != "OK" {
		log.Panicf("restore command response = '%s', should be 'OK'", s)
	}
}

func iocopy(r io.Reader, w io.Writer, p []byte, max int) int {
	if max <= 0 || len(p) == 0 {
		log.Panicf("invalid max = %d, len(p) = %d", max, len(p))
	}
	if len(p) > max {
		p = p[:max]
	}
	if n, err := r.Read(p); err != nil {
		log.PanicError(err, "read error")
	} else {
		p = p[:n]
	}
	if _, err := ioutils.WriteFull(w, p); err != nil {
		log.PanicError(err, "write full error")
	}
	return len(p)
}

func flushWriter(w *bufio.Writer) {
	if err := w.Flush(); err != nil {
		log.PanicError(err, "flush error")
	}
}

func newRDBLoader(reader *bufio.Reader, size int) chan *rdb.BinEntry {
	pipe := make(chan *rdb.BinEntry, size)
	go func() {
		defer close(pipe)
		l := rdb.NewLoader(reader)
		if err := l.Header(); err != nil {
			log.PanicError(err, "parse rdb header error")
		}
		for {
			if entry, err := l.NextBinEntry(); err != nil {
				log.PanicError(err, "parse rdb entry error")
			} else {
				if entry != nil {
					pipe <- entry
				} else {
					if err := l.Footer(); err != nil {
						log.PanicError(err, "parse rdb checksum error")
					}
					return
				}
			}
		}
	}()
	return pipe
}
