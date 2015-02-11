package redisconn

import (
	"bufio"
	"net"
	"time"
)

//not thread-safe
type Conn struct {
	addr string
	net.Conn
	closed     bool
	r          *bufio.Reader
	w          *bufio.Writer
	netTimeout int //second
}

func NewConnection(addr string, netTimeout int) (*Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, time.Duration(netTimeout)*time.Second)
	if err != nil {
		return nil, err
	}

	return &Conn{
		addr:       addr,
		Conn:       conn,
		r:          bufio.NewReaderSize(conn, 204800),
		w:          bufio.NewWriterSize(conn, 204800),
		netTimeout: netTimeout,
	}, nil
}

//requre read to use bufio
func (c *Conn) Read(p []byte) (int, error) {
	panic("not allowed")
}

func (c *Conn) Flush() error {
	return c.w.Flush()
}

func (c *Conn) Write(p []byte) (int, error) {
	if c.w.Available() < len(p) {
		c.Conn.SetWriteDeadline(time.Now().Add(time.Duration(c.netTimeout) * time.Second))
	}

	return c.w.Write(p)
}

func (c *Conn) BufioReader() *bufio.Reader {
	return c.r
}
