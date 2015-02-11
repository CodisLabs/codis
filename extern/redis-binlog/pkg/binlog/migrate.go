// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"bufio"
	"container/list"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

var poolmap struct {
	m map[string]*list.List
	sync.Mutex
}

type conn struct {
	summ string
	sock net.Conn
	last time.Time

	err error

	r *bufio.Reader
	w *bufio.Writer
}

func (c *conn) encodeResp(resp redis.Resp, timeout time.Duration) error {
	if err := c.sock.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	if err := redis.Encode(c.w, resp); err != nil {
		return err
	}
	return errors.Trace(c.w.Flush())
}

func (c *conn) decodeResp(timeout time.Duration) (redis.Resp, error) {
	if err := c.sock.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	return redis.Decode(c.r)
}

func (c *conn) Do(cmd *redis.Array, timeout time.Duration) (redis.Resp, error) {
	if c.err != nil {
		return nil, c.err
	}
	if err := c.encodeResp(cmd, timeout); err != nil {
		c.err = err
		log.WarnErrorf(err, "encode resp failed")
		return nil, c.err
	}
	if rsp, err := c.decodeResp(timeout); err != nil {
		c.err = err
		log.WarnErrorf(err, "decode resp failed")
		return nil, c.err
	} else {
		c.last = time.Now()
		return rsp, nil
	}
}

func (c *conn) DoMustOK(cmd *redis.Array, timeout time.Duration) error {
	if rsp, err := c.Do(cmd, timeout); err != nil {
		return err
	} else {
		s, ok := rsp.(*redis.String)
		if ok {
			if s.Value == "OK" {
				return nil
			}
			c.err = errors.Errorf("not OK, got %s", s.Value)
		} else {
			c.err = errors.Errorf("not string response, got %v", rsp.Type())
		}
		return c.err
	}
}

func init() {
	poolmap.m = make(map[string]*list.List)
	go func() {
		for {
			time.Sleep(time.Second)
			poolmap.Lock()
			for addr, pool := range poolmap.m {
				for i := pool.Len(); i != 0; i-- {
					c := pool.Remove(pool.Front()).(*conn)
					if time.Now().Before(c.last.Add(time.Second * 5)) {
						pool.PushBack(c)
					} else {
						c.sock.Close()
						log.Infof("close connection %s : %s", addr, c.summ)
					}
				}
				if pool.Len() != 0 {
					continue
				}
				delete(poolmap.m, addr)
			}
			poolmap.Unlock()
		}
	}()
}

func getSockConn(addr string, timeout time.Duration) (*conn, error) {
	poolmap.Lock()
	if pool := poolmap.m[addr]; pool != nil && pool.Len() != 0 {
		c := pool.Remove(pool.Front()).(*conn)
		poolmap.Unlock()
		return c, nil
	}
	poolmap.Unlock()
	sock, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	c := &conn{
		summ: fmt.Sprintf("<local> %s -- %s <remote>", sock.LocalAddr(), sock.RemoteAddr()),
		sock: sock,
		last: time.Now(),
		r:    bufio.NewReader(sock),
		w:    bufio.NewWriter(sock),
	}
	log.Infof("create connection %s : %s", addr, c.summ)
	return c, nil
}

func putSockConn(addr string, c *conn) {
	if c.err != nil {
		c.sock.Close()
		log.InfoErrorf(c.err, "close error connection %s : %s", addr, c.summ)
	} else {
		poolmap.Lock()
		pool := poolmap.m[addr]
		if pool == nil {
			pool = list.New()
			poolmap.m[addr] = pool
		}
		pool.PushFront(c)
		poolmap.Unlock()
	}
}

func doMigrate(addr string, timeout time.Duration, db uint32, bins []*rdb.BinEntry) error {
	c, err := getSockConn(addr, timeout)
	if err != nil {
		log.WarnErrorf(err, "connect to %s failed, timeout = %d", addr, timeout)
		return err
	}
	defer putSockConn(addr, c)

	cmd1 := redis.NewArray()
	cmd1.AppendBulkBytes([]byte("select"))
	cmd1.AppendBulkBytes([]byte(FormatUint(uint64(db))))

	if err := c.DoMustOK(cmd1, timeout); err != nil {
		log.WarnErrorf(err, "command select failed, addr = %s, db = %d", addr, db)
		return err
	}
	log.Debugf("command select ok, addr = %s, db = %d", addr, db)

	cmd2 := redis.NewArray()
	cmd2.AppendBulkBytes([]byte("slotsrestore"))
	for _, bin := range bins {
		cmd2.AppendBulkBytes(bin.Key)
		ttlms := int64(0)
		if bin.ExpireAt != 0 {
			if v, ok := ExpireAtToTTLms(bin.ExpireAt); ok && v > 0 {
				ttlms = v
			} else {
				ttlms = 1
			}
		}
		cmd2.AppendBulkBytes([]byte(FormatInt(ttlms)))
		cmd2.AppendBulkBytes(bin.Value)
	}

	if err := c.DoMustOK(cmd2, timeout); err != nil {
		log.WarnErrorf(err, "command restore failed, addr = %s, db = %d, len(bins) = %d", addr, db, len(bins))
		return err
	} else {
		log.Debugf("command restore ok, addr = %s, db = %d, len(bins) = %d", addr, db, len(bins))
		return nil
	}
}
