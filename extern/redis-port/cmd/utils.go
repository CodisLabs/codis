// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bufio"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	redigo "github.com/garyburd/redigo/redis"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/utils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

func openRedisConn(target string) redigo.Conn {
	return redigo.NewConn(openNetConn(target), 0, 0)
}

func openNetConn(target string) net.Conn {
	c, err := net.Dial("tcp", target)
	if err != nil {
		utils.ErrorPanic(err, "cannot connect to '%s'", target)
	}
	return c
}

func openReadFile(name string) (f *os.File, nsize int64) {
	var err error
	if f, err = os.Open(name); err != nil {
		utils.ErrorPanic(err, "cannot open file-reader '%s'", name)
	}
	if fi, err := f.Stat(); err != nil {
		utils.ErrorPanic(err, "cannot stat file-reader '%s'", name)
	} else {
		nsize = fi.Size()
	}
	return
}

func openWriteFile(name string) *os.File {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		utils.ErrorPanic(err, "cannot open file-writer '%s'", name)
	}
	return f
}

func openSyncConn(target string) (net.Conn, chan int64) {
	c := openNetConn(target)
	if _, err := ioutils.WriteFull(c, []byte("sync\r\n")); err != nil {
		utils.ErrorPanic(err, "write sync command failed")
	}
	size := make(chan int64)
	go func() {
		var rsp string
		for {
			b := []byte{0}
			if _, err := c.Read(b); err != nil {
				utils.ErrorPanic(err, "read sync response = '%s'", rsp)
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
			utils.ErrorPanic(err, "invalid sync response = '%s', n = %d", rsp, n)
		}
		size <- int64(n)
	}()
	return c, size
}

func selectDB(c redigo.Conn, db uint32) {
	s, err := redigo.String(c.Do("select", db))
	if err != nil {
		utils.ErrorPanic(err, "select command error")
	}
	if s != "OK" {
		utils.Panic("select command response = '%s', should be 'OK'", s)
	}
}

func restoreRdbEntry(c redigo.Conn, e *rdb.Entry) {
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
	s, err := redigo.String(c.Do("slotsrestore", e.Key, ttlms, e.ValDump))
	if err != nil {
		utils.ErrorPanic(err, "restore command error")
	}
	if s != "OK" {
		utils.Panic("restore command response = '%s', should be 'OK'", s)
	}
}

func iocopy(r io.Reader, w io.Writer, p []byte, max int) int {
	if max <= 0 || len(p) == 0 {
		utils.Panic("invalid max = %d, len(p) = %d", max, len(p))
	}
	if len(p) > max {
		p = p[:max]
	}
	if n, err := r.Read(p); err != nil {
		utils.ErrorPanic(err, "read error")
	} else {
		p = p[:n]
	}
	if _, err := ioutils.WriteFull(w, p); err != nil {
		utils.ErrorPanic(err, "write full error")
	}
	return len(p)
}

func flushWriter(w *bufio.Writer) {
	if err := w.Flush(); err != nil {
		utils.ErrorPanic(err, "flush error")
	}
}

func newRDBLoader(reader *bufio.Reader, size int) chan *rdb.Entry {
	pipe := make(chan *rdb.Entry, size)
	go func() {
		defer close(pipe)
		l := rdb.NewLoader(reader)
		if err := l.LoadHeader(); err != nil {
			utils.ErrorPanic(err, "parse rdb header error")
		}
		for {
			if entry, err := l.LoadEntry(); err != nil {
				utils.ErrorPanic(err, "parse rdb entry error")
			} else {
				if entry != nil {
					pipe <- entry
				} else {
					if err := l.LoadChecksum(); err != nil {
						utils.ErrorPanic(err, "parse rdb checksum error")
					}
					return
				}
			}
		}
	}()
	return pipe
}
