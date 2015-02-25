// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redispool

import (
	"bufio"
	"net"
)

//not thread-safe
type Conn struct {
	addr string
	net.Conn
	closed bool
	r      *bufio.Reader
}

func (c *Conn) Close() {
	c.Conn.Close()
	c.closed = true
}

func (c *Conn) IsClosed() bool {
	return c.closed
}

type IPool interface {
	Put(conn PoolConnection)
	Get() (PoolConnection, error)
	Open(fact CreateConnectionFunc)
	Close()
}

type PooledConn struct {
	*Conn
	pool IPool
}

func (pc *PooledConn) Recycle() {
	if pc.IsClosed() {
		pc.pool.Put(nil)
	} else {
		pc.pool.Put(pc)
	}
}

//requre read to use bufio
func (pc *PooledConn) Read(p []byte) (int, error) {
	panic("not allowed")
}

func (pc *PooledConn) Write(p []byte) (int, error) {
	return pc.Conn.Write(p)
}

func (pc *PooledConn) BufioReader() *bufio.Reader {
	return pc.r
}

func NewConnection(addr string) (*Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &Conn{
		addr: addr,
		Conn: conn,
		r:    bufio.NewReaderSize(conn, 204800),
	}, nil
}

func ConnectionCreator(addr string) CreateConnectionFunc {
	return func(pool IPool) (PoolConnection, error) {
		c, err := NewConnection(addr)
		if err != nil {
			return nil, err
		}
		return &PooledConn{Conn: c, pool: pool}, nil
	}
}
