// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/math2"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
)

const (
	stateConnected = iota + 1
	stateDataStale
)

type BackendConn struct {
	stop sync.Once
	addr string

	input chan *Request
	retry struct {
		fails int
		delay Delay
	}
	state atomic2.Int64

	closed atomic2.Bool
	config *Config

	database int
}

func NewBackendConn(addr string, database int, config *Config) *BackendConn {
	bc := &BackendConn{
		addr: addr, config: config, database: database,
	}
	bc.input = make(chan *Request, 1024)
	bc.retry.delay = &DelayExp2{
		Min: 50, Max: 5000,
		Unit: time.Millisecond,
	}

	go bc.run()

	return bc
}

func (bc *BackendConn) Addr() string {
	return bc.addr
}

func (bc *BackendConn) Close() {
	bc.stop.Do(func() {
		close(bc.input)
	})
	bc.closed.Set(true)
}

func (bc *BackendConn) IsConnected() bool {
	return bc.state.Int64() == stateConnected
}

func (bc *BackendConn) PushBack(r *Request) {
	if r.Batch != nil {
		r.Batch.Add(1)
	}
	bc.input <- r
}

func (bc *BackendConn) KeepAlive() bool {
	if len(bc.input) != 0 {
		return false
	}
	switch bc.state.Int64() {
	default:
		m := &Request{}
		m.Multi = []*redis.Resp{
			redis.NewBulkBytes([]byte("PING")),
		}
		bc.PushBack(m)

	case stateDataStale:
		m := &Request{}
		m.Multi = []*redis.Resp{
			redis.NewBulkBytes([]byte("INFO")),
		}
		m.Batch = &sync.WaitGroup{}
		bc.PushBack(m)

		keepAliveCallback <- func() {
			m.Batch.Wait()
			var err = func() error {
				if err := m.Err; err != nil {
					return err
				}
				switch resp := m.Resp; {
				case resp == nil:
					return ErrRespIsRequired
				case resp.IsError():
					return fmt.Errorf("bad info resp: %s", resp.Value)
				case resp.IsBulkBytes():
					var info = make(map[string]string)
					for _, line := range strings.Split(string(resp.Value), "\n") {
						kv := strings.SplitN(line, ":", 2)
						if len(kv) != 2 {
							continue
						}
						if key := strings.TrimSpace(kv[0]); key != "" {
							info[key] = strings.TrimSpace(kv[1])
						}
					}
					if info["master_link_status"] == "down" {
						return nil
					}
					if info["loading"] == "1" {
						return nil
					}
					if bc.state.CompareAndSwap(stateDataStale, stateConnected) {
						log.Warnf("backend conn [%p] to %s, db-%d state = Connected (keepalive)",
							bc, bc.addr, bc.database)
					}
					return nil
				default:
					return fmt.Errorf("bad info resp: should be string, but got %s", resp.Type)
				}
			}()
			if err != nil && bc.closed.IsFalse() {
				log.WarnErrorf(err, "backend conn [%p] to %s, db-%d recover from DataStale failed",
					bc, bc.addr, bc.database)
			}
		}
	}
	return true
}

var keepAliveCallback = make(chan func(), 128)

func init() {
	go func() {
		for fn := range keepAliveCallback {
			fn()
		}
	}()
}

func (bc *BackendConn) newBackendReader(round int, config *Config) (*redis.Conn, chan<- *Request, error) {
	c, err := redis.DialTimeout(bc.addr, time.Second*5,
		config.BackendRecvBufsize.AsInt(),
		config.BackendSendBufsize.AsInt())
	if err != nil {
		return nil, nil, err
	}
	c.ReaderTimeout = config.BackendRecvTimeout.Duration()
	c.WriterTimeout = config.BackendSendTimeout.Duration()
	c.SetKeepAlivePeriod(config.BackendKeepAlivePeriod.Duration())

	if err := bc.verifyAuth(c, config.ProductAuth); err != nil {
		c.Close()
		return nil, nil, err
	}
	if err := bc.selectDatabase(c, bc.database); err != nil {
		c.Close()
		return nil, nil, err
	}

	tasks := make(chan *Request, config.BackendMaxPipeline)
	go bc.loopReader(tasks, c, round)

	return c, tasks, nil
}

