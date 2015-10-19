package topom

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type ServerStats struct {
	Stats map[string]string
	Error error

	UnixTime int64
}

func (s *ServerStats) MarshalJSON() ([]byte, error) {
	var v = &struct {
		Stats    map[string]string `json:"stats,omitempty"`
		Error    *rpc.RemoteError  `json:"error,omitempty"`
		UnixTime int64             `json:"unix_time"`
	}{
		s.Stats, rpc.ToRemoteError(s.Error), s.UnixTime,
	}
	return json.Marshal(v)
}

func (s *ServerStats) UnmarshalJSON(b []byte) error {
	var v = &struct {
		Stats    map[string]string `json:"stats,omitempty"`
		Error    *rpc.RemoteError  `json:"error,omitempty"`
		UnixTime int64             `json:"unix_time"`
	}{}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	} else {
		s.Stats = v.Stats
		s.Error = v.Error.ToError()
		s.UnixTime = v.UnixTime
		return nil
	}
}

func (s *Topom) updateServerStats(addr string, stats *ServerStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	_, ok := s.stats.servers[addr]
	if ok {
		s.stats.servers[addr] = stats
		stats.UnixTime = time.Now().Unix()
		return true
	}
	return false
}

func (s *Topom) runServerStats(addr string, timeout time.Duration) *ServerStats {
	var ch = make(chan *ServerStats, 1)

	go func() (stats map[string]string, err error) {
		defer func() {
			ch <- &ServerStats{
				Stats: stats, Error: err,
			}
		}()
		c, err := s.redisp.GetClient(addr)
		if err != nil {
			return nil, err
		}
		defer s.redisp.PutClient(c)
		return c.GetInfo()
	}()

	select {
	case stats := <-ch:
		return stats
	case <-time.After(timeout):
		return &ServerStats{}
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
			s.updateServerStats(addr, stats)
		}(addr)
	}
	return &wg
}

type ProxyStats struct {
	Stats *proxy.Stats
	Error error

	UnixTime int64
}

func (s *ProxyStats) MarshalJSON() ([]byte, error) {
	var v = &struct {
		Stats    *proxy.Stats     `json:"stats,omitempty"`
		Error    *rpc.RemoteError `json:"error,omitempty"`
		UnixTime int64            `json:"unixtime"`
	}{
		s.Stats, rpc.ToRemoteError(s.Error), s.UnixTime,
	}
	return json.Marshal(v)
}

func (s *ProxyStats) UnmarshalJSON(b []byte) error {
	var v = &struct {
		Stats    *proxy.Stats     `json:"stats,omitempty"`
		Error    *rpc.RemoteError `json:"error,omitempty"`
		UnixTime int64            `json:"unixtime"`
	}{}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	} else {
		s.Stats = v.Stats
		s.Error = v.Error.ToError()
		s.UnixTime = v.UnixTime
		return nil
	}
}

func (s *Topom) updateProxyStats(token string, stats *ProxyStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	_, ok := s.stats.proxies[token]
	if ok {
		s.stats.proxies[token] = stats
		stats.UnixTime = time.Now().Unix()
		return true
	}
	return false
}

func (s *Topom) runProxyStats(c *proxy.ApiClient, timeout time.Duration) *ProxyStats {
	var ch = make(chan *ProxyStats, 1)

	go func() (stats *proxy.Stats, err error) {
		defer func() {
			ch <- &ProxyStats{
				Stats: stats, Error: err,
			}
		}()
		return c.Stats()
	}()

	select {
	case stats := <-ch:
		return stats
	case <-time.After(time.Second):
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
			stats := s.runProxyStats(c, timeout)
			s.updateProxyStats(token, stats)
		}(token, c)
	}
	return &wg
}
