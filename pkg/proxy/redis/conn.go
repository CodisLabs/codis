package redis

import (
	"net"
	"time"

	"github.com/wandoulabs/codis/pkg/utils/errors"
)

type Conn struct {
	Sock net.Conn

	ReaderTimeout  time.Duration
	ReaderLastUnix int64

	WriterTimeout  time.Duration
	WriterLastUnix int64

	Reader *Decoder
	Writer *Encoder
}

func DialTimeout(addr string, timeout time.Duration) (*Conn, error) {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return NewConn(c), nil
}

func NewConn(sock net.Conn) *Conn {
	var currtime = time.Now().Unix()
	conn := &Conn{
		Sock: sock,

		ReaderLastUnix: currtime,
		WriterLastUnix: currtime,
	}

	var bufsize = 1024 * 512
	conn.Reader = NewDecoderSize(&connReader{Conn: conn}, bufsize)
	conn.Writer = NewEncoderSize(&connWriter{Conn: conn}, bufsize)
	return conn
}

func (c *Conn) Close() error {
	return c.Sock.Close()
}

func (c *Conn) IsTimeout(lastunix int64) bool {
	return c.ReaderLastUnix < lastunix && c.WriterLastUnix < lastunix
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
	r.ReaderLastUnix = time.Now().Unix()
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
	w.WriterLastUnix = time.Now().Unix()
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
