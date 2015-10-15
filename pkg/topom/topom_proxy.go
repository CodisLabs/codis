package topom

import (
	"sync"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/async"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

var (
	ErrProxyExists    = errors.New("proxy already exists")
	ErrProxyNotExists = errors.New("proxy does not exist")
	ErrProxyRpcFailed = errors.New("proxy call rpc failed")
)

func (s *Topom) ListProxy() []*models.Proxy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	plist := make([]*models.Proxy, 0, len(s.proxies))
	for _, p := range s.proxies {
		plist = append(plist, p)
	}
	return plist
}

func (s *Topom) getProxyClient(token string) (*proxy.ApiClient, error) {
	if c := s.clients[token]; c != nil {
		return c, nil
	}
	return nil, errors.Trace(ErrProxyNotExists)
}

func (s *Topom) maxProxyId() int {
	var maxId int
	for _, p := range s.proxies {
		maxId = utils.MaxInt(maxId, p.Id)
	}
	return maxId
}

func (s *Topom) CreateProxy(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	c := proxy.NewApiClient(addr)
	p, err := c.Model()
	if err != nil {
		log.WarnErrorf(err, "[%p] proxy@%s fetch model failed", s, addr)
		return errors.Trace(ErrProxyRpcFailed)
	}
	c.SetXAuth(s.config.ProductName, s.config.ProductAuth, p.Token)

	if err := c.XPing(); err != nil {
		log.WarnErrorf(err, "[%p] proxy@%s check xauth failed", s, addr)
		return errors.Trace(ErrProxyRpcFailed)
	}

	if s.proxies[p.Token] != nil {
		log.Warnf("[%p] proxy@%s with token=%s already exists", s, addr, p.Token)
		return errors.Trace(ErrProxyExists)
	} else {
		p.Id = s.maxProxyId() + 1
	}

	if err := s.store.CreateProxy(p.Id, p); err != nil {
		log.ErrorErrorf(err, "[%p] create proxy-[%d] failed", s, p.Id)
		return errors.Trace(ErrUpdateStore)
	}

	s.proxies[p.Token] = p
	s.clients[p.Token] = c

	log.Infof("[%p] create proxy-[%d]:\n%s", s, p.Id, p.Encode())

	return s.reinitProxy(p.Token)
}

func (s *Topom) RemoveProxy(token string, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	c, err := s.getProxyClient(token)
	if err != nil {
		return err
	}
	p := s.proxies[token]

	if err := c.Shutdown(); err != nil {
		log.WarnErrorf(err, "[%p] proxy-[%d] shutdown failed, force remove = %t", s, p.Id, force)
		if !force {
			return errors.Trace(ErrProxyRpcFailed)
		}
	}

	if err := s.store.RemoveProxy(p.Id); err != nil {
		log.ErrorErrorf(err, "[%p] remove proxy-[%d] failed", s, p.Id)
		return errors.Trace(ErrUpdateStore)
	}

	delete(s.proxies, token)
	delete(s.clients, token)

	log.Infof("[%p] remove proxy-[%d]:\n%s", s, p.Id, p.Encode())

	return nil
}

func (s *Topom) ReinitProxy(token string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrClosedTopom
	}
	return s.reinitProxy(token)
}

func (s *Topom) reinitProxy(token string) error {
	c, err := s.getProxyClient(token)
	if err != nil {
		return err
	}
	if err := c.FillSlots(s.getSlots()...); err != nil {
		log.WarnErrorf(err, "[%p] proxy-[%s] fill slots failed", s, token)
		return errors.Trace(ErrProxyRpcFailed)
	}
	if err := c.Start(); err != nil {
		log.WarnErrorf(err, "[%p] proxy-[%s] call start failed", s, token)
		return errors.Trace(ErrProxyRpcFailed)
	}
	return nil
}

func (s *Topom) XPingAll(debug bool) (map[string]error, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosedTopom
	}
	return s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
		if err := c.XPing(); err != nil {
			if debug {
				log.WarnErrorf(err, "[%p] proxy-[%s] call xping failed", s, p.Token)
			}
			return errors.Trace(ErrProxyRpcFailed)
		}
		return nil
	}), nil
}

func (s *Topom) StatsAll(debug bool) (map[string]*proxy.Stats, map[string]error, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, nil, ErrClosedTopom
	}
	var lock sync.Mutex
	var stats = make(map[string]*proxy.Stats)
	var errs = s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
		x, err := c.Stats()
		if err != nil {
			if debug {
				log.WarnErrorf(err, "[%p] proxy-[%s] call stats failed", s, p.Token)
			}
			return errors.Trace(ErrProxyRpcFailed)
		}
		lock.Lock()
		defer lock.Unlock()
		stats[p.Token] = x
		return nil
	})
	return stats, errs, nil
}

func (s *Topom) broadcast(fn func(p *models.Proxy, c *proxy.ApiClient) error) map[string]error {
	var lock sync.Mutex
	var wait sync.WaitGroup
	var errs = make(map[string]error)
	for token, p := range s.proxies {
		c := s.clients[token]
		wait.Add(1)
		async.Call(func() {
			defer wait.Done()
			if err := fn(p, c); err != nil {
				lock.Lock()
				errs[token] = err
				lock.Unlock()
			}
		})
	}
	wait.Wait()
	return errs
}
