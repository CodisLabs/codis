package cmd

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
)

import (
	"github.com/wandoulabs/codis/ext/redis-port/rdb"
	"github.com/wandoulabs/codis/ext/redis-port/utils"
)

func Decode(ncpu int, input, output string) {
	log.Printf("[ncpu=%d] decode from '%s' to '%s'\n", ncpu, input, output)

	fin, nsize := openReadFile(input)
	defer fin.Close()

	fout := openWriteFile(output)
	defer fout.Close()

	var nread, nwrite AtomicInt64
	var wg sync.WaitGroup

	onTick := func() {
		r, w := nread.Get(), nwrite.Get()
		if nsize != 0 {
			p := 100 * r / nsize
			log.Printf("total = %d  - %3d%%, read=%-14d write=%-14d\n", nsize, p, r, w)
		} else {
			log.Printf("total = unknown  -  read=%-14d write=%-14d\n", r, w)
		}
	}
	onClose := func() {
		onTick()
		log.Printf("done\n")
	}
	ticker := NewClockTicker(&wg, onTick, onClose)

	loader := NewRdbLoader(&wg, ncpu*32, bufio.NewReaderSize(fin, 1024*1024*32), &nread)
	writer := NewBufWriter(&wg, ncpu*32, bufio.NewWriterSize(fout, 128*1024), &nwrite)

	for i, count := 0, AtomicInt64(ncpu); i < ncpu; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if count.Sub(1) == 0 {
					writer.Close()
					ticker.Close()
				}
			}()
			toHexString := func(p []byte) string {
				var b bytes.Buffer
				b.WriteByte('{')
				for _, c := range p {
					switch {
					case c >= 'a' && c <= 'z':
						fallthrough
					case c >= 'A' && c <= 'Z':
						fallthrough
					case c >= '0' && c <= '9':
						b.WriteByte(c)
					default:
						b.WriteByte('.')
					}
				}
				b.WriteByte('|')
				b.WriteString(hex.EncodeToString(p))
				b.WriteByte('}')
				return b.String()
			}
			for e := range loader.Pipe() {
				o, err := rdb.DecodeDump(e.ValDump)
				if err != nil {
					utils.Panic("decode error = '%s'", err)
				}
				var b bytes.Buffer
				key := toHexString(e.Key)
				switch obj := o.(type) {
				default:
					utils.Panic("unknown object %v", o)
				case rdb.String:
					val := toHexString(obj)
					fmt.Fprintf(&b, "db=%d type=%s expireat=%d key=%s value=%s\n", e.DB, "string", e.ExpireAt, key, val)
				case rdb.List:
					for _, x := range obj {
						ele := toHexString(x)
						fmt.Fprintf(&b, "db=%d type=%s expireat=%d key=%s element=%s\n", e.DB, "list", e.ExpireAt, key, ele)
					}
				case rdb.HashMap:
					for _, x := range obj {
						fld := toHexString(x.Field)
						mem := toHexString(x.Value)
						fmt.Fprintf(&b, "db=%d type=%s expireat=%d key=%s field=%s member=%s\n", e.DB, "hset", e.ExpireAt, key, fld, mem)
					}
				case rdb.Set:
					for _, x := range obj {
						mem := toHexString(x)
						fmt.Fprintf(&b, "db=%d type=%s expireat=%d key=%s member=%s\n", e.DB, "set", e.ExpireAt, key, mem)
					}
				case rdb.ZSet:
					for _, x := range obj {
						mem := toHexString(x.Value)
						fmt.Fprintf(&b, "db=%d type=%s expireat=%d key=%s member=%s score=%f\n", e.DB, "zset", e.ExpireAt, key, mem, x.Score)
					}
				}
				writer.Append(b.String())
			}
		}()
	}

	wg.Wait()
}
