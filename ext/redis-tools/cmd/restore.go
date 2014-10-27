package cmd

import (
	"bufio"

	"log"

	"sync"
)

import (
	"github.com/spinlock/redis-tools/utils"
)

func Restore(ncpu int, input, target string) {
	fin, nsize := openFileReader(input)
	defer fin.Close()

	var nread, nrestore AtomicInt64
	var wg sync.WaitGroup

	onTick := func() {
		r, s := nread.Get(), nrestore.Get()
		p := 100 * r / nsize
		log.Printf("total = %d  - %3d%%, read=%-14d restore=%-14d\n", nsize, p, r, s)
	}
	onKick := func() {
		onTick()
		log.Printf("done\n")
	}
	kick := goClockTicker(&wg, onTick, onKick)

	pipe := goRdbLoader(&wg, ncpu*32, bufio.NewReaderSize(fin, 1024*1024*32), &nread)

	for i, count := 0, AtomicInt64(ncpu); i < ncpu; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if count.Sub(1) == 0 {
					close(kick)
				}
			}()
			c := openRedisConn(target)
			defer c.Close()
			for e := range pipe {
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
