package topom

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

var ErrStatsTimeout = errors.New("update stats timeout")

type ServerStats struct {
	Infomap map[string]string
	Slotmap map[int]int
	Error   error
}

func (s *ServerStats) MarshalJSON() ([]byte, error) {
	var v = &struct {
		Infomap map[string]string `json:"infomap,omitempty"`
		Slotmap map[string]int    `json:"slotmap,omitempty"`
		Error   *rpc.RemoteError  `json:"error,omitempty"`
	}{
		s.Infomap, nil,
		rpc.ToRemoteError(s.Error),
	}
	if s.Slotmap != nil {
		v.Slotmap = make(map[string]int)
		for i, val := range s.Slotmap {
			v.Slotmap[strconv.Itoa(i)] = val
		}
	}
	return json.Marshal(v)
}

func (s *ServerStats) UnmarshalJSON(b []byte) error {
	var v = &struct {
		Infomap map[string]string `json:"infomap,omitempty"`
		Slotmap map[string]int    `json:"slotmap,omitempty"`
		Error   *rpc.RemoteError  `json:"error,omitempty"`
	}{}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	} else {
		s.Infomap = v.Infomap
		s.Error = v.Error.ToError()
		if v.Slotmap != nil {
			s.Slotmap = make(map[int]int)
			for a, val := range v.Slotmap {
				i, err := strconv.Atoi(a)
				if err != nil {
					return errors.Trace(err)
				}
				s.Slotmap[i] = val
			}
		}
		return nil
	}
}

func (s *Topom) UpdateServerStats(addr string, stats *ServerStats) bool {
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

func (s *Topom) runServerStats(addr string, timeout time.Duration) *ServerStats {
	var sigch = make(chan struct{})
	var stats = &ServerStats{}

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
		infomap, err := c.GetInfo()
		if err != nil {
			return err
		}
		slotmap, err := c.SlotsInfo()
		if err != nil {
			return err
		}
		stats.Infomap = infomap
		stats.Slotmap = slotmap
		return nil
	}()

	select {
	case <-sigch:
		return stats
	case <-time.After(timeout):
		return &ServerStats{Error: ErrStatsTimeout}
	}
}

func (s *Topom) RefreshServerStats(timeout time.Duration) *sync.WaitGroup {
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
			stats := s.runServerStats(addr, timeout)
			s.UpdateServerStats(addr, stats)
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
