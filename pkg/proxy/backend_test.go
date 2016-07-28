// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func TestBackend(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.MustNoError(err)
	defer l.Close()

	addr := l.Addr().String()
	reqc := make(chan *Request, 16384)
	go func() {
		bc := NewBackendConn(addr, NewDefaultConfig())
		defer bc.Close()
		defer close(reqc)
		var multi = []*redis.Resp{redis.NewBulkBytes(make([]byte, 4096))}
		for i := 0; i < cap(reqc); i++ {
			r := &Request{}
			r.Multi = multi
			r.Batch = &sync.WaitGroup{}
			bc.PushBack(r)
			reqc <- r
		}
	}()

	const bufsize = 8192

	go func() {
		c, err := l.Accept()
		assert.MustNoError(err)
		defer c.Close()
		conn := redis.NewConn(c, bufsize, bufsize)
		time.Sleep(time.Millisecond * 300)
		for i := 0; i < cap(reqc); i++ {
			_, err := conn.Decode()
			assert.MustNoError(err)
			resp := redis.NewString([]byte(strconv.Itoa(i)))
			assert.MustNoError(conn.Encode(resp, true))
		}
	}()

	var n int
	for r := range reqc {
		r.Batch.Wait()
		assert.Must(string(r.Response.Resp.Value) == strconv.Itoa(n))
		n++
	}
	assert.Must(n == cap(reqc))
}
