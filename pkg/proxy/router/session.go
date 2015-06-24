// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"container/list"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
	"github.com/wandoulabs/codis/pkg/utils/atomic2"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Session struct {
	*redis.Conn

	Sid int64
	Ops atomic2.Int64

	LastOpUnix atomic2.Int64
	CreateUnix int64

	quit   bool
	zombie atomic2.Bool
	closed atomic2.Bool
}

func (s *Session) String() string {
	o := &struct {
		Sid        int64  `json:"sid"`
		Ops        int64  `json:"ops"`
		LastOpUnix int64  `json:"lastop"`
		CreateUnix int64  `json:"create"`
		RemoteAddr string `json:"remote"`
	}{
		s.Sid, s.Ops.Get(), s.LastOpUnix.Get(), s.CreateUnix,
		s.Conn.Sock.RemoteAddr().String(),
	}
	b, _ := json.Marshal(o)
	return string(b)
}

func NewSession(c net.Conn) *Session {
	s := &Session{Sid: sessions.sid.Incr(), CreateUnix: time.Now().Unix()}
	s.Conn = redis.NewConn(c)
	s.Conn.ReaderTimeout = time.Minute * 30
	s.Conn.WriterTimeout = time.Second * 30
	log.Infof("session [%p] create: %s", s, s)
	return addToSessions(s)
}

func (s *Session) MarkZombie() {
	s.zombie.Set(true)
}

func (s *Session) IsZombie() bool {
	return s.zombie.Get()
}

func (s *Session) IsClosed() bool {
	return s.closed.Get()
}

func (s *Session) IsTimeout(lastunix int64) bool {
	return s.LastOpUnix.Get() < lastunix && s.CreateUnix < lastunix
}

func (s *Session) Serve(d Dispatcher) {
	var errlist errors.ErrorList
	defer func() {
		if err := errlist.First(); err != nil {
			log.Infof("session [%p] closed: %s, error = %s", s, s, err)
		} else {
			log.Infof("session [%p] closed: %s, quit", s, s)
		}
	}()

	tasks := make(chan *Request, 256)
	go func() {
		defer func() {
			s.Close()
			for _ = range tasks {
			}
			s.closed.Set(true)
		}()
		if err := s.loopWriter(tasks); err != nil {
			errlist.PushBack(err)
		}
		s.MarkZombie()
	}()

	defer close(tasks)
	if err := s.loopReader(tasks, d); err != nil {
		errlist.PushBack(err)
	}
}

func (s *Session) loopReader(tasks chan<- *Request, d Dispatcher) error {
	if d == nil {
		return errors.New("nil dispatcher")
	}
	for !s.quit {
		resp, err := s.Reader.Decode()
		if err != nil {
			return err
		}
		r, err := s.handleRequest(resp, d)
		if err != nil {
			return err
		} else {
			tasks <- r
		}
	}
	return nil
}

func (s *Session) loopWriter(tasks <-chan *Request) error {
	p := &FlushPolicy{
		Encoder:     s.Writer,
		MaxBuffered: 32,
		MaxInterval: 300,
	}
	for r := range tasks {
		resp, err := s.handleResponse(r)
		if err != nil {
			return err
		}
		if err := p.Encode(resp, len(tasks) == 0); err != nil {
			return err
		}
	}
	return nil
}

var ErrRespIsRequired = errors.New("resp is required")

func (s *Session) handleResponse(r *Request) (*redis.Resp, error) {
	r.Wait.Wait()
	if r.Coalesce != nil {
		if err := r.Coalesce(); err != nil {
			return nil, err
		}
	}
	resp, err := r.Response.Resp, r.Response.Err
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, ErrRespIsRequired
	}
	incrOpStats(r.OpStr, microseconds()-r.Start)
	return resp, nil
}

func (s *Session) handleRequest(resp *redis.Resp, d Dispatcher) (*Request, error) {
	opstr, err := getOpStr(resp)
	if err != nil {
		return nil, err
	}
	if isNotAllowed(opstr) {
		return nil, errors.New(fmt.Sprintf("command <%s> is not allowed", opstr))
	}

	usnow := microseconds()
	s.LastOpUnix.Set(usnow / 1e6)

	r := &Request{
		Owner: s,
		OpSeq: s.Ops.Incr(),
		OpStr: opstr,
		Start: usnow,
		Wait:  &sync.WaitGroup{},
		Resp:  resp,
	}

	switch opstr {
	case "QUIT":
		s.quit = true
		fallthrough
	case "AUTH", "SELECT":
		r.Response.Resp = redis.NewString([]byte("OK"))
		return r, nil
	case "MGET":
		return s.handleRequestMGet(r, d)
	case "MSET":
		return s.handleRequestMSet(r, d)
	case "DEL":
		return s.handleRequestMDel(r, d)
	}
	return r, d.Dispatch(r)
}

