// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bufio"
	"io"
	"os"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

type cmdRestore struct {
	nread, nobjs atomic2.Int64
}

func (cmd *cmdRestore) Main() {
	input, target := args.input, args.target
	if len(target) == 0 {
		log.Panic("invalid argument: target")
	}
	if len(input) == 0 {
		input = "/dev/stdin"
	}

	log.Infof("restore from '%s' to '%s'\n", input, target)

	var readin io.ReadCloser
	var nsize int64
	if input != "/dev/stdin" {
		readin, nsize = openReadFile(input)
		defer readin.Close()
	} else {
		readin, nsize = os.Stdin, 0
	}

	reader := bufio.NewReaderSize(ioutils.NewCountReader(readin, &cmd.nread), ReaderBufferSize)

	cmd.RestoreRDBFile(reader, target, nsize)

	if !args.extra {
		return
	}

	if nsize != 0 && nsize == cmd.nread.Get() {
		return
	}

	cmd.RestoreCommand(reader, target, nsize)
}

func (cmd *cmdRestore) RestoreRDBFile(reader *bufio.Reader, target string, nsize int64) {
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
				c := openRedisConn(target)
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
		if nsize != 0 {
			p := 100 * n / nsize
			log.Infof("total = %d - %12d [%3d%%]  objs=%d\n", nsize, n, p, o)
		} else {
			log.Infof("total = %12d  objs=%d\n", n, o)
		}
	}
	log.Info("restore: rdb done")
}

func (cmd *cmdRestore) RestoreCommand(reader *bufio.Reader, slave string, nsize int64) {
	var forward, nbypass atomic2.Int64
	c := openNetConn(slave)
	defer c.Close()

	writer := bufio.NewWriterSize(c, WriterBufferSize)
	defer flushWriter(writer)

	discard := bufio.NewReaderSize(c, ReaderBufferSize)

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
			redis.MustEncode(writer, resp)
			flushWriter(writer)
			forward.Incr()
			redis.MustDecode(discard)
		}
	}()

	for {
		forward.Snapshot()
		nbypass.Snapshot()
		time.Sleep(time.Second)
		log.Infof("restore: +forward=%-6d  +bypass=%-6d\n", forward.Delta(), nbypass.Delta())
	}
}
