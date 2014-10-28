package cmd

import (
	"bufio"
	"log"
	"sync"
)

import (
	"github.com/wandoulabs/codis/ext/redis-port/utils"
)

func Restore(ncpu int, input, target string) {
	log.Printf("[ncpu=%d] restore from `%s' to `%s'\n", ncpu, input, target)

	fin, nsize := openReadFile(input)
	defer fin.Close()

	var nread, nrestore AtomicInt64
	var wg sync.WaitGroup

	onTick := func() {
		r, s := nread.Get(), nrestore.Get()
		if nsize != 0 {
			p := 100 * r / nsize
			log.Printf("total = %d  - %3d%%, read=%-14d restore=%-14d\n", nsize, p, r, s)
		} else {
			log.Printf("total = unknown  -  read=%-14d restore=%-14d\n", r, s)
		}
	}
	onClose := func() {
		onTick()
		log.Printf("done\n")
	}
	ticker := NewClockTicker(&wg, onTick, onClose)

	loader := NewRdbLoader(&wg, ncpu*32, bufio.NewReaderSize(fin, 1024*1024*32), &nread)

	for i, count := 0, AtomicInt64(ncpu); i < ncpu; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if count.Sub(1) == 0 {
					ticker.Close()
				}
			}()
			c := openRedisConn(target)
			defer c.Close()
			for e := range loader.Pipe() {
				if e.Db != 0 {
					utils.Panic("dbnum must be 0, but got %d", e.Db)
				}
				restoreRdbEntry(c, e)
				nrestore.Add(1)
			}
		}()
	}

	wg.Wait()
}
