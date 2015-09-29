package topom

import (
	"sync"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/async"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
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
	return nil, errors.New("proxy does not exist")
}

func (s *Topom) maxProxyId() int {
	var maxId int
	for _, p := range s.proxies {
		if p.Id > maxId {
			maxId = p.Id
		}
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
		log.WarnErrorf(err, "proxy fetch model failed, target = %s", addr)
		return errors.New("fetch model failed")
	}
	c.SetXAuth(s.config.ProductName, s.config.ProductAuth, p.Token)

	if err := c.XPing(); err != nil {
		log.WarnErrorf(err, "proxy verify auth failed, target = %s", addr)
		return errors.New("verify auth failed")
	}

	if s.proxies[p.Token] != nil {
		log.Warnf("proxy-[%s] already exists, target = %s", p.Token, addr)
		return errors.New("proxy already exists")
	} else {
		p.Id = s.maxProxyId() + 1
	}

	if err := s.store.CreateProxy(p.Id, p); err != nil {
		log.WarnErrorf(err, "proxy-[%s] create failed, target = %s", p.Token, addr)
		return errors.New("proxy create failed")
	}

	log.Infof("[%p] create proxy: %s", s, p.ToJson())

	s.proxies[p.Token] = p
	s.clients[p.Token] = c
	return s.repairProxy(p.Token)
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

	log.Infof("proxy-[%s] will be marked as shutdown", token)

	if err := c.Shutdown(); err != nil {
		log.WarnErrorf(err, "proxy-[%s] shutdown failed, force = %t", token, force)
		if !force {
			return errors.New("proxy shutdown failed")
		}
	}

	if err := s.store.RemoveProxy(p.Id); err != nil {
		log.WarnErrorf(err, "proxy-[%s] remove failed", token)
		return errors.New("proxy remove failed")
	}

	log.Infof("[%p] remove proxy: %s", s, p.ToJson())

	delete(s.proxies, token)
	delete(s.clients, token)
	return nil
}

func (s *Topom) RepairProxy(token string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrClosedTopom
	}
	return s.repairProxy(token)
}

func (s *Topom) repairProxy(token string) error {
	c, err := s.getProxyClient(token)
	if err != nil {
		return err
	}
	if err := c.FillSlots(s.getSlots()...); err != nil {
		log.WarnErrorf(err, "proxy-[%s] repair failed", token)
		return errors.New("proxy fill slots failed")
	}
	if err := c.Start(); err != nil {
		log.WarnErrorf(err, "proxy-[%s] repair failed", token)
		return errors.New("proxy call start failed")
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
				log.WarnErrorf(err, "proxy-[%s] call xping failed", p.Token)
			}
			return errors.New("proxy xping failed")
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
	errs := s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
		x, err := c.Stats()
		if err != nil {
			if debug {
				log.WarnErrorf(err, "proxy-[%s] call stats failed", p.Token)
			}
			return errors.New("proxy stats failed")
		}
		lock.Lock()
		defer lock.Unlock()
		stats[p.Token] = x
		return nil
	})
	return stats, errs, nil
}

func (s *Topom) SummaryAll(debug bool) (map[string]*proxy.Summary, map[string]error, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, nil, ErrClosedTopom
	}
	var lock sync.Mutex
	var sums = make(map[string]*proxy.Summary)
	errs := s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
		x, err := c.Summary()
		if err != nil {
			if debug {
				log.WarnErrorf(err, "proxy-[%s] call summary failed", p.Token)
			}
			return errors.New("proxy summary failed")
		}
		lock.Lock()
		defer lock.Unlock()
		sums[p.Token] = x
		return nil
	})
	return sums, errs, nil
}

func (s *Topom) broadcast(fn func(p *models.Proxy, c *proxy.ApiClient) error) map[string]error {
	var rets = &struct {
		sync.Mutex
		wait sync.WaitGroup
		errs map[string]error
	}{errs: make(map[string]error)}

	for token, p := range s.proxies {
		c := s.clients[token]
		rets.wait.Add(1)
		async.Call(func() {
			defer rets.wait.Done()
			if err := fn(p, c); err != nil {
				rets.Lock()
				rets.errs[token] = err
				rets.Unlock()
			}
		})
	}
	rets.wait.Wait()
	return rets.errs
}
