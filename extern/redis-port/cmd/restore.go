// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bufio"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/iocount"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/utils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

type cmdRestore struct {
	nread, nrecv, nobjs atomic2.AtomicInt64
}

func (cmd *cmdRestore) Main() {
	ncpu, input, target := args.ncpu, args.input, args.target
	if len(target) == 0 {
		utils.Panic("invalid argument: target")
	}
	if len(input) == 0 {
		input = "/dev/stdin"
	}

	log.Printf("[ncpu=%d] restore from '%s' to '%s'\n", ncpu, input, target)

	var readin io.ReadCloser
	var nsize int64
	if input != "/dev/stdin" {
		readin, nsize = openReadFile(input)
		defer readin.Close()
	} else {
		readin, nsize = os.Stdin, 0
	}

	reader := bufio.NewReaderSize(iocount.NewReaderWithCounter(readin, &cmd.nread), ReaderBufferSize)

	cmd.RestoreRDBFile(reader, target, nsize, ncpu)

	if !args.extra {
		return
	}

	if nsize != 0 && nsize == cmd.nread.Get() {
		return
	}

	cmd.RestoreCommand(reader, target, nsize)
}

func (cmd *cmdRestore) RestoreRDBFile(reader *bufio.Reader, target string, nsize int64, ncpu int) {
	pipe := newRDBLoader(reader, ncpu*32)
	wait := make(chan struct{})
	go func() {
		defer close(wait)
		group := make(chan int)
		for i := 0; i < ncpu; i++ {
			go func() {
				defer func() {
					group <- 0
				}()
				c := openRedisConn(target)
				defer c.Close()
				var lastdb uint32 = 0
				for e := range pipe {
					if !acceptDB(int64(e.DB)) {
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
		for i := 0; i < ncpu; i++ {
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
			log.Printf("total = %d - %12d [%3d%%]  objs=%d\n", nsize, n, p, o)
		} else {
			log.Printf("total = %12d  objs=%d\n", n, o)
		}
	}
	log.Println("restore: rdb done")
}

func (cmd *cmdRestore) RestoreCommand(reader *bufio.Reader, slave string, nsize int64) {
	var forward, nbypass atomic2.AtomicInt64
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

	go func() {
		var bypass bool = false
		for {
			resp := redis.MustDecode(reader)
			if cmd, args, err := redis.ParseArgs(resp); err != nil {
				utils.ErrorPanic(err, "parse command arguments failed")
			} else if cmd != "ping" {
				if cmd == "select" {
					if len(args) != 1 {
						utils.Panic("select command len(args) = %d", len(args))
					}
					s := string(args[0])
					n, err := strconv.ParseInt(s, 10, 64)
					if err != nil {
						utils.ErrorPanic(err, "parse db = '%s' failed", s)
					}
					bypass = !acceptDB(n)
				}
				if bypass {
					nbypass.Incr()
					continue
				}
			}
			redis.MustEncode(writer, resp)
			flushWriter(writer)
			forward.Incr()
		}
	}()

	for {
		forward.Snapshot()
		nbypass.Snapshot()
		cmd.nrecv.Snapshot()
		time.Sleep(time.Second)
		log.Printf("restore: +forward=%-6d  +bypass=%-6d  +nrecv=%-9d\n", forward.Delta(), nbypass.Delta(), cmd.nrecv.Delta())
	}
}
