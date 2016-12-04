// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"container/list"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"

	redigo "github.com/garyburd/redigo/redis"
)

type Client struct {
	conn redigo.Conn
	addr string
	auth string

	LastUse time.Time
	Timeout time.Duration
}

func NewClientNoAuth(addr string, timeout time.Duration) (*Client, error) {
	return NewClient(addr, "", timeout)
}

func NewClient(addr string, auth string, timeout time.Duration) (*Client, error) {
	c, err := redigo.Dial("tcp", addr, []redigo.DialOption{
		redigo.DialConnectTimeout(time.Second),
		redigo.DialPassword(auth),
		redigo.DialReadTimeout(timeout),
		redigo.DialWriteTimeout(time.Second * 5),
	}...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &Client{
		conn: c, addr: addr, auth: auth,
		LastUse: time.Now(), Timeout: timeout,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Do(cmd string, args ...interface{}) (interface{}, error) {
	r, err := c.conn.Do(cmd, args...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	c.LastUse = time.Now()
	return r, nil
}

func (c *Client) Flush(cmd string, args ...interface{}) error {
	if err := c.conn.Send(cmd, args...); err != nil {
		return errors.Trace(err)
	}
	if err := c.conn.Flush(); err != nil {
		return errors.Trace(err)
	}
	c.LastUse = time.Now()
	return nil
}

func (c *Client) Receive() (interface{}, error) {
	r, err := c.conn.Receive()
	if err != nil {
		return nil, errors.Trace(err)
	}
	return r, nil
}

func (c *Client) Info() (map[string]string, error) {
	r, err := c.Do("INFO")
	if err != nil {
		return nil, err
	}
	text, err := redigo.String(r, nil)
	if err != nil {
		return nil, errors.Trace(err)
	}
	info := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		if key := strings.TrimSpace(kv[0]); key != "" {
			info[key] = strings.TrimSpace(kv[1])
		}
	}
	return info, nil
}

func (c *Client) InfoFull() (map[string]string, error) {
	if info, err := c.Info(); err != nil {
		return nil, err
	} else {
		host := info["master_host"]
		port := info["master_port"]
		if host != "" || port != "" {
			info["master_addr"] = net.JoinHostPort(host, port)
		}
		r, err := c.Do("CONFIG", "get", "maxmemory")
		if err != nil {
			return nil, err
		}
		p, err := redigo.Values(r, nil)
		if err != nil || len(p) != 2 {
			return nil, errors.Errorf("invalid response = %v", r)
		}
		v, err := redigo.Int(p[1], nil)
		if err != nil {
			return nil, errors.Errorf("invalid response = %v", r)
		}
		info["maxmemory"] = strconv.Itoa(v)
		return info, nil
	}
}

func (c *Client) SetMaster(master string) error {
	if master == "" || strings.ToUpper(master) == "NO:ONE" {
		if _, err := c.Do("SLAVEOF", "NO", "ONE"); err != nil {
			return err
		}
	} else {
		host, port, err := net.SplitHostPort(master)
		if err != nil {
			return errors.Trace(err)
		}
		if _, err := c.Do("CONFIG", "set", "masterauth", c.auth); err != nil {
			return err
		}
		if _, err := c.Do("SLAVEOF", host, port); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) MigrateSlot(slot int, target string) (int, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return 0, errors.Trace(err)
	}
	mseconds := int(c.Timeout / time.Millisecond)
	if reply, err := c.Do("SLOTSMGRTTAGSLOT", host, port, mseconds, slot); err != nil {
		return 0, err
	} else {
		p, err := redigo.Ints(redigo.Values(reply, nil))
		if err != nil || len(p) != 2 {
			return 0, errors.Errorf("invalid response = %v", reply)
		}
		return p[1], nil
	}
}

func (c *Client) SlotsInfo() (map[int]int, error) {
	if reply, err := c.Do("SLOTSINFO"); err != nil {
		return nil, err
	} else {
		infos, err := redigo.Values(reply, nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
		slots := make(map[int]int)
		for i, info := range infos {
			p, err := redigo.Ints(info, nil)
			if err != nil || len(p) != 2 {
				return nil, errors.Errorf("invalid response[%d] = %v", i, info)
			}
			slots[p[0]] = p[1]
		}
		return slots, nil
	}
}

func (c *Client) Role() (string, error) {
	if reply, err := c.Do("ROLE"); err != nil {
		return "", err
	} else {
		values, err := redigo.Values(reply, nil)
		if err != nil {
			return "", errors.Trace(err)
		}
		if len(values) == 0 {
			return "", errors.Errorf("invalid response = %v", reply)
		}
		role, err := redigo.String(values[0], nil)
		if err != nil {
			return "", errors.Errorf("invalid response[0] = %v", values[0])
		}
		return strings.ToUpper(role), nil
	}
}

var ErrClosedPool = errors.New("use of closed redis pool")

type Pool struct {
	mu sync.Mutex

	auth    string
	pool    map[string]*list.List
	timeout time.Duration

	exit struct {
		C chan struct{}
	}

	closed bool
}

func NewPool(auth string, timeout time.Duration) *Pool {
	p := &Pool{
		auth: auth, timeout: timeout,
		pool: make(map[string]*list.List),
	}
	p.exit.C = make(chan struct{})

	if timeout != 0 {
		go func() {
			var ticker = time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-p.exit.C:
					return
				case <-ticker.C:
					p.Cleanup()
				}
			}
		}()
	}

	return p
}

func (p *Pool) isRecyclable(c *Client) bool {
	if c.conn.Err() != nil {
		return false
	}
	return p.timeout == 0 || time.Since(c.LastUse) < p.timeout
}

func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	close(p.exit.C)

	for addr, list := range p.pool {
		for i := list.Len(); i != 0; i-- {
			c := list.Remove(list.Front()).(*Client)
			c.Close()
		}
		delete(p.pool, addr)
	}
	return nil
}

func (p *Pool) Cleanup() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosedPool
	}

	for addr, list := range p.pool {
		for i := list.Len(); i != 0; i-- {
			c := list.Remove(list.Front()).(*Client)
			if p.isRecyclable(c) {
				list.PushBack(c)
			} else {
				c.Close()
			}
		}
		if list.Len() == 0 {
			delete(p.pool, addr)
		}
	}
	return nil
}

