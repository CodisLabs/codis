// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"bufio"
	"errors"
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
	return 0, errors.New("not implemented")
}

//write without bufio
func (s *session) Write(p []byte) (int, error) {
	return s.Conn.Write(p)
}
