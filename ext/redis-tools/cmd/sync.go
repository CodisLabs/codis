package cmd

import (
	"bufio"

	"io/ioutil"
	"log"

	"sync"
	"time"
)

import (
	"github.com/spinlock/redis-tools/utils"
)

func Sync(ncpu int, from, target string) {
	master, nsize := openSyncConn(from)
	defer master.Close()

	var nread, nrestore AtomicInt64
	var wg sync.WaitGroup

	onTick := func() {
		r, s := nread.Get(), nrestore.Get()
		p := 100 * r / nsize
		log.Printf("sync: total = %d  - %3d%%, read=%-14d restore=%-14d\n", nsize, p, r, s)
	}
	onKick := func() {
		onTick()
		log.Printf("sync: done\n")
	}
	kick := goClockTicker(&wg, onTick, onKick)

	reader := bufio.NewReaderSize(master, 1024*1024*32)
	pipe := goRdbLoader(&wg, ncpu*32, reader, &nread)

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
					utils.Panic("dbnum must b 0, but got %d", e.Db)
				}
				restoreRdbEntry(c, e)
				nrestore.Add(1)
			}
		}()
	}

	wg.Wait()

	slave := openNetConn(target)
	defer slave.Close()

	var nsend, nrecv, ndiscard AtomicInt64
	goReaderWriterPipe(&wg, reader, slave, &nsend, &ndiscard, -1)
	goReaderWriterPipe(&wg, slave, ioutil.Discard, &nrecv, &ndiscard, -1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			time.Sleep(time.Second)
			s, r := nsend.Reset(), nrecv.Reset()
			log.Printf("pipe: send=%-14d recv=%-14d\n", s, r)
		}
	}()

	wg.Wait()
}
