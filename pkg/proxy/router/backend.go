// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type BackendConn struct {
	addr string
	stop sync.Once

	input chan *Request
}

func NewBackendConn(addr string) *BackendConn {
	bc := &BackendConn{
		addr:  addr,
		input: make(chan *Request, 1024),
	}
	go bc.Run()
	return bc
}

func (bc *BackendConn) Run() {
	log.Infof("backend conn [%p] to %s, start service", bc, bc.addr)
	for k := 0; ; k++ {
		err := bc.loopWriter()
		if err == nil {
			break
		}
		log.WarnErrorf(err, "backend conn [%p] to %s, restart [%d]", bc, bc.addr, k)
		time.Sleep(time.Millisecond * 50)
	}
	log.Infof("backend conn [%p] to %s, stop and exit", bc, bc.addr)
}

func (bc *BackendConn) Addr() string {
	return bc.addr
}

func (bc *BackendConn) Close() {
	bc.stop.Do(func() {
		close(bc.input)
	})
}

func (bc *BackendConn) PushBack(r *Request) {
	bc.input <- r
}

var ErrZombieRequest = errors.New("request from zombie session")

func (bc *BackendConn) loopWriter() error {
	r, ok := <-bc.input
	if ok {
		c, tasks, err := bc.newBackendReader()
		if err != nil {
			return bc.setResponse(r, nil, err)
		}
		defer close(tasks)

		p := &FlushPolicy{
			Encoder:     c.Writer,
			MaxBuffered: 64,
			MaxInterval: 300,
		}
		for ok {
			var flush = len(bc.input) == 0
			if bc.canForward(r) {
				if err := p.Encode(r.Resp, flush); err != nil {
					return bc.setResponse(r, nil, err)
				}
				tasks <- r
			} else {
				if err := p.Flush(flush); err != nil {
					return bc.setResponse(r, nil, err)
				}
				bc.setResponse(r, nil, ErrZombieRequest)
			}

			r, ok = <-bc.input
		}
	}
	return nil
}

func (bc *BackendConn) newBackendReader() (*redis.Conn, chan<- *Request, error) {
	c, err := redis.DialTimeout(bc.addr, 1024*512, time.Second)
	if err != nil {
		return nil, nil, err
	}
	c.ReaderTimeout = time.Minute
	c.WriterTimeout = time.Minute

	tasks := make(chan *Request, 4096)
	go func() {
		defer c.Close()
		for r := range tasks {
			resp, err := c.Reader.Decode()
			bc.setResponse(r, resp, err)
		}
	}()
	return c, tasks, nil
}

func (bc *BackendConn) canForward(r *Request) bool {
	return r.Owner == nil || !r.Owner.IsZombie()
}

func (bc *BackendConn) setResponse(r *Request, resp *redis.Resp, err error) error {
	r.Response.Resp, r.Response.Err = resp, err
	if s := r.slot; s != nil {
		s.jobs.Done()
	}
	if err != nil && r.Owner != nil {
		r.Owner.MarkZombie()
	}
	r.Wait.Done()
	return err
}

type SharedBackendConn struct {
	*BackendConn
	mu sync.Mutex

	refcnt int
}

func NewSharedBackendConn(addr string) *SharedBackendConn {
	return &SharedBackendConn{BackendConn: NewBackendConn(addr), refcnt: 1}
}

func (s *SharedBackendConn) Close() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refcnt <= 0 {
		log.Panicf("shared backend conn has been closed, close too many times")
	}
	if s.refcnt == 1 {
		s.BackendConn.Close()
	}
	s.refcnt--
	return s.refcnt == 0
}

func (s *SharedBackendConn) IncrRefcnt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refcnt == 0 {
		log.Panicf("shared backend conn has been closed")
	}
	s.refcnt++
}

type FlushPolicy struct {
	*redis.Encoder

	MaxBuffered int
	MaxInterval int64

	nbuffered int
	lastflush int64
}

func (p *FlushPolicy) needFlush() bool {
	if p.nbuffered != 0 {
		if p.nbuffered > p.MaxBuffered {
			return true
		}
		if microseconds()-p.lastflush > p.MaxInterval {
			return true
		}
	}
	return false
}

func (p *FlushPolicy) Flush(force bool) error {
	if force || p.needFlush() {
		if err := p.Encoder.Flush(); err != nil {
			return err
		}
		p.nbuffered = 0
		p.lastflush = microseconds()
	}
	return nil
}

func (p *FlushPolicy) Encode(resp *redis.Resp, force bool) error {
	if err := p.Encoder.Encode(resp, false); err != nil {
		return err
	} else {
		p.nbuffered++
		return p.Flush(force)
	}
}
