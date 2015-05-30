package router

import (
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type BackendConn struct {
	Addr string
	stop sync.Once

	input chan *Request
}

func NewBackendConn(addr string) *BackendConn {
	bc := &BackendConn{
		Addr:  addr,
		input: make(chan *Request, 1024),
	}
	go bc.Run()
	return bc
}

func (bc *BackendConn) Run() {
	log.Infof("backend conn [%p] to %s, start service", bc, bc.Addr)
	for k := 0; ; k++ {
		stop, err := bc.loopWriter()
		if stop {
			break
		}
		n := bc.discard(err)
		log.ErrorErrorf(err, "backend conn [%p] to %s, error break[%d], discard all %d requests and restart [%d]", bc, bc.Addr, k, n)
		time.Sleep(time.Millisecond * 50)
	}
	log.Infof("backend conn [%p] to %s, stop and exit", bc, bc.Addr)
}

func (bc *BackendConn) Close() {
	bc.stop.Do(func() {
		close(bc.input)
	})
}

func (bc *BackendConn) PushBack(r *Request) {
	r.wait.Add(1)
	r.slot.jobs.Add(1)
	bc.input <- r
}

func (bc *BackendConn) discard(err error) int {
	var n int
	for i := len(bc.input); i != 0; i-- {
		select {
		case r := <-bc.input:
			if r != nil {
				r.SetResponse(nil, err)
				n++
			}
		default:
		}
	}
	return n
}

func (bc *BackendConn) loopWriter() (bool, error) {
	r, ok := <-bc.input
	if ok {
		c, tasks, err := bc.newBackendReader()
		if err != nil {
			r.SetResponse(nil, err)
			return false, err
		}
		defer func() {
			c.Close()
			close(tasks)
		}()
		for ok {
			flush := len(bc.input) == 0
			if err := c.Writer.Encode(r.Resp, flush); err != nil {
				r.SetResponse(nil, err)
				return false, err
			}
			tasks <- r
			r, ok = <-bc.input
		}
	}
	return true, nil
}

func (bc *BackendConn) newBackendReader() (*redis.Conn, chan<- *Request, error) {
	c, err := redis.DialTimeout(bc.Addr, time.Second)
	if err != nil {
		return nil, nil, err
	}
	c.ReaderTimeout = time.Minute
	c.WriterTimeout = time.Minute

	tasks := make(chan *Request, 1024)
	go func() {
		defer c.Close()
		for r := range tasks {
			r.SetResponse(c.Reader.Decode())
		}
	}()
	return c, tasks, nil
}
