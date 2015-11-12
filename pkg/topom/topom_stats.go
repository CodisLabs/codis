// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type RedisStats struct {
	Infom    map[string]string `json:"infom,omitempty"`
	UnixTime int64             `json:"unixtime"`
	Error    *rpc.RemoteError  `json:"error,omitempty"`
}

func (s *Topom) UpdateRedisStats(addr string, stats *RedisStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	_, ok := s.stats.servers[addr]
	if ok {
		s.stats.servers[addr] = stats
		return true
	}
	return false
}

func (s *Topom) newRedisStats(addr string, timeout time.Duration) *RedisStats {
	var ch = make(chan struct{})
	stats := &RedisStats{}

	go func() (err error) {
		defer func() {
			if err != nil {
				stats.Error = rpc.NewRemoteError(err)
			}
			close(ch)
		}()

		c, err := s.redisp.GetClient(addr)
		if err != nil {
			return err
		}
		defer s.redisp.PutClient(c)

		m, err := c.InfoMap()
		if err != nil {
			return err
		}
		stats.Infom = m
		return nil
	}()

	select {
	case <-ch:
		return stats
	case <-time.After(timeout):
		return &RedisStats{}
	}
}

func (s *Topom) RefreshRedisStats(timeout time.Duration) *sync.WaitGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil
	}
	var wg sync.WaitGroup
	for addr, _ := range s.stats.servers {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			stats := s.newRedisStats(addr, timeout)
			stats.UnixTime = time.Now().Unix()
			s.UpdateRedisStats(addr, stats)
		}(addr)
	}
	return &wg
}

type ProxyStats struct {
	Stats    *proxy.Stats     `json:"stats,omitempty"`
	UnixTime int64            `json:"unixtime"`
	Error    *rpc.RemoteError `json:"error,omitempty"`
}

func (s *Topom) UpdateProxyStats(token string, stats *ProxyStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	_, ok := s.stats.proxies[token]
	if ok {
		s.stats.proxies[token] = stats
		return true
	}
	return false
}

func (s *Topom) newProxyStats(c *proxy.ApiClient, timeout time.Duration) *ProxyStats {
	var ch = make(chan struct{})
	stats := &ProxyStats{}

	go func() (err error) {
		defer func() {
			if err != nil {
				stats.Error = rpc.NewRemoteError(err)
			}
			close(ch)
		}()

		x, err := c.Stats()
		if err != nil {
			return err
		}
		stats.Stats = x
		return nil
	}()

	select {
	case <-ch:
		return stats
	case <-time.After(timeout):
		return &ProxyStats{}
	}
}

func (s *Topom) RefreshProxyStats(timeout time.Duration) *sync.WaitGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil
	}
	var wg sync.WaitGroup
	for token, c := range s.clients {
		wg.Add(1)
		go func(token string, c *proxy.ApiClient) {
			defer wg.Done()
			stats := s.newProxyStats(c, timeout)
			stats.UnixTime = time.Now().Unix()
			s.UpdateProxyStats(token, stats)
		}(token, c)
	}
	return &wg
}
