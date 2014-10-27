package router

import (
	"bufio"
	"net"
	"time"
)

type session struct {
	r *bufio.Reader
	w *bufio.Writer
	net.Conn

	CreateAt time.Time
	Ops      int64
}

//make sure all read using bufio.Reader
func (s *session) Read(p []byte) (int, error) {
	return s.r.Read(p)
}

//write without bufio
func (s *session) Write(p []byte) (int, error) {
	return s.Conn.Write(p)
}
