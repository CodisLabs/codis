// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"io"
	"net"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/binlog"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

func Serve(config *Config, bl *binlog.Binlog) error {
	h := &Handler{
		config: config,
		master: make(chan *conn, 0),
		signal: make(chan int, 0),
	}
	defer func() {
		close(h.signal)
	}()

	l, err := net.Listen("tcp", config.Listen)
	if err != nil {
		return errors.Trace(err)
	}
	defer l.Close()

	if h.htable, err = redis.NewHandlerTable(h); err != nil {
		return err
	} else {
		go h.daemonSyncMaster()
	}

	log.Infof("open listen address '%s' and start service", l.Addr())

	for {
		if nc, err := l.Accept(); err != nil {
			return errors.Trace(err)
		} else {
			h.counters.clientsAccepted.Add(1)
			go func() {
				h.counters.clients.Add(1)
				defer h.counters.clients.Sub(1)
				c := newConn(nc, bl, h.config.ConnTimeout)
				defer c.Close()
				log.Infof("new connection: %s", c.summ)
				if err := c.serve(h); err != nil {
					if errors.Equal(err, io.EOF) {
						log.Infof("connection lost: %s [io.EOF]", c.summ)
					} else {
						log.InfoErrorf(err, "connection lost: %s", c.summ)
					}
				} else {
					log.Infof("connection exit: %s", c.summ)
				}
			}()
		}
	}
}

type Session interface {
	DB() uint32
	SetDB(db uint32)
	Binlog() *binlog.Binlog
}

type Handler struct {
	config *Config
	htable redis.HandlerTable

	syncto       string
	syncto_since int64

	master chan *conn
	signal chan int

	counters struct {
		bgsave          atomic2.Int64
		clients         atomic2.Int64
		clientsAccepted atomic2.Int64
		commands        atomic2.Int64
		commandsFailed  atomic2.Int64
		syncRdbRemains  atomic2.Int64
		syncCacheBytes  atomic2.Int64
		syncTotalBytes  atomic2.Int64
	}
}

func toRespError(err error) (redis.Resp, error) {
	return redis.NewError(err), err
}

func toRespErrorf(format string, args ...interface{}) (redis.Resp, error) {
	err := errors.Errorf(format, args...)
	return toRespError(err)
}

func session(arg0 interface{}, args [][]byte) (Session, error) {
	s, _ := arg0.(Session)
	if s == nil {
		return nil, errors.New("invalid session")
	}
	for i, v := range args {
		if len(v) == 0 {
			return nil, errors.Errorf("args[%d] is nil", i)
		}
	}
	return s, nil
}

func iconvert(args [][]byte) []interface{} {
	iargs := make([]interface{}, len(args))
	for i, v := range args {
		iargs[i] = v
	}
	return iargs
}
