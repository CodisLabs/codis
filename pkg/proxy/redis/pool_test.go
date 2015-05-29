package redis

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/wandoulabs/codis/pkg/utils/assert"
)

func TestPool(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.MustNoError(err)
	defer l.Close()

	go func() {
		var cc []net.Conn
		for {
			c, err := l.Accept()
			if err != nil {
				for _, c := range cc {
					c.Close()
				}
				return
			}
			cc = append(cc, c)
		}
	}()

	const n = 4

	addr := l.Addr().String()
	var cc []*Conn
	for i := 0; i < n*2; i++ {
		c, err := getPoolConn(addr)
		assert.MustNoError(err)
		cc = append(cc, c)
	}
	for i, c := range cc {
		if i%2 == 0 {
			c.Reader.Err = io.EOF
		}
		putPoolConn(c, addr)
	}
	assert.Must(connPool.Len() == n)

	cc = cc[:0]
	for i := 0; i < n; i++ {
		c, err := getPoolConn(addr)
		assert.MustNoError(err)
		cc = append(cc, c)
	}
	for i := 0; i < n; i++ {
		c, err := getPoolConn(addr)
		assert.MustNoError(err)
		cc = append(cc, c)
	}
	for _, c := range cc {
		putPoolConn(c, addr)
	}
	assert.Must(connPool.Len() == n*2)

	cleanupPool(time.Now().Unix() - 10)
	assert.Must(connPool.Len() == n*2)
	cleanupPool(time.Now().Unix() + 10)
	assert.Must(connPool.Len() == 0)
}
