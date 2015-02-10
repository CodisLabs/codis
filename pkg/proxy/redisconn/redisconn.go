package redisconn

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
	w      *bufio.Writer
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
		w:    bufio.NewWriterSize(conn, 204800),
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
	return c.w.Write(p)
}

func (c *Conn) BufioReader() *bufio.Reader {
	return c.r
}
