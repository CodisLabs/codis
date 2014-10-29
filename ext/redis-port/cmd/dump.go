package cmd

import (
	"bufio"
	"log"
	"sync"
	"time"
)

import (
	"github.com/wandoulabs/codis/ext/redis-port/utils"
)

func Dump(ncpu int, from, output string) {
	log.Printf("[ncpu=%d] dump from '%s' to '%s'\n", ncpu, from, output)

	fout := openWriteFile(output)
	defer fout.Close()

	master, wait := openSyncConn(from)
	defer master.Close()

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

	var nread, nwrite AtomicInt64
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			r, w := nread.Get(), nwrite.Get()
			p := 100 * r / nsize
			log.Printf("total = %d  - %3d%%, read=%-14d write=%-14d\n", nsize, p, r, w)
			if nsize == r {
				log.Printf("done\n")
				return
			}
			time.Sleep(time.Second)
		}
	}()

	reader := bufio.NewReaderSize(master, 1024*1024*32)
	writer := bufio.NewWriterSize(fout, 1024*64)

	PipeReaderWriter(&wg, reader, writer, &nread, &nwrite, nsize)

	wg.Wait()

	if err := writer.Flush(); err != nil {
		utils.Panic("writer flush error = '%s'", err)
	}
}
