// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

var ErrStatsTimeout = errors.New("update stats timeout")

type RedisStats struct {
	Infom map[string]string
	Error error
}

func (s *RedisStats) MarshalJSON() ([]byte, error) {
	var v = &struct {
		Infom map[string]string `json:"infom,omitempty"`
		Error *rpc.RemoteError  `json:"error,omitempty"`
	}{
		s.Infom, rpc.ToRemoteError(s.Error),
	}
	return json.Marshal(v)
}

func (s *RedisStats) UnmarshalJSON(b []byte) error {
	var v = &struct {
		Infom map[string]string `json:"infom,omitempty"`
		Error *rpc.RemoteError  `json:"error,omitempty"`
	}{}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	} else {
		s.Infom = v.Infom
		s.Error = v.Error.ToError()
		return nil
	}
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

func (s *Topom) runRedisStats(addr string, timeout time.Duration) *RedisStats {
	var sigch = make(chan struct{})
	var stats = &RedisStats{}

	go func() (err error) {
		defer func() {
			stats.Error = err
			close(sigch)
		}()
		c, err := s.redisp.GetClient(addr)
		if err != nil {
			return err
		}
		defer s.redisp.PutClient(c)
		infom, err := c.GetInfo()
		if err != nil {
			return err
		}
		stats.Infom = infom
		return nil
	}()

	select {
	case <-sigch:
		return stats
	case <-time.After(timeout):
		return &RedisStats{Error: ErrStatsTimeout}
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
			stats := s.runRedisStats(addr, timeout)
			s.UpdateRedisStats(addr, stats)
		}(addr)
	}
	return &wg
}

type ProxyStats struct {
	Stats *proxy.Stats
	Error error
}

func (s *ProxyStats) MarshalJSON() ([]byte, error) {
	var v = &struct {
		Stats *proxy.Stats     `json:"stats,omitempty"`
		Error *rpc.RemoteError `json:"error,omitempty"`
	}{
		s.Stats, rpc.ToRemoteError(s.Error),
	}
	return json.Marshal(v)
}

func (s *ProxyStats) UnmarshalJSON(b []byte) error {
	var v = &struct {
		Stats *proxy.Stats     `json:"stats,omitempty"`
		Error *rpc.RemoteError `json:"error,omitempty"`
	}{}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	} else {
		s.Stats = v.Stats
		s.Error = v.Error.ToError()
		return nil
	}
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

func (s *Topom) runProxyStats(c *proxy.ApiClient, timeout time.Duration) *ProxyStats {
	var sigch = make(chan struct{})
	var stats = &ProxyStats{}

	go func() (err error) {
		defer func() {
			stats.Error = err
			close(sigch)
		}()
		x, err := c.Stats()
		if err != nil {
			return err
		}
		stats.Stats = x
		return nil
	}()

	select {
	case <-sigch:
		return stats
	case <-time.After(timeout):
		return &ProxyStats{Error: ErrStatsTimeout}
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
			stats := s.runProxyStats(c, timeout)
			s.UpdateProxyStats(token, stats)
		}(token, c)
	}
	return &wg
}
