// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/iocount"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/utils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

type cmdDecode struct {
	nread, nsave, nobjs atomic2.AtomicInt64
}

func (cmd *cmdDecode) Main() {
	ncpu, input, output := args.ncpu, args.input, args.output
	if len(input) == 0 {
		input = "/dev/stdin"
	}
	if len(output) == 0 {
		output = "/dev/stdout"
	}

	log.Printf("[ncpu=%d] decode from '%s' to '%s'\n", ncpu, input, output)

	var readin io.ReadCloser
	var nsize int64
	if input != "/dev/stdin" {
		readin, nsize = openReadFile(input)
		defer readin.Close()
	} else {
		readin, nsize = os.Stdin, 0
	}

	var saveto io.WriteCloser
	if output != "/dev/stdout" {
		saveto = openWriteFile(output)
		defer saveto.Close()
	} else {
		saveto = os.Stdout
	}

	reader := bufio.NewReaderSize(iocount.NewReaderWithCounter(readin, &cmd.nread), ReaderBufferSize)
	writer := bufio.NewWriterSize(iocount.NewWriterWithCounter(saveto, &cmd.nsave), WriterBufferSize)

	ipipe := newRDBLoader(reader, ncpu*32)
	opipe := make(chan string, cap(ipipe))

	go func() {
		defer close(opipe)
		group := make(chan int)
		for i := 0; i < ncpu; i++ {
			go func() {
				defer func() {
					group <- 0
				}()
				cmd.decoderMain(ipipe, opipe)
			}()
		}
		for i := 0; i < ncpu; i++ {
			<-group
		}
	}()

	wait := make(chan struct{})
	go func() {
		defer close(wait)
		for s := range opipe {
			if _, err := writer.WriteString(s); err != nil {
				utils.ErrorPanic(err, "write string failed")
			}
			flushWriter(writer)
		}
	}()

	for done := false; !done; {
		select {
		case <-wait:
			done = true
		case <-time.After(time.Second):
		}
		n, w, o := cmd.nread.Get(), cmd.nsave.Get(), cmd.nobjs.Get()
		if nsize != 0 {
			p := 100 * n / nsize
			log.Printf("total = %d - %12d [%3d%%]  write=%-12d objs=%d\n", nsize, n, p, w, o)
		} else {
			log.Printf("total = %12d  write=%-12d objs=%d\n", n, w, o)
		}
	}
	log.Println("done")
}

func (cmd *cmdDecode) decoderMain(ipipe <-chan *rdb.Entry, opipe chan<- string) {
	toText := func(p []byte) string {
		var b bytes.Buffer
		for _, c := range p {
			switch {
			case c >= '#' && c <= '~':
				b.WriteByte(c)
			default:
				b.WriteByte('.')
			}
		}
		return b.String()
	}
	toBase64 := func(p []byte) string {
		return base64.StdEncoding.EncodeToString(p)
	}
	toJson := func(o interface{}) string {
		b, err := json.Marshal(o)
		if err != nil {
			utils.ErrorPanic(err, "encode to json failed")
		}
		return string(b)
	}
	for e := range ipipe {
		o, err := rdb.DecodeDump(e.ValDump)
		if err != nil {
			utils.ErrorPanic(err, "decode failed")
		}
		var b bytes.Buffer
		switch obj := o.(type) {
		default:
			utils.Panic("unknown object %v", o)
		case rdb.String:
			o := &struct {
				DB       uint32 `json:"db"`
				Type     string `json:"type"`
				ExpireAt uint64 `json:"expireat"`
				Key      string `json:"key"`
				Key64    string `json:"key64"`
				Value64  string `json:"value64"`
			}{
				e.DB, "string", e.ExpireAt, toText(e.Key), toBase64(e.Key),
				toBase64(obj),
			}
			fmt.Fprintf(&b, "%s\n", toJson(o))
		case rdb.List:
			for i, ele := range obj {
				o := &struct {
					DB       uint32 `json:"db"`
					Type     string `json:"type"`
					ExpireAt uint64 `json:"expireat"`
					Key      string `json:"key"`
					Key64    string `json:"key64"`
					Index    int    `json:"index"`
					Value64  string `json:"value64"`
				}{
					e.DB, "list", e.ExpireAt, toText(e.Key), toBase64(e.Key),
					i, toBase64(ele),
				}
				fmt.Fprintf(&b, "%s\n", toJson(o))
			}
		case rdb.Hash:
			for _, ele := range obj {
				o := &struct {
					DB       uint32 `json:"db"`
					Type     string `json:"type"`
					ExpireAt uint64 `json:"expireat"`
					Key      string `json:"key"`
					Key64    string `json:"key64"`
					Field    string `json:"field"`
					Field64  string `json:"field64"`
					Value64  string `json:"value64"`
				}{
					e.DB, "hash", e.ExpireAt, toText(e.Key), toBase64(e.Key),
					toText(ele.Field), toBase64(ele.Field), toBase64(ele.Value),
				}
				fmt.Fprintf(&b, "%s\n", toJson(o))
			}
		case rdb.Set:
			for _, mem := range obj {
				o := &struct {
					DB       uint32 `json:"db"`
					Type     string `json:"type"`
					ExpireAt uint64 `json:"expireat"`
					Key      string `json:"key"`
					Key64    string `json:"key64"`
					Member   string `json:"member"`
					Member64 string `json:"member64"`
				}{
					e.DB, "set", e.ExpireAt, toText(e.Key), toBase64(e.Key),
					toText(mem), toBase64(mem),
				}
				fmt.Fprintf(&b, "%s\n", toJson(o))
			}
		case rdb.ZSet:
			for _, ele := range obj {
				o := &struct {
					DB       uint32  `json:"db"`
					Type     string  `json:"type"`
					ExpireAt uint64  `json:"expireat"`
					Key      string  `json:"key"`
					Key64    string  `json:"key64"`
					Member   string  `json:"member"`
					Member64 string  `json:"member64"`
					Score    float64 `json:"score"`
				}{
					e.DB, "zset", e.ExpireAt, toText(e.Key), toBase64(e.Key),
					toText(ele.Member), toBase64(ele.Member), ele.Score,
				}
				fmt.Fprintf(&b, "%s\n", toJson(o))
			}
		}
		cmd.nobjs.Incr()
		opipe <- b.String()
	}
}
