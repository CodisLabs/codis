// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/iocount"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/utils"
)

type cmdDump struct {
	ndump atomic2.AtomicInt64
}

func (cmd *cmdDump) Main() {
	ncpu, from, output := args.ncpu, args.from, args.output
	if len(from) == 0 {
		utils.Panic("invalid argument: from")
	}
	if len(output) == 0 {
		output = "/dev/stdout"
	}

	log.Printf("[ncpu=%d] dump from '%s' to '%s'\n", ncpu, from, output)

	var dumpto io.WriteCloser
	if output != "/dev/stdout" {
		dumpto = openWriteFile(output)
		defer dumpto.Close()
	} else {
		dumpto = os.Stdout
	}

	master, nsize := cmd.SendCmd(from)
	defer master.Close()

	log.Printf("rdb file = %d\n", nsize)

	reader := bufio.NewReaderSize(master, ReaderBufferSize)
	writer := bufio.NewWriterSize(iocount.NewWriterWithCounter(dumpto, &cmd.ndump), WriterBufferSize)

	cmd.DumpRDBFile(reader, writer, nsize)

	if !args.extra {
		return
	}

	cmd.DumpCommand(reader, writer)
}

func (cmd *cmdDump) SendCmd(master string) (net.Conn, int64) {
	c, wait := openSyncConn(master)
	var nsize int64
	for nsize == 0 {
		select {
		case nsize = <-wait:
			if nsize == 0 {
				log.Println("+")
			}
		case <-time.After(time.Second):
			log.Println("-")
		}
	}
	return c, nsize
}

func (cmd *cmdDump) DumpRDBFile(reader *bufio.Reader, writer *bufio.Writer, nsize int64) {
	var nread atomic2.AtomicInt64
	wait := make(chan struct{})
	go func() {
		defer close(wait)
		p := make([]byte, WriterBufferSize)
		for nsize != nread.Get() {
			cnt := iocopy(reader, writer, p, int(nsize-nread.Get()))
			nread.Add(int64(cnt))
		}
		flushWriter(writer)
	}()

	for done := false; !done; {
		select {
		case <-wait:
			done = true
		case <-time.After(time.Second):
		}
		n := nread.Get()
		p := 100 * n / nsize
		log.Printf("total = %d - %12d [%3d%%]\n", nsize, n, p)
	}
	log.Println("dump: rdb done")
}

func (cmd *cmdDump) DumpCommand(reader *bufio.Reader, writer *bufio.Writer) {
	go func() {
		p := make([]byte, ReaderBufferSize)
		for {
			iocopy(reader, writer, p, len(p))
			flushWriter(writer)
		}
	}()

	for {
		time.Sleep(time.Second)
		log.Printf("dump: size = %d\n", cmd.ndump.Get())
	}
}
