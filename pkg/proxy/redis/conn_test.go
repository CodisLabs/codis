// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"net"
	"testing"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func newConnPair() (*Conn, *Conn) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.MustNoError(err)
	defer l.Close()

	const bufsize = 128 * 1024

	cc := make(chan *Conn, 1)
	go func() {
		defer close(cc)
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			cc <- NewConnSize(c, bufsize)
		}
	}()

	conn1, err := DialTimeout(l.Addr().String(), bufsize, time.Millisecond*50)
	assert.MustNoError(err)

	conn2, ok := <-cc
	assert.Must(ok)
	return conn1, conn2
}

func benchmarkConn(b *testing.B, n int) {
	for i := 0; i < b.N; i++ {
		c := NewConnSize(&net.TCPConn{}, n)
		c.Close()
	}
}

func BenchmarkConn16K(b *testing.B)  { benchmarkConn(b, 1024*16) }
func BenchmarkConn32K(b *testing.B)  { benchmarkConn(b, 1024*32) }
func BenchmarkConn64K(b *testing.B)  { benchmarkConn(b, 1024*64) }
func BenchmarkConn128K(b *testing.B) { benchmarkConn(b, 1024*128) }
func BenchmarkConn256K(b *testing.B) { benchmarkConn(b, 1024*256) }