func (bc *BackendConn) verifyAuth(c *redis.Conn, auth string) error {
	if auth == "" {
		return nil
	}

	multi := []*redis.Resp{
		redis.NewBulkBytes([]byte("AUTH")),
		redis.NewBulkBytes([]byte(auth)),
	}

	if err := c.EncodeMultiBulk(multi, true); err != nil {
		return err
	}

	resp, err := c.Decode()
	switch {
	case err != nil:
		return err
	case resp == nil:
		return ErrRespIsRequired
	case resp.IsError():
		return fmt.Errorf("error resp: %s", resp.Value)
	case resp.IsString():
		return nil
	default:
		return fmt.Errorf("error resp: should be string, but got %s", resp.Type)
	}
}

func (bc *BackendConn) selectDatabase(c *redis.Conn, database int) error {
	if database == 0 {
		return nil
	}

	multi := []*redis.Resp{
		redis.NewBulkBytes([]byte("SELECT")),
		redis.NewBulkBytes([]byte(strconv.Itoa(database))),
	}

	if err := c.EncodeMultiBulk(multi, true); err != nil {
		return err
	}

	resp, err := c.Decode()
	switch {
	case err != nil:
		return err
	case resp == nil:
		return ErrRespIsRequired
	case resp.IsError():
		return fmt.Errorf("error resp: %s", resp.Value)
	case resp.IsString():
		return nil
	default:
		return fmt.Errorf("error resp: should be string, but got %s", resp.Type)
	}
}

func (bc *BackendConn) setResponse(r *Request, resp *redis.Resp, err error) error {
	r.Resp, r.Err = resp, err
	if r.Group != nil {
		r.Group.Done()
	}
	if r.Batch != nil {
		r.Batch.Done()
	}
	return err
}

var (
	ErrBackendConnReset = errors.New("backend conn reset")
	ErrRequestIsBroken  = errors.New("request is broken")
)

func (bc *BackendConn) run() {
	log.Warnf("backend conn [%p] to %s, db-%d start service",
		bc, bc.addr, bc.database)
	for round := 0; bc.closed.IsFalse(); round++ {
		log.Warnf("backend conn [%p] to %s, db-%d round-[%d]",
			bc, bc.addr, bc.database, round)
		if err := bc.loopWriter(round); err != nil {
			bc.delayBeforeRetry()
		}
	}
	log.Warnf("backend conn [%p] to %s, db-%d stop and exit",
		bc, bc.addr, bc.database)
}

var (
	errRespMasterDown = []byte("MASTERDOWN")
	errRespLoading    = []byte("LOADING")
)

func (bc *BackendConn) loopReader(tasks <-chan *Request, c *redis.Conn, round int) (err error) {
	defer func() {
		c.Close()
		for r := range tasks {
			bc.setResponse(r, nil, ErrBackendConnReset)
		}
		log.WarnErrorf(err, "backend conn [%p] to %s, db-%d reader-[%d] exit",
			bc, bc.addr, bc.database, round)
	}()
	for r := range tasks {
		resp, err := c.Decode()
		if err != nil {
			return bc.setResponse(r, nil, fmt.Errorf("backend conn failure, %s", err))
		}
		if resp != nil && resp.IsError() {
			switch {
			case bytes.HasPrefix(resp.Value, errRespMasterDown):
				if bc.state.CompareAndSwap(stateConnected, stateDataStale) {
					log.Warnf("backend conn [%p] to %s, db-%d state = DataStale, caused by 'MASTERDOWN'",
						bc, bc.addr, bc.database)
				}
			case bytes.HasPrefix(resp.Value, errRespLoading):
				if bc.state.CompareAndSwap(stateConnected, stateDataStale) {
					log.Warnf("backend conn [%p] to %s, db-%d state = DataStale, caused by 'LOADING'",
						bc, bc.addr, bc.database)
				}
			}
		}
		bc.setResponse(r, resp, nil)
	}
	return nil
}

func (bc *BackendConn) delayBeforeRetry() {
	bc.retry.fails += 1
	if bc.retry.fails <= 10 {
		return
	}
	timeout := bc.retry.delay.After()
	for bc.closed.IsFalse() {
		select {
		case <-timeout:
			return
		case r, ok := <-bc.input:
			if !ok {
				return
			}
			bc.setResponse(r, nil, ErrBackendConnReset)
		}
	}
}

