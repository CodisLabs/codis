// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bufio"
	"io"
	"net"
	"os"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
)

type cmdDump struct {
	ndump atomic2.Int64
}

func (cmd *cmdDump) Main() {
	from, output := args.from, args.output
	if len(from) == 0 {
		log.Panic("invalid argument: from")
	}
	if len(output) == 0 {
		output = "/dev/stdout"
	}

	log.Infof("dump from '%s' to '%s'\n", from, output)

	var dumpto io.WriteCloser
	if output != "/dev/stdout" {
		dumpto = openWriteFile(output)
		defer dumpto.Close()
	} else {
		dumpto = os.Stdout
	}

	master, nsize := cmd.SendCmd(from)
	defer master.Close()

	log.Infof("rdb file = %d\n", nsize)

	reader := bufio.NewReaderSize(master, ReaderBufferSize)
	writer := bufio.NewWriterSize(ioutils.NewCountWriter(dumpto, &cmd.ndump), WriterBufferSize)

	cmd.DumpRDBFile(reader, writer, nsize)

	if !args.extra {
		return
	}

	cmd.DumpCommand(reader, writer)
}

func (cmd *cmdDump) SendCmd(master string) (net.Conn, int64) {
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

func (cmd *cmdDump) DumpRDBFile(reader *bufio.Reader, writer *bufio.Writer, nsize int64) {
	var nread atomic2.Int64
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
		log.Infof("total = %d - %12d [%3d%%]\n", nsize, n, p)
	}
	log.Info("dump: rdb done")
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
		log.Infof("dump: size = %d\n", cmd.ndump.Get())
	}
}
