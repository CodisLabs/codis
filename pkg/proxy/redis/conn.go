package redis

import (
	"net"
	"sync/atomic"
	"time"

	"github.com/wandoulabs/codis/pkg/utils/errors"
)

type Conn struct {
	Sock net.Conn

	ReaderTimeout time.Duration
	WriterTimeout time.Duration

	readerLastUnix int64
	writerLastUnix int64
	closed         int64

	Reader *Decoder
	Writer *Encoder
}

func DialTimeout(addr string, bufsize int, timeout time.Duration) (*Conn, error) {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return NewConnSize(c, bufsize), nil
}

func NewConn(sock net.Conn) *Conn {
	return NewConnSize(sock, 1024*64)
}

func NewConnSize(sock net.Conn, bufsize int) *Conn {
	unixtime := time.Now().Unix()
	conn := &Conn{Sock: sock}
	conn.readerLastUnix = unixtime
	conn.writerLastUnix = unixtime
	conn.Reader = NewDecoderSize(&connReader{Conn: conn}, bufsize)
	conn.Writer = NewEncoderSize(&connWriter{Conn: conn}, bufsize)
	return conn
}

func (c *Conn) Close() error {
	atomic.StoreInt64(&c.closed, 1)
	return c.Sock.Close()
}

func (c *Conn) IsClosed() bool {
	return atomic.LoadInt64(&c.closed) != 0
}

func (c *Conn) ReaderLastUnix() int64 {
	return atomic.LoadInt64(&c.readerLastUnix)
}

func (c *Conn) WriterLastUnix() int64 {
	return atomic.LoadInt64(&c.writerLastUnix)
}

func (c *Conn) IsTimeout(lastunix int64) bool {
	return c.ReaderLastUnix() < lastunix && c.WriterLastUnix() < lastunix
}

type connReader struct {
	*Conn
	hasDeadline bool
}

func (r *connReader) Read(b []byte) (int, error) {
	if timeout := r.ReaderTimeout; timeout != 0 {
		if err := r.Sock.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return 0, errors.Trace(err)
		}
		r.hasDeadline = true
	} else if r.hasDeadline {
		if err := r.Sock.SetReadDeadline(time.Time{}); err != nil {
			return 0, errors.Trace(err)
		}
		r.hasDeadline = false
	}
	n, err := r.Sock.Read(b)
	if err != nil {
		err = errors.Trace(err)
	}
	atomic.StoreInt64(&r.readerLastUnix, time.Now().Unix())
	return n, err
}

type connWriter struct {
	*Conn
	hasDeadline bool
}

func (w *connWriter) Write(b []byte) (int, error) {
	if timeout := w.WriterTimeout; timeout != 0 {
		if err := w.Sock.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
			return 0, errors.Trace(err)
		}
		w.hasDeadline = true
	} else if w.hasDeadline {
		if err := w.Sock.SetWriteDeadline(time.Time{}); err != nil {
			return 0, errors.Trace(err)
		}
		w.hasDeadline = false
	}
	n, err := w.Sock.Write(b)
	if err != nil {
		err = errors.Trace(err)
	}
	atomic.StoreInt64(&w.writerLastUnix, time.Now().Unix())
	return n, err
}

func IsTimeout(err error) bool {
	if err := errors.Cause(err); err != nil {
		e, ok := err.(*net.OpError)
		if ok {
			return e.Timeout()
		}
	}
	return false
}
