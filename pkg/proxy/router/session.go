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

	Sid    int64
	Seq    int64
	Quit   bool
	Closed bool

	CreateUnix int64
}

func (s *Session) String() string {
	o := &struct {
		Sid          int64
		Seq          int64
		Quit, Closed bool
		CreateUnix   int64
		RemoteAddr   string
	}{
		s.Sid, s.Seq, s.Quit, s.Closed, s.CreateUnix,
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
	return addToSessions(s)
}

func (s *Session) Close() {
	s.Closed = true
	s.Conn.Close()
}

func (s *Session) Serve(d Dispatcher) {
	var errlist errors.ErrorList
	defer func() {
		if err := errlist.First(); err != nil {
			log.Infof("session [%p] closed, session = %s, error = %s", s, s, err)
		} else {
			log.Infof("session [%p] closed, session = %s, quit", s, s)
		}
	}()

	tasks := make(chan *Request, 256)
	go func() {
		defer func() {
			s.Close()
			for _ = range tasks {
			}
		}()
		if err := s.loopWriter(tasks); err != nil {
			errlist.PushBack(err)
		}
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
	for !s.Quit {
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
	var lastflush time.Time
	for r := range tasks {
		resp, err := s.handleResponse(r)
		if err != nil {
			return err
		}

		var flush bool
		if len(tasks) == 0 {
			flush = true
		} else if time.Since(lastflush) >= time.Millisecond*100 {
			flush = true
		}

		if err := s.Writer.Encode(resp, flush); err != nil {
			return err
		}

		if flush {
			lastflush = time.Now()
		}
	}
	return nil
}

var ErrRespIsRequired = errors.New("resp is required")

func (s *Session) handleResponse(r *Request) (*redis.Resp, error) {
	r.Wait()
	if r.Callback != nil {
		if err := r.Callback(); err != nil {
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

	s.Seq++
	r := &Request{
		Sid:   s.Sid,
		Seq:   s.Seq,
		OpStr: opstr,
		Resp:  resp,
		wait:  &sync.WaitGroup{},
	}

	switch opstr {
	case "QUIT":
		s.Quit = true
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
			Sid:   -r.Sid,
			Seq:   -r.Seq,
			OpStr: r.OpStr,
			Resp: redis.NewArray([]*redis.Resp{
				r.Resp.Array[0],
				r.Resp.Array[i+1],
			}),
			wait: r.wait,
		}
		if err := d.Dispatch(sub[i]); err != nil {
			return nil, err
		}
	}
	r.Callback = func() error {
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
	if nblks <= 1 {
		return r, d.Dispatch(r)
	}
	if nblks%2 != 0 {
		r.Response.Resp = redis.NewError([]byte("ERR wrong number of arguments for MSET"))
		return r, nil
	}
	var sub = make([]*Request, nblks/2)
	for i := 0; i < len(sub); i++ {
		sub[i] = &Request{
			Sid:   -r.Sid,
			Seq:   -r.Seq,
			OpStr: r.OpStr,
			Resp: redis.NewArray([]*redis.Resp{
				r.Resp.Array[0],
				r.Resp.Array[i*2+1],
				r.Resp.Array[i*2+2],
			}),
			wait: r.wait,
		}
		if err := d.Dispatch(sub[i]); err != nil {
			return nil, err
		}
	}
	r.Callback = func() error {
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
			Sid:   -r.Sid,
			Seq:   -r.Seq,
			OpStr: r.OpStr,
			Resp: redis.NewArray([]*redis.Resp{
				r.Resp.Array[0],
				r.Resp.Array[i+1],
			}),
			wait: r.wait,
		}
		if err := d.Dispatch(sub[i]); err != nil {
			return nil, err
		}
	}
	r.Callback = func() error {
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
	log.Infof("session [%p] created, sid = %d", s, s.Sid)
	return s
}

func cleanupSessions(lastunix int64) {
	sessions.Lock()
	for i := sessions.Len(); i != 0; i-- {
		e := sessions.Front()
		s := e.Value.(*Session)
		if s.Closed {
			sessions.Remove(e)
		} else if s.IsTimeout(lastunix) {
			log.Infof("session [%p] killed, due to timeout, sid = %d", s, s.Sid)
			s.Close()
			sessions.Remove(e)
		} else {
			sessions.MoveToBack(e)
		}
	}
	sessions.Unlock()
}