func (bc *BackendConn) loopWriter(round int) (err error) {
	defer func() {
		for i := len(bc.input); i != 0; i-- {
			r := <-bc.input
			bc.setResponse(r, nil, ErrBackendConnReset)
		}
		log.WarnErrorf(err, "backend conn [%p] to %s, db-%d writer-[%d] exit",
			bc, bc.addr, bc.database, round)
	}()
	c, tasks, err := bc.newBackendReader(round, bc.config)
	if err != nil {
		return err
	}
	defer close(tasks)

	defer bc.state.Set(0)

	bc.state.Set(stateConnected)
	bc.retry.fails = 0
	bc.retry.delay.Reset()

	p := c.FlushEncoder()
	p.MaxInterval = time.Millisecond
	p.MaxBuffered = cap(tasks) / 2

	for r := range bc.input {
		if r.IsReadOnly() && r.IsBroken() {
			bc.setResponse(r, nil, ErrRequestIsBroken)
			continue
		}
		if err := p.EncodeMultiBulk(r.Multi); err != nil {
			return bc.setResponse(r, nil, fmt.Errorf("backend conn failure, %s", err))
		}
		if err := p.Flush(len(bc.input) == 0); err != nil {
			return bc.setResponse(r, nil, fmt.Errorf("backend conn failure, %s", err))
		} else {
			tasks <- r
		}
	}
	return nil
}

type sharedBackendConn struct {
	addr string
	host []byte
	port []byte

	owner *sharedBackendConnPool
	conns [][]*BackendConn

	single []*BackendConn

	refcnt int
}

func newSharedBackendConn(addr string, pool *sharedBackendConnPool) *sharedBackendConn {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.ErrorErrorf(err, "split host-port failed, address = %s", addr)
	}
	s := &sharedBackendConn{
		addr: addr,
		host: []byte(host), port: []byte(port),
	}
	s.owner = pool
	s.conns = make([][]*BackendConn, pool.config.BackendNumberDatabases)
	for database := range s.conns {
		parallel := make([]*BackendConn, pool.parallel)
		for i := range parallel {
			parallel[i] = NewBackendConn(addr, database, pool.config)
		}
		s.conns[database] = parallel
	}
	if pool.parallel == 1 {
		s.single = make([]*BackendConn, len(s.conns))
		for database := range s.conns {
			s.single[database] = s.conns[database][0]
		}
	}
	s.refcnt = 1
	return s
}

func (s *sharedBackendConn) Addr() string {
	if s == nil {
		return ""
	}
	return s.addr
}

func (s *sharedBackendConn) Release() {
	if s == nil {
		return
	}
	if s.refcnt <= 0 {
		log.Panicf("shared backend conn has been closed, close too many times")
	} else {
		s.refcnt--
	}
	if s.refcnt != 0 {
		return
	}
	for _, parallel := range s.conns {
		for _, bc := range parallel {
			bc.Close()
		}
	}
	delete(s.owner.pool, s.addr)
}

func (s *sharedBackendConn) Retain() *sharedBackendConn {
	if s == nil {
		return nil
	}
	if s.refcnt <= 0 {
		log.Panicf("shared backend conn has been closed")
	} else {
		s.refcnt++
	}
	return s
}

func (s *sharedBackendConn) KeepAlive() {
	if s == nil {
		return
	}
	for _, parallel := range s.conns {
		for _, bc := range parallel {
			bc.KeepAlive()
		}
	}
}

func (s *sharedBackendConn) BackendConn(database int32, seed uint, must bool) *BackendConn {
	if s == nil {
		return nil
	}

	if s.single != nil {
		bc := s.single[database]
		if must || bc.IsConnected() {
			return bc
		}
		return nil
	}

	var parallel = s.conns[database]

	var i = seed
	for range parallel {
		i = (i + 1) % uint(len(parallel))
		if bc := parallel[i]; bc.IsConnected() {
			return bc
		}
	}
	if !must {
		return nil
	}
	return parallel[0]
}

type sharedBackendConnPool struct {
	config   *Config
	parallel int

	pool map[string]*sharedBackendConn
}

func newSharedBackendConnPool(config *Config, parallel int) *sharedBackendConnPool {
	p := &sharedBackendConnPool{
		config: config, parallel: math2.MaxInt(1, parallel),
	}
	p.pool = make(map[string]*sharedBackendConn)
	return p
}

func (p *sharedBackendConnPool) KeepAlive() {
	for _, bc := range p.pool {
		bc.KeepAlive()
	}
}

func (p *sharedBackendConnPool) Get(addr string) *sharedBackendConn {
	return p.pool[addr]
}

func (p *sharedBackendConnPool) Retain(addr string) *sharedBackendConn {
	if bc := p.pool[addr]; bc != nil {
		return bc.Retain()
	} else {
		bc = newSharedBackendConn(addr, p)
		p.pool[addr] = bc
		return bc
	}
}
