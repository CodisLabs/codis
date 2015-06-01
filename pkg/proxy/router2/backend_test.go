package router

import (
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
	"github.com/wandoulabs/codis/pkg/utils/assert"
)

func TestBackend(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.MustNoError(err)
	defer l.Close()

	addr := l.Addr().String()
	reqc := make(chan *Request, 16384)
	go func() {
		bc := NewBackendConn(addr)
		defer bc.Close()
		defer close(reqc)
		var resp = redis.NewBulkBytes(make([]byte, 4096))
		for i := 0; i < cap(reqc); i++ {
			r := &Request{
				Flush: true,
				Resp:  resp,
				wait:  &sync.WaitGroup{},
			}
			r.wait.Add(1)
			bc.PushBack(r)
			reqc <- r
		}
	}()

	go func() {
		c, err := l.Accept()
		assert.MustNoError(err)
		defer c.Close()
		conn := redis.NewConn(c)
		time.Sleep(time.Millisecond * 300)
		for i := 0; i < cap(reqc); i++ {
			_, err := conn.Reader.Decode()
			assert.MustNoError(err)
			resp := redis.NewString([]byte(strconv.Itoa(i)))
			assert.MustNoError(conn.Writer.Encode(resp, true))
		}
	}()

	var n int
	for r := range reqc {
		r.Wait()
		assert.Must(string(r.Response.Resp.Value) == strconv.Itoa(n))
		n++
	}
	assert.Must(n == cap(reqc))
}
