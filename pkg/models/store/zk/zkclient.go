package zkstore

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/samuel/go-zookeeper/zk"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

var ErrClosedZkClient = errors.New("use of closed zk client")

type ZkClient struct {
	sync.Mutex

	conn *zk.Conn
	addr []string

	dialAt time.Time
	closed bool
}

type logger struct {
}

func (l *logger) Printf(format string, v ...interface{}) {
	log.Infof(format, v...)
}

func NewClient(addr []string) (*ZkClient, error) {
	c := &ZkClient{
		addr: addr,
	}
	if err := c.reset(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *ZkClient) reset() error {
	c.dialAt = time.Now()
	conn, events, err := zk.Connect(c.addr, time.Minute)
	if err != nil {
		return errors.Trace(err)
	}
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = conn
	conn.SetLogger(&logger{})

	go func() {
		for _ = range events {
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
	_, err := conn.Create(dir, []byte{}, 0, zk.WorldACL(zk.PermAll))
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *ZkClient) Create(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedZkClient)
	}
	return c.do(func(conn *zk.Conn) error {
		return c.create(conn, path, data)
	})
}

func (c *ZkClient) create(conn *zk.Conn, path string, data []byte) error {
	if err := c.mkdir(conn, filepath.Dir(path)); err != nil {
		return err
	}
	_, err := conn.Create(path, data, 0, zk.WorldACL(zk.PermAdmin|zk.PermRead|zk.PermWrite))
	if err != nil {
		return errors.Trace(err)
	}
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
	if err := c.create(conn, path, data); err != nil {
		if errors.NotEqual(err, zk.ErrNodeExists) {
			return err
		}
	}
	_, err := conn.Set(path, data, -1)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *ZkClient) Delete(path string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedZkClient)
	}
	return c.do(func(conn *zk.Conn) error {
		if err := conn.Delete(path, -1); err != nil {
			if errors.NotEqual(err, zk.ErrNoNode) {
				return errors.Trace(err)
			}
		}
		return nil
	})
}

func (c *ZkClient) LoadData(path string) ([]byte, error) {
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

func (c *ZkClient) ListFile(path string) ([]string, error) {
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
