package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"sync"
)

import (
	"github.com/spinlock/redis-tools/rdb"
	"github.com/spinlock/redis-tools/utils"
)

func Decode(ncpu int, input, output string) {
	fin, nsize := openFileReader(input)
	defer fin.Close()

	fout := openFileWriter(output)
	defer fout.Close()

	var nread, nwrite AtomicInt64
	var wg sync.WaitGroup
	kick := make(chan int)

	onTick := func() {
		r, w := nread.Get(), nwrite.Get()
		p := 100 * r / nsize
		log.Printf("total = %d  - %3d%%, read=%-14d write=%-14d\n", nsize, p, r, w)
	}
	onKick := func() {
		onTick()
		log.Printf("done\n")
	}
	kick = goClockTicker(&wg, onTick, onKick)

	ipipe := goRdbLoader(&wg, ncpu*32, bufio.NewReaderSize(fin, 1024*1024*32), &nread)
	opipe := goBufWriter(&wg, ncpu*32, bufio.NewWriterSize(fout, 128*1024), &nwrite)

	for i, count := 0, AtomicInt64(ncpu); i < ncpu; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if count.Sub(1) == 0 {
					close(opipe)
					close(kick)
				}
			}()
			var b bytes.Buffer
			for e := range ipipe {
				o, err := rdb.Decode(e.Val)
				if err != nil {
					utils.Panic("decode error = '%s'", err)
				}
				key := rdb.HexToString(e.Key)
				switch obj := o.(type) {
				default:
					utils.Panic("unknown object %v", o)
				case rdb.String:
					val := rdb.HexToString(obj)
					fmt.Fprintf(&b, "db=%d type=%s expire=%d key=%s value=%s\n", e.Db, "string", e.Expire, key, val)
				case rdb.List:
					for _, x := range obj {
						ele := rdb.HexToString(x)
						fmt.Fprintf(&b, "db=%d type=%s expire=%d key=%s element=%s\n", e.Db, "list", e.Expire, key, ele)
					}
				case rdb.HashMap:
					for _, x := range obj {
						fld := rdb.HexToString(x.Field)
						mem := rdb.HexToString(x.Value)
						fmt.Fprintf(&b, "db=%d type=%s expire=%d key=%s field=%s member=%s\n", e.Db, "hset", e.Expire, key, fld, mem)
					}
				case rdb.Set:
					for _, x := range obj {
						mem := rdb.HexToString(x)
						fmt.Fprintf(&b, "db=%d type=%s expire=%d key=%s member=%s\n", e.Db, "set", e.Expire, key, mem)
					}
				case rdb.ZSet:
					for _, x := range obj {
						mem := rdb.HexToString(x.Value)
						fmt.Fprintf(&b, "db=%d type=%s expire=%d key=%s member=%s score=%f\n", e.Db, "zset", e.Expire, key, mem, x.Score)
					}
				}
				opipe <- b.String()
				b.Reset()
			}
		}()
	}

	wg.Wait()
}
