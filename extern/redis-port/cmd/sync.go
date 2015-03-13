// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bufio"
	"io"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/pipe"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

type cmdSync struct {
	nread, nrecv, nobjs atomic2.Int64
}

func (cmd *cmdSync) Main() {
	from, target := args.from, args.target
	if len(from) == 0 {
		log.Panic("invalid argument: from")
	}
	if len(target) == 0 {
		log.Panic("invalid argument: target")
	}

	log.Infof("sync from '%s' to '%s'\n", from, target)

	var sockfile *os.File
	if len(args.sockfile) != 0 {
		f, err := pipe.OpenFile(args.sockfile, false)
		if err != nil {
			log.PanicError(err, "open sockbuff file failed")
		}
		sockfile = f
	}

	master, nsize := cmd.SendCmd(from)
	defer master.Close()

	log.Infof("rdb file = %d\n", nsize)

	var input io.Reader
	if sockfile != nil {
		r, w := pipe.PipeFile(ReaderBufferSize, int(args.filesize), sockfile)
		defer r.Close()
		go func() {
			defer w.Close()
			p := make([]byte, ReaderBufferSize)
			for {
				iocopy(master, w, p, len(p))
			}
		}()
		input = r
	} else {
		input = master
	}

	reader := bufio.NewReaderSize(ioutils.NewCountReader(input, &cmd.nread), ReaderBufferSize)

	cmd.SyncRDBFile(reader, target, nsize)
	cmd.SyncCommand(reader, target)
}

func (cmd *cmdSync) SendCmd(master string) (net.Conn, int64) {
	c, wait := openSyncConn(master, args.auth)
	for {
		select {
		case nsize := <-wait:
			if nsize == 0 {
				log.Info("+")
			} else {
				return c, nsize
			}
		case <-time.After(time.Second):
			log.Info("-")
		}
	}
}

func (cmd *cmdSync) SyncRDBFile(reader *bufio.Reader, slave string, nsize int64) {
	pipe := newRDBLoader(reader, args.parallel*32)
	wait := make(chan struct{})
	go func() {
		defer close(wait)
		group := make(chan int, args.parallel)
		for i := 0; i < cap(group); i++ {
			go func() {
				defer func() {
					group <- 0
				}()
				c := openRedisConn(slave)
				defer c.Close()
				var lastdb uint32 = 0
				for e := range pipe {
					if !acceptDB(e.DB) {
						continue
					}
					if e.DB != lastdb {
						lastdb = e.DB
						selectDB(c, lastdb)
					}
					restoreRdbEntry(c, e)
					cmd.nobjs.Incr()
				}
			}()
		}
		for i := 0; i < cap(group); i++ {
			<-group
		}
	}()

	for done := false; !done; {
		select {
		case <-wait:
			done = true
		case <-time.After(time.Second):
		}
		n, o := cmd.nread.Get(), cmd.nobjs.Get()
		p := 100 * n / nsize
		log.Infof("total=%d - %12d [%3d%%]  objs=%d\n", nsize, n, p, o)
	}
	log.Info("sync rdb done")
}

func (cmd *cmdSync) SyncCommand(reader *bufio.Reader, slave string) {
	var forward, nbypass atomic2.Int64
	c := openNetConn(slave)
	defer c.Close()

	writer := bufio.NewWriterSize(c, WriterBufferSize)
	defer flushWriter(writer)

	go func() {
		p := make([]byte, ReaderBufferSize)
		for {
			cnt := iocopy(c, ioutil.Discard, p, len(p))
			cmd.nrecv.Add(int64(cnt))
		}
	}()

	var mu sync.Mutex
	go func() {
		for {
			time.Sleep(time.Second)
			mu.Lock()
			flushWriter(writer)
			mu.Unlock()
		}
	}()

	go func() {
		var bypass bool = false
		for {
			resp := redis.MustDecode(reader)
			if cmd, args, err := redis.ParseArgs(resp); err != nil {
				log.PanicError(err, "parse command arguments failed")
			} else if cmd != "ping" {
				if cmd == "select" {
					if len(args) != 1 {
						log.Panicf("select command len(args) = %d", len(args))
					}
					s := string(args[0])
					n, err := parseInt(s, MinDB, MaxDB)
					if err != nil {
						log.PanicErrorf(err, "parse db = %s failed", s)
					}
					bypass = !acceptDB(uint32(n))
				}
				if bypass {
					nbypass.Incr()
					continue
				}
			}
			mu.Lock()
			redis.MustEncode(writer, resp)
			mu.Unlock()
			forward.Incr()
		}
	}()

	for {
		forward.Snapshot()
		nbypass.Snapshot()
		cmd.nread.Snapshot()
		cmd.nrecv.Snapshot()
		time.Sleep(time.Second)
		log.Infof("sync: +forward=%-6d  +bypass=%-6d  +read=%-9d  +recv=%-9d\n", forward.Delta(), nbypass.Delta(), cmd.nread.Delta(), cmd.nrecv.Delta())
	}
}
