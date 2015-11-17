// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"container/list"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"

	"github.com/wandoulabs/codis/pkg/utils/errors"
)

var ErrFailedRedisClient = errors.New("use of failed redis client")

type RedisClient struct {
	conn redis.Conn
	addr string

	LastErr error
	LastUse time.Time
	Timeout time.Duration
}

func NewRedisClient(addr string, auth string, timeout time.Duration) (*RedisClient, error) {
	c, err := redis.DialTimeout("tcp", addr, time.Second, timeout, timeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if auth != "" {
		if _, err := c.Do("AUTH", auth); err != nil {
			c.Close()
			return nil, errors.Trace(err)
		}
		if _, err := c.Do("CONFIG", "SET", "MASTERAUTH", auth); err != nil {
			c.Close()
			return nil, errors.Trace(err)
		}
	}
	return &RedisClient{
		conn: c, addr: addr, LastUse: time.Now(), Timeout: timeout,
	}, nil
}

func (c *RedisClient) Close() error {
	return c.conn.Close()
}

func (c *RedisClient) command(cmd string, args ...interface{}) (interface{}, error) {
	if c.LastErr != nil {
		return nil, ErrFailedRedisClient
	}
	if reply, err := c.conn.Do(cmd, args...); err != nil {
		c.LastErr = errors.Trace(err)
		return nil, c.LastErr
	} else {
		c.LastUse = time.Now()
		return reply, nil
	}
}

func (c *RedisClient) SlotsInfo() (map[int]int, error) {
	if reply, err := c.command("SLOTSINFO"); err != nil {
		return nil, err
	} else {
		infos, err := redis.Values(reply, nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
		slots := make(map[int]int)
		for i, info := range infos {
			p, err := redis.Ints(info, nil)
			if err != nil || len(p) != 2 {
				return nil, errors.Errorf("invalid response[%d] = %v", i, info)
			}
			slots[p[0]] = p[1]
		}
		return slots, nil
	}
}

func (c *RedisClient) InfoMap() (map[string]string, error) {
	var info map[string]string
	if reply, err := c.command("INFO"); err != nil {
		return nil, err
	} else {
		text, err := redis.String(reply, nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
		info = make(map[string]string)
		for _, line := range strings.Split(text, "\n") {
			kv := strings.SplitN(line, ":", 2)
			if len(kv) != 2 {
				continue
			}
			if key := strings.TrimSpace(kv[0]); key != "" {
				info[key] = strings.TrimSpace(kv[1])
			}
		}
		host := info["master_host"]
		port := info["master_port"]
		if host != "" || port != "" {
			info["master_addr"] = net.JoinHostPort(host, port)
		}
	}
	if reply, err := c.command("CONFIG", "GET", "maxmemory"); err != nil {
		return nil, err
	} else {
		p, err := redis.Values(reply, nil)
		if err != nil || len(p) != 2 {
			return nil, errors.Errorf("invalid response = %v", reply)
		}
		v, err := redis.Int(p[1], nil)
		if err != nil {
			return nil, errors.Errorf("invalid response = %v", reply)
		}
		info["maxmemory"] = strconv.Itoa(v)
	}
	return info, nil
}

func (c *RedisClient) SetMaster(master string) error {
	if master == "" || strings.ToUpper(master) == "NO:ONE" {
		if _, err := c.command("SLAVEOF", "NO", "ONE"); err != nil {
			return err
		}
	} else {
		host, port, err := net.SplitHostPort(master)
		if err != nil {
			return errors.Trace(err)
		}
		if _, err := c.command("SLAVEOF", host, port); err != nil {
			return err
		}
	}
	return nil
}

func (c *RedisClient) MigrateSlot(slot int, target string) (int, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return 0, errors.Trace(err)
	}
	var timeout = int(c.Timeout / time.Millisecond)
	if reply, err := c.command("SLOTSMGRTTAGSLOT", host, port, timeout, slot); err != nil {
		return 0, err
	} else {
		p, err := redis.Ints(redis.Values(reply, nil))
		if err != nil || len(p) != 2 {
			return 0, errors.Errorf("invalid response = %v", reply)
		}
		return p[1], nil
	}
}

var ErrClosedRedisPool = errors.New("use of closed redis pool")

type RedisPool struct {
	mu sync.Mutex

	auth    string
	pool    map[string]*list.List
	timeout time.Duration

	exit struct {
		C chan struct{}
	}

	closed bool
}

func NewRedisPool(auth string, timeout time.Duration) *RedisPool {
	p := &RedisPool{
		auth: auth, timeout: timeout,
		pool: make(map[string]*list.List),
	}
	p.exit.C = make(chan struct{})

	if timeout != 0 {
		go func() {
			var ticker = time.NewTicker(timeout)
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

func (p *RedisPool) isRecyclable(c *RedisClient) bool {
	if c.LastErr != nil {
		return false
	}
	if p.timeout == 0 {
		return true
	} else {
		return c.LastUse.Add(p.timeout / 2).After(time.Now())
	}
}

func (p *RedisPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	close(p.exit.C)

	for addr, list := range p.pool {
		for i := list.Len(); i != 0; i-- {
			c := list.Remove(list.Front()).(*RedisClient)
			c.Close()
		}
		delete(p.pool, addr)
	}
	return nil
}

func (p *RedisPool) Cleanup() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosedRedisPool
	}

	for addr, list := range p.pool {
		for i := list.Len(); i != 0; i-- {
			c := list.Remove(list.Front()).(*RedisClient)
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

func (p *RedisPool) GetClient(addr string) (*RedisClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, ErrClosedRedisPool
	}

	if list := p.pool[addr]; list != nil {
		for i := list.Len(); i != 0; i-- {
			c := list.Remove(list.Front()).(*RedisClient)
			if p.isRecyclable(c) {
				return c, nil
			} else {
				c.Close()
			}
		}
	}
	return NewRedisClient(addr, p.auth, p.timeout)
}

func (p *RedisPool) PutClient(client *RedisClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || !p.isRecyclable(client) {
		client.Close()
	} else {
		cache := p.pool[client.addr]
		if cache == nil {
			cache = list.New()
			p.pool[client.addr] = cache
		}
		cache.PushFront(client)
	}
}
