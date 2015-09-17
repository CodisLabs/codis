// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"fmt"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type BackendConn struct {
	addr string
	auth string
	stop sync.Once

	input chan *Request
}

func NewBackendConn(addr, auth string) *BackendConn {
	bc := &BackendConn{
		addr: addr, auth: auth,
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
		} else {
			for i := len(bc.input); i != 0; i-- {
				r := <-bc.input
				bc.setResponse(r, nil, err)
			}
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
	if r.Wait != nil {
		r.Wait.Add(1)
	}
	bc.input <- r
}

func (bc *BackendConn) KeepAlive() bool {
	if len(bc.input) != 0 {
		return false
	}
	r := &Request{
		Resp: redis.NewArray([]*redis.Resp{
			redis.NewBulkBytes([]byte("PING")),
		}),
	}

	select {
	case bc.input <- r:
		return true
	default:
		return false
	}
}

var ErrFailedRequest = errors.New("discard failed request")

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
				bc.setResponse(r, nil, ErrFailedRequest)
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

	if err := bc.verifyAuth(c); err != nil {
		c.Close()
		return nil, nil, err
	}

	tasks := make(chan *Request, 4096)
	go func() {
		defer c.Close()
		for r := range tasks {
			resp, err := c.Reader.Decode()
			bc.setResponse(r, resp, err)
			if err != nil {
				// close tcp to tell writer we are failed and should quit
				c.Close()
			}
		}
	}()
	return c, tasks, nil
}

func (bc *BackendConn) verifyAuth(c *redis.Conn) error {
	if bc.auth == "" {
		return nil
	}
	resp := redis.NewArray([]*redis.Resp{
		redis.NewBulkBytes([]byte("AUTH")),
		redis.NewBulkBytes([]byte(bc.auth)),
	})

	if err := c.Writer.Encode(resp, true); err != nil {
		return err
	}

	resp, err := c.Reader.Decode()
	if err != nil {
		return err
	}
	if resp == nil {
		return errors.New(fmt.Sprintf("error resp: nil response"))
	}
	if resp.IsError() {
		return errors.New(fmt.Sprintf("error resp: %s", resp.Value))
	}
	if resp.IsString() {
		return nil
	} else {
		return errors.New(fmt.Sprintf("error resp: should be string, but got %s", resp.Type))
	}
}

func (bc *BackendConn) canForward(r *Request) bool {
	if r.Failed != nil && r.Failed.Get() {
		return false
	} else {
		return true
	}
}

func (bc *BackendConn) setResponse(r *Request, resp *redis.Resp, err error) error {
	r.Response.Resp, r.Response.Err = resp, err
	if err != nil && r.Failed != nil {
		r.Failed.Set(true)
	}
	if r.Wait != nil {
		r.Wait.Done()
	}
	if r.slot != nil {
		r.slot.Done()
	}
	return err
}

type SharedBackendConn struct {
	*BackendConn
	mu sync.Mutex

	refcnt int
}

func NewSharedBackendConn(addr, auth string) *SharedBackendConn {
	return &SharedBackendConn{BackendConn: NewBackendConn(addr, auth), refcnt: 1}
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
