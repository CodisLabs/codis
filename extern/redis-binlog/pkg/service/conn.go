// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/binlog"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

type conn struct {
	r *bufio.Reader
	w *bufio.Writer

	db uint32
	nc net.Conn
	bl *binlog.Binlog

	summ    string
	timeout time.Duration
}

func newConn(nc net.Conn, bl *binlog.Binlog, timeout int) *conn {
	c := &conn{
		nc: nc,
		bl: bl,
	}
	c.r = bufio.NewReader(nc)
	c.w = bufio.NewWriter(nc)
	c.summ = fmt.Sprintf("<local> %s -- %s <remote>", nc.LocalAddr(), nc.RemoteAddr())
	c.timeout = time.Duration(timeout) * time.Second
	return c
}

func (c *conn) serve(h *Handler) error {
	for {
		if c.timeout != 0 {
			deadline := time.Now().Add(c.timeout)
			if err := c.nc.SetReadDeadline(deadline); err != nil {
				return errors.Trace(err)
			}
		}
		request, err := redis.Decode(c.r)
		if err != nil {
			return err
		}
		h.counters.commands.Add(1)
		response, err := c.dispatch(h, request)
		if err != nil {
			h.counters.commandsFailed.Add(1)
			b, _ := redis.EncodeToBytes(request)
			log.WarnErrorf(err, "handle commands failed, conn = %s, request = '%s'", c.summ, base64.StdEncoding.EncodeToString(b))
		}
		if response == nil {
			continue
		}
		if c.timeout != 0 {
			deadline := time.Now().Add(c.timeout)
			if err := c.nc.SetWriteDeadline(deadline); err != nil {
				return errors.Trace(err)
			}
		}
		if err := redis.Encode(c.w, response); err != nil {
			return err
		}
		if err := errors.Trace(c.w.Flush()); err != nil {
			return err
		}
	}
}

func (c *conn) dispatch(h *Handler, request redis.Resp) (redis.Resp, error) {
	cmd, args, err := redis.ParseArgs(request)
	if err != nil {
		return toRespError(err)
	}
	if f := h.htable[cmd]; f == nil {
		return toRespErrorf("unknown command %s", cmd)
	} else {
		return f(c, args...)
	}
}

func (c *conn) ping() error {
	deadline := time.Now().Add(time.Second * 5)
	if err := c.nc.SetDeadline(deadline); err != nil {
		return errors.Trace(err)
	}
	if _, err := c.w.WriteString("*1\r\n$4\r\nping\r\n"); err != nil {
		return errors.Trace(err)
	}
	if err := c.w.Flush(); err != nil {
		return errors.Trace(err)
	}
	var rsp string
	for !strings.HasSuffix(rsp, "\r\n") {
		b := []byte{0}
		if _, err := c.r.Read(b); err != nil {
			return errors.Trace(err)
		}
		if len(rsp) == 0 && b[0] == '\n' {
			continue
		}
		rsp += string(b)
	}
	rsp = rsp[:len(rsp)-2]

	if strings.ToLower(rsp) != "+pong" {
		return errors.Errorf("invalid response of command ping: %s", rsp)
	} else {
		return nil
	}
}

func (c *conn) presync() (int64, error) {
	deadline := time.Now().Add(time.Second * 15)
	if err := c.nc.SetDeadline(deadline); err != nil {
		return 0, errors.Trace(err)
	}
	if _, err := c.w.WriteString("*1\r\n$4\r\nsync\r\n"); err != nil {
		return 0, errors.Trace(err)
	}
	if err := c.w.Flush(); err != nil {
		return 0, errors.Trace(err)
	}
	var rsp string
	for !strings.HasSuffix(rsp, "\r\n") {
		deadline := time.Now().Add(time.Second * 15)
		if err := c.nc.SetDeadline(deadline); err != nil {
			return 0, errors.Trace(err)
		}
		b := []byte{0}
		if _, err := c.r.Read(b); err != nil {
			return 0, errors.Trace(err)
		}
		if len(rsp) == 0 && b[0] == '\n' {
			continue
		}
		rsp += string(b)
	}
	rsp = rsp[:len(rsp)-2]

	if rsp[0] != '$' {
		return 0, errors.Errorf("invalid sync response, rsp = '%s'", rsp)
	}

	n, err := strconv.Atoi(rsp[1:])
	if err != nil || n <= 0 {
		return 0, errors.Errorf("invalid sync response = '%s', error = '%s', n = %d", rsp, err, n)
	}
	return int64(n), nil
}

func (c *conn) Close() {
	c.nc.Close()
}

func (c *conn) DB() uint32 {
	return c.db
}

func (c *conn) SetDB(db uint32) {
	c.db = db
}

func (c *conn) Binlog() *binlog.Binlog {
	return c.bl
}