func (s *Session) handleRequestMGet(r *Request, d Dispatcher) (*Request, error) {
	nkeys := len(r.Resp.Array) - 1
	if nkeys <= 1 {
		return r, d.Dispatch(r)
	}
	var sub = make([]*Request, nkeys)
	for i := 0; i < len(sub); i++ {
		sub[i] = &Request{
			Owner: r.Owner,
			OpSeq: -r.OpSeq,
			OpStr: r.OpStr,
			Start: r.Start,
			Wait:  r.Wait,
			Resp: redis.NewArray([]*redis.Resp{
				r.Resp.Array[0],
				r.Resp.Array[i+1],
			}),
		}
		if err := d.Dispatch(sub[i]); err != nil {
			return nil, err
		}
	}
	r.Coalesce = func() error {
		var array = make([]*redis.Resp, len(sub))
		for i, x := range sub {
			if err := x.Response.Err; err != nil {
				return err
			}
			resp := x.Response.Resp
			if resp == nil {
				return ErrRespIsRequired
			}
			if !resp.IsArray() || len(resp.Array) != 1 {
				return errors.New(fmt.Sprintf("bad mget resp: %s array.len = %d", resp.Type, len(resp.Array)))
			}
			array[i] = resp.Array[0]
		}
		r.Response.Resp = redis.NewArray(array)
		return nil
	}
	return r, nil
}

func (s *Session) handleRequestMSet(r *Request, d Dispatcher) (*Request, error) {
	nblks := len(r.Resp.Array) - 1
	if nblks <= 2 {
		return r, d.Dispatch(r)
	}
	if nblks%2 != 0 {
		r.Response.Resp = redis.NewError([]byte("ERR wrong number of arguments for MSET"))
		return r, nil
	}
	var sub = make([]*Request, nblks/2)
	for i := 0; i < len(sub); i++ {
		sub[i] = &Request{
			Owner: r.Owner,
			OpSeq: -r.OpSeq,
			OpStr: r.OpStr,
			Start: r.Start,
			Wait:  r.Wait,
			Resp: redis.NewArray([]*redis.Resp{
				r.Resp.Array[0],
				r.Resp.Array[i*2+1],
				r.Resp.Array[i*2+2],
			}),
		}
		if err := d.Dispatch(sub[i]); err != nil {
			return nil, err
		}
	}
	r.Coalesce = func() error {
		for _, x := range sub {
			if err := x.Response.Err; err != nil {
				return err
			}
			resp := x.Response.Resp
			if resp == nil {
				return ErrRespIsRequired
			}
			if !resp.IsString() {
				return errors.New(fmt.Sprintf("bad mset resp: %s value.len = %d", resp.Type, len(resp.Value)))
			}
			r.Response.Resp = resp
		}
		return nil
	}
	return r, nil
}

func (s *Session) handleRequestMDel(r *Request, d Dispatcher) (*Request, error) {
	nkeys := len(r.Resp.Array) - 1
	if nkeys <= 1 {
		return r, d.Dispatch(r)
	}
	var sub = make([]*Request, nkeys)
	for i := 0; i < len(sub); i++ {
		sub[i] = &Request{
			Owner: r.Owner,
			OpSeq: -r.OpSeq,
			OpStr: r.OpStr,
			Start: r.Start,
			Wait:  r.Wait,
			Resp: redis.NewArray([]*redis.Resp{
				r.Resp.Array[0],
				r.Resp.Array[i+1],
			}),
		}
		if err := d.Dispatch(sub[i]); err != nil {
			return nil, err
		}
	}
	r.Coalesce = func() error {
		var n int
		for _, x := range sub {
			if err := x.Response.Err; err != nil {
				return err
			}
			resp := x.Response.Resp
			if resp == nil {
				return ErrRespIsRequired
			}
			if !resp.IsInt() || len(resp.Value) != 1 {
				return errors.New(fmt.Sprintf("bad mdel resp: %s value.len = %d", resp.Type, len(resp.Value)))
			}
			if resp.Value[0] != '0' {
				n++
			}
		}
		r.Response.Resp = redis.NewInt([]byte(strconv.Itoa(n)))
		return nil
	}
	return r, nil
}

var sessions struct {
	sid atomic2.Int64
	list.List
	sync.Mutex
}

func init() {
	go func() {
		for {
			time.Sleep(time.Minute)
			lastunix := time.Now().Add(-time.Minute * 45).Unix()
			cleanupSessions(lastunix)
		}
	}()
}

func addToSessions(s *Session) *Session {
	sessions.Lock()
	sessions.PushBack(s)
	sessions.Unlock()
	return s
}

func cleanupSessions(lastunix int64) {
	sessions.Lock()
	for i := sessions.Len(); i != 0; i-- {
		e := sessions.Front()
		s := e.Value.(*Session)
		if s.IsClosed() {
			sessions.Remove(e)
		} else if s.IsTimeout(lastunix) {
			log.Infof("session [%p] killed, due to timeout, sid = %d, ops = %d", s, s.Sid, s.Ops.Get())
			s.Close()
			sessions.Remove(e)
		} else {
			sessions.MoveToBack(e)
		}
	}
	sessions.Unlock()
}

func microseconds() int64 {
	return time.Now().UnixNano() / int64(time.Microsecond)
}
