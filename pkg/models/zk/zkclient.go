// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package zkclient

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/samuel/go-zookeeper/zk"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

var ErrClosedZkClient = errors.New("use of closed zk client")

var DefaultLogfunc = func(format string, v ...interface{}) {
	log.Info("zookeeper - " + fmt.Sprintf(format, v...))
}

type ZkClient struct {
	sync.Mutex

	conn *zk.Conn
	addr string

	dialAt time.Time
	closed bool

	logger  *zkLogger
	timeout time.Duration
}

type zkLogger struct {
	logfunc func(format string, v ...interface{})
}

func (l *zkLogger) Printf(format string, v ...interface{}) {
	if l != nil && l.logfunc != nil {
		l.logfunc(format, v...)
	}
}

func New(addr string, timeout time.Duration) (*ZkClient, error) {
	return NewWithLogfunc(addr, timeout, DefaultLogfunc)
}

func NewWithLogfunc(addr string, timeout time.Duration, logfunc func(foramt string, v ...interface{})) (*ZkClient, error) {
	c := &ZkClient{
		addr: addr, timeout: timeout, logger: &zkLogger{logfunc},
	}
	if err := c.reset(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *ZkClient) reset() error {
	c.dialAt = time.Now()
	conn, events, err := zk.Connect(strings.Split(c.addr, ","), c.timeout)
	if err != nil {
		return errors.Trace(err)
	}
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = conn
	c.conn.SetLogger(c.logger)

	c.logger.Printf("zkclient create new connection to %s", c.addr)

	go func() {
		for e := range events {
			log.Debugf("zookeeper event: %+v", e)
		}
	}()
	return nil
}

func (c *ZkClient) Close() error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}

func (c *ZkClient) Do(fn func(conn *zk.Conn) error) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedZkClient)
	}
	return c.do(fn)
}

func (c *ZkClient) do(fn func(conn *zk.Conn) error) error {
	if err := fn(c.conn); err != nil {
		for _, e := range []error{zk.ErrNoNode, zk.ErrNodeExists, zk.ErrNotEmpty} {
			if errors.Equal(e, err) {
				return err
			}
		}
		if time.Now().After(c.dialAt.Add(time.Second)) {
			c.reset()
		}
		return err
	}
	return nil
}

func (c *ZkClient) Mkdir(dir string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedZkClient)
	}
	return c.do(func(conn *zk.Conn) error {
		return c.mkdir(conn, dir)
	})
}

func (c *ZkClient) mkdir(conn *zk.Conn, dir string) error {
	if dir == "" || dir == "/" {
		return nil
	}
	if exists, _, err := conn.Exists(dir); err != nil {
		return errors.Trace(err)
	} else if exists {
		return nil
	}
	if err := c.mkdir(conn, filepath.Dir(dir)); err != nil {
		return err
	}
	log.Debugf("zkclient mkdir = %s", dir)
	_, err := conn.Create(dir, []byte{}, 0, zk.WorldACL(zk.PermAll))
	if err != nil {
		log.Debugf("zkclient mkdir = %s failed: %s", dir, err)
		return errors.Trace(err)
	}
	log.Debugf("zkclient mkdir OK")
	return nil
}

func (c *ZkClient) Create(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedZkClient)
	}
	return c.do(func(conn *zk.Conn) error {
		return c.create(conn, path, data, false)
	})
}

func (c *ZkClient) CreateEphemeral(path string, data []byte) (<-chan struct{}, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedZkClient)
	}
	var watch chan struct{}
	err := c.do(func(conn *zk.Conn) error {
		if err := c.create(conn, path, data, true); err != nil {
			return err
		}
		log.Debugf("zkclient create-ephemeral %s", path)
		if _, _, w, err := conn.GetW(path); err != nil {
			log.Debugf("zkclient create-ephemeral %s failed: %s", path, err)
			return errors.Trace(err)
		} else {
			log.Debugf("zkclient create-ephemeral OK")
			watch = make(chan struct{})
			go func() {
				<-w
				close(watch)
				log.Debugf("zkclient watching node %s lost", path)
			}()
			return nil
		}
	})
	return watch, err
}

func (c *ZkClient) create(conn *zk.Conn, path string, data []byte, ephemeral bool) error {
	if err := c.mkdir(conn, filepath.Dir(path)); err != nil {
		return err
	}
	var flag int32
	if ephemeral {
		flag |= zk.FlagEphemeral
	}
	log.Debugf("zkclient create node %s", path)
	_, err := conn.Create(path, data, flag, zk.WorldACL(zk.PermAdmin|zk.PermRead|zk.PermWrite))
	if err != nil {
		log.Debugf("zkclient create node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("zkclient create node OK")
	return nil
}

func (c *ZkClient) Update(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedZkClient)
	}
	return c.do(func(conn *zk.Conn) error {
		return c.update(conn, path, data)
	})
}

func (c *ZkClient) update(conn *zk.Conn, path string, data []byte) error {
	if exists, _, err := conn.Exists(path); err != nil {
		return errors.Trace(err)
	} else if !exists {
		if err := c.create(conn, path, data, false); err != nil {
			if errors.NotEqual(err, zk.ErrNodeExists) {
				return err
			}
		}
	}
	log.Debugf("zkclient update node %s", path)
	_, err := conn.Set(path, data, -1)
	if err != nil {
		log.Debugf("zkclient update node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("zkclient update node OK")
	return nil
}

func (c *ZkClient) Delete(path string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedZkClient)
	}
	return c.do(func(conn *zk.Conn) error {
		log.Debugf("zkclient delete node %s", path)
		if err := conn.Delete(path, -1); err != nil {
			if errors.NotEqual(err, zk.ErrNoNode) {
				log.Debugf("zkclient delete node %s failed: %s", path, err)
				return errors.Trace(err)
			}
		}
		log.Debugf("zkclient delete node OK")
		return nil
	})
}

func (c *ZkClient) Read(path string) ([]byte, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedZkClient)
	}
	var data []byte
	err := c.do(func(conn *zk.Conn) error {
		if bytes, _, err := conn.Get(path); err != nil {
			if errors.NotEqual(err, zk.ErrNoNode) {
				return errors.Trace(err)
			}
		} else {
			data = bytes
		}
		return nil
	})
	return data, err
}

func (c *ZkClient) List(path string) ([]string, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedZkClient)
	}
	var list []string
	err := c.do(func(conn *zk.Conn) error {
		if files, _, err := conn.Children(path); err != nil {
			if errors.NotEqual(err, zk.ErrNoNode) {
				return errors.Trace(err)
			}
		} else {
			for _, file := range files {
				list = append(list, filepath.Join(path, file))
			}
		}
		return nil
	})
	return list, err
}
