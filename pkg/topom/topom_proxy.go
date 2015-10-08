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
		log.WarnErrorf(err, "proxy fetch model failed, target = %s", addr)
		return errors.New("proxy fetch model failed")
	}
	c.SetXAuth(s.config.ProductName, s.config.ProductAuth, p.Token)

	if err := c.XPing(); err != nil {
		log.WarnErrorf(err, "proxy verify auth failed, target = %s", addr)
		return errors.New("proxy verify auth failed")
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

	log.Infof("[%p] create proxy: \n%s", s, p.ToJson())

	s.proxies[p.Token] = p
	s.clients[p.Token] = c
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
		log.WarnErrorf(err, "proxy-[%s] shutdown failed, force remove = %t", token, force)
		if !force {
			return errors.New("proxy shutdown failed")
		}
	}

	if err := s.store.RemoveProxy(p.Id); err != nil {
		log.WarnErrorf(err, "proxy-[%s] remove failed", token)
		return errors.New("proxy remove failed")
	}

	log.Infof("[%p] remove proxy: \n%s", s, p.ToJson())

	delete(s.proxies, token)
	delete(s.clients, token)
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
		log.WarnErrorf(err, "proxy-[%s] reinit failed", token)
		return errors.New("proxy fill slots failed")
	}
	if err := c.Start(); err != nil {
		log.WarnErrorf(err, "proxy-[%s] reinit failed", token)
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
	var errs = s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
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