func (p *Pool) GetClient(addr string) (*Client, error) {
	c, err := p.getClientFromCache(addr)
	if err != nil || c != nil {
		return c, err
	}
	return NewClient(addr, p.auth, p.timeout)
}

func (p *Pool) getClientFromCache(addr string) (*Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, ErrClosedPool
	}
	if list := p.pool[addr]; list != nil {
		for i := list.Len(); i != 0; i-- {
			c := list.Remove(list.Front()).(*Client)
			if p.isRecyclable(c) {
				return c, nil
			} else {
				c.Close()
			}
		}
	}
	return nil, nil
}

func (p *Pool) PutClient(c *Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || !p.isRecyclable(c) {
		c.Close()
	} else {
		cache := p.pool[c.addr]
		if cache == nil {
			cache = list.New()
			p.pool[c.addr] = cache
		}
		cache.PushFront(c)
	}
}

func (p *Pool) Info(addr string) (map[string]string, error) {
	c, err := p.GetClient(addr)
	if err != nil {
		return nil, err
	}
	defer p.PutClient(c)
	return c.Info()
}

func (p *Pool) InfoFull(addr string) (map[string]string, error) {
	c, err := p.GetClient(addr)
	if err != nil {
		return nil, err
	}
	defer p.PutClient(c)
	return c.InfoFull()
}

func (p *Pool) MigrateSlot(slot int, from, dest string) (int, error) {
	c, err := p.GetClient(from)
	if err != nil {
		return 0, err
	}
	defer p.PutClient(c)
	return c.MigrateSlot(slot, dest)
}
