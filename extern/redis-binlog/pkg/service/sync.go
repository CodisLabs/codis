// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/binlog"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/pipe"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

// BGSAVE
func (h *Handler) Bgsave(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 0 {
		return toRespErrorf("len(args) = %d, expect = 0", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	bg := h.counters.bgsave.Add(1)
	defer h.counters.bgsave.Sub(1)

	if bg != 1 {
		return toRespErrorf("bgsave is busy: %d, should be 1")
	}

	sp, err := s.Binlog().NewSnapshot()
	if err != nil {
		return toRespError(err)
	}
	defer s.Binlog().ReleaseSnapshot(sp)

	if err := h.bgsaveTo(sp, h.config.DumpPath); err != nil {
		return toRespError(err)
	} else {
		return redis.NewString("OK"), nil
	}
}

// BGSAVETO path
func (h *Handler) BgsaveTo(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 1 {
		return toRespErrorf("len(args) = %d, expect = 1", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	bg := h.counters.bgsave.Add(1)
	defer h.counters.bgsave.Sub(1)

	if bg != 1 {
		return toRespErrorf("bgsave is busy: %d, should be 1", bg)
	}

	sp, err := s.Binlog().NewSnapshot()
	if err != nil {
		return toRespError(err)
	}
	defer s.Binlog().ReleaseSnapshot(sp)

	if err := h.bgsaveTo(sp, string(args[0])); err != nil {
		return toRespError(err)
	} else {
		return redis.NewString("OK"), nil
	}
}

func (h *Handler) bgsaveTo(sp *binlog.BinlogSnapshot, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Trace(err)
	}
	defer f.Close()

	buf := bufio.NewWriterSize(f, 1024*1024)
	enc := rdb.NewEncoder(buf)

	if err := enc.EncodeHeader(); err != nil {
		return err
	}

	ncpu := runtime.GOMAXPROCS(0)
	cron := time.Millisecond * time.Duration(100)
	for {
		objs, more, err := sp.LoadObjCron(cron, ncpu, 1024)
		if err != nil {
			return err
		} else {
			for _, obj := range objs {
				if err := enc.EncodeObject(obj.DB, obj.Key, obj.ExpireAt, obj.Value); err != nil {
					return err
				}
			}
		}
		if !more {
			break
		}
	}

	if err := enc.EncodeFooter(); err != nil {
		return err
	}

	if err := errors.Trace(buf.Flush()); err != nil {
		return err
	}
	return errors.Trace(f.Close())
}

// SLAVEOF host port
func (h *Handler) SlaveOf(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 2 {
		return toRespErrorf("len(args) = %d, expect = 2", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	addr := fmt.Sprintf("%s:%s", string(args[0]), string(args[1]))
	log.Infof("set slave of %s", addr)

	var c *conn
	if strings.ToLower(addr) != "no:one" {
		if nc, err := net.DialTimeout("tcp", addr, time.Second); err != nil {
			return toRespError(errors.Trace(err))
		} else {
			c = newConn(nc, s.Binlog(), 0)
			if err := c.ping(); err != nil {
				c.Close()
				return toRespError(err)
			}
		}
	}
	select {
	case <-h.signal:
		if c != nil {
			c.Close()
		}
		return toRespErrorf("sync master has been closed")
	case h.master <- c:
		return redis.NewString("OK"), nil
	}
}

func (h *Handler) daemonSyncMaster() {
	var last *conn
	lost := make(chan int, 0)
	for exit := false; !exit; {
		var c *conn
		select {
		case <-lost:
			last = nil
		case <-h.signal:
			exit = true
		case c = <-h.master:
		}
		if last != nil {
			last.Close()
			<-lost
		}
		last = c
		if c != nil {
			go func() {
				defer func() {
					lost <- 0
				}()
				defer c.Close()
				err := h.doSyncTo(c)
				log.InfoErrorf(err, "stop sync: %s", c.summ)
			}()
			h.syncto = c.nc.RemoteAddr().String()
			h.syncto_since = time.Now().UnixNano() / int64(time.Millisecond)
			log.Infof("sync to %s", h.syncto)
		} else {
			h.syncto = ""
			h.syncto_since = 0
			log.Infof("sync to no one")
		}
	}
}

func (h *Handler) doSyncTo(c *conn) error {
	defer func() {
		h.counters.syncTotalBytes.Set(0)
		h.counters.syncCacheBytes.Set(0)
	}()

	filePath := h.config.SyncFilePath
	fileSize := h.config.SyncFileSize
	buffSize := h.config.SyncBuffSize

	var file *os.File
	if filePath != "" {
		f, err := pipe.OpenFile(filePath, false)
		if err != nil {
			log.ErrorErrorf(err, "open pipe file '%s' failed", filePath)
		} else {
			file = f
		}
	}

	pr, pw := pipe.PipeFile(buffSize, fileSize, file)
	defer pr.Close()

	wg := &sync.WaitGroup{}
	defer wg.Wait()

	wg.Add(1)
	go func(r io.Reader) {
		defer wg.Done()
		defer pw.Close()
		p := make([]byte, 8192)
		for {
			deadline := time.Now().Add(time.Minute)
			if err := c.nc.SetReadDeadline(deadline); err != nil {
				pr.CloseWithError(errors.Trace(err))
				return
			}
			n, err := r.Read(p)
			if err != nil {
				pr.CloseWithError(err)
				return
			}
			h.counters.syncTotalBytes.Add(int64(n))
			s := p[:n]
			for len(s) != 0 {
				n, err := pw.Write(s)
				if err != nil {
					pr.CloseWithError(err)
					return
				}
				s = s[n:]
			}
		}
	}(c.r)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			time.Sleep(time.Millisecond * 200)
			n, err := pr.Buffered()
			if err != nil {
				return
			}
			h.counters.syncCacheBytes.Set(int64(n))
		}
	}()

	c.r = bufio.NewReader(pr)

	size, err := c.presync()
	if err != nil {
		return err
	}
	log.Infof("sync rdb file size = %d bytes\n", size)

	c.w = bufio.NewWriter(ioutil.Discard)

	if err := c.Binlog().Reset(); err != nil {
		return err
	}

	if err := h.doSyncRDB(c, size); err != nil {
		return err
	}
	log.Infof("sync rdb done")

	return c.serve(h)
}

func (h *Handler) doSyncRDB(c *conn, size int64) error {
	defer h.counters.syncRdbRemains.Set(0)
	h.counters.syncRdbRemains.Set(size)

	r := ioutils.NewCountReader(c.r, nil)
	l := rdb.NewLoader(r)
	if err := l.Header(); err != nil {
		return err
	}

	ncpu := runtime.GOMAXPROCS(0)
	errs := make(chan error, ncpu)

	var lock sync.Mutex
	var flag atomic2.Int64
	loadNextEntry := func() (*rdb.BinEntry, error) {
		lock.Lock()
		defer lock.Unlock()
		if flag.Get() != 0 {
			return nil, nil
		}
		entry, err := l.NextBinEntry()
		if err != nil || entry == nil {
			flag.Set(1)
			return nil, err
		}
		return entry, nil
	}

	for i := 0; i < ncpu; i++ {
		go func() {
			defer flag.Set(1)
			for {
				entry, err := loadNextEntry()
				if err != nil || entry == nil {
					errs <- err
					return
				}
				db, key, value := entry.DB, entry.Key, entry.Value
				ttlms := int64(0)
				if entry.ExpireAt != 0 {
					if v, ok := binlog.ExpireAtToTTLms(entry.ExpireAt); ok && v > 0 {
						ttlms = v
					} else {
						ttlms = 1
					}
				}
				if err := c.Binlog().SlotsRestore(db, key, ttlms, value); err != nil {
					errs <- err
					return
				}
			}
		}()
	}

	for {
		select {
		case <-time.After(time.Second):
			h.counters.syncRdbRemains.Set(size - r.Count())
		case err := <-errs:
			for i := 1; i < cap(errs); i++ {
				e := <-errs
				if err == nil && e != nil {
					err = e
				}
			}
			if err != nil {
				return err
			}
			return l.Footer()
		}
	}
}
