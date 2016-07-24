// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"net"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"
)

type Conn struct {
	Sock net.Conn

	*Decoder
	*Encoder

	ReaderTimeout time.Duration
	WriterTimeout time.Duration

	LastWrite time.Time
}

func DialTimeout(addr string, bufsize int, timeout time.Duration) (*Conn, error) {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return NewConnSize(c, bufsize), nil
}

func NewConn(sock net.Conn) *Conn {
	return NewConnSize(sock, 8192)
}

func NewConnSize(sock net.Conn, bufsize int) *Conn {
	conn := &Conn{Sock: sock}
	conn.Decoder = NewDecoderSize(&connReader{Conn: conn}, bufsize)
	conn.Encoder = NewEncoderSize(&connWriter{Conn: conn}, bufsize)
	return conn
}

func (c *Conn) LocalAddr() string {
	return c.Sock.LocalAddr().String()
}

func (c *Conn) RemoteAddr() string {
	return c.Sock.RemoteAddr().String()
}

func (c *Conn) Close() error {
	return c.Sock.Close()
}

func (c *Conn) SetKeepAlive(keepalive bool) error {
	if t, ok := c.Sock.(*net.TCPConn); ok {
		if err := t.SetKeepAlive(keepalive); err != nil {
			return errors.Trace(err)
		}
		return nil
	}
	return errors.Errorf("not tcp connection")
}

func (c *Conn) SetKeepAlivePeriod(d time.Duration) error {
	if t, ok := c.Sock.(*net.TCPConn); ok {
		if err := t.SetKeepAlivePeriod(d); err != nil {
			return errors.Trace(err)
		}
		return nil
	}
	return errors.Errorf("not tcp connection")
}

func (c *Conn) FlushPolicy(maxBuffered int, maxInterval time.Duration) *FlushPolicy {
	p := &FlushPolicy{Conn: c}
	p.MaxBuffered = maxBuffered
	p.MaxInterval = maxInterval
	return p
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
	w.LastWrite = time.Now()
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

type FlushPolicy struct {
	Conn *Conn

	MaxBuffered int
	MaxInterval time.Duration

	nbuffered int
}

func (p *FlushPolicy) NeedFlush() bool {
	if p.nbuffered != 0 {
		if p.MaxBuffered < p.nbuffered {
			return true
		}
		if p.MaxInterval < time.Since(p.Conn.LastWrite) {
			return true
		}
	}
	return false
}

func (p *FlushPolicy) Flush(force bool) error {
	if force || p.NeedFlush() {
		if err := p.Conn.Flush(); err != nil {
			return err
		}
		p.nbuffered = 0
	}
	return nil
}

func (p *FlushPolicy) Encode(resp *Resp) error {
	if err := p.Conn.Encode(resp, false); err != nil {
		return err
	} else {
		p.nbuffered++
		return nil
	}
}

func (p *FlushPolicy) EncodeMultiBulk(multi []*Resp) error {
	if err := p.Conn.EncodeMultiBulk(multi, false); err != nil {
		return err
	} else {
		p.nbuffered++
		return nil
	}
}
