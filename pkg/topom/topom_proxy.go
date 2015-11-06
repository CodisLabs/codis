// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"sync"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func (s *Topom) GetProxyModels() []*models.Proxy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getProxyModels()
}

func (s *Topom) getProxyModels() []*models.Proxy {
	plist := make([]*models.Proxy, 0, len(s.proxies))
	for _, p := range s.proxies {
		plist = append(plist, p)
	}
	models.SortProxy(plist, func(p1, p2 *models.Proxy) bool {
		return p1.Id < p2.Id
	})
	return plist
}

func (s *Topom) getProxyClient(token string) (*proxy.ApiClient, error) {
	if c := s.clients[token]; c != nil {
		return c, nil
	}
	return nil, errors.Errorf("proxy-[%s] doesn't exist", token)
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
		return errors.Errorf("call rpc model to proxy@%s failed", addr)
	}
	c.SetXAuth(s.config.ProductName, s.config.ProductAuth, p.Token)

	if err := c.XPing(); err != nil {
		log.WarnErrorf(err, "[%p] proxy@%s check xauth failed", s, addr)
		return errors.Errorf("call rpc xauth to proxy@%s failed", addr)
	}

	if s.proxies[p.Token] != nil {
		log.Warnf("[%p] proxy@%s with token = %s already exists", s, addr, p.Token)
		return errors.Errorf("proxy-[%s] already exists", p.Token)
	} else {
		p.Id = s.maxProxyId() + 1
	}

	if err := s.store.CreateProxy(p.Id, p); err != nil {
		log.ErrorErrorf(err, "[%p] create proxy-[%d] failed", s, p.Id)
		return errors.Errorf("store: create proxy-[%d] failed", p.Id)
	}

	s.proxies[p.Token] = p
	s.clients[p.Token] = c
	s.stats.proxies[p.Token] = nil

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
		log.WarnErrorf(err, "[%p] proxy-[%s] shutdown failed, force remove = %t", s, token, force)
		if !force {
			return errors.Errorf("call rpc shutdown to proxy-[%s] failed", token)
		}
	}

	if err := s.store.RemoveProxy(p.Id); err != nil {
		log.ErrorErrorf(err, "[%p] remove proxy-[%d] failed", s, p.Id)
		return errors.Errorf("store: remove proxy-[%d] failed", p.Id)
	}

	delete(s.proxies, token)
	delete(s.clients, token)
	delete(s.stats.proxies, token)

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
		return errors.Errorf("call rpc fillslots to proxy-[%s] failed", token)
	}
	if err := c.Start(); err != nil {
		log.WarnErrorf(err, "[%p] proxy-[%s] call start failed", s, token)
		return errors.Errorf("call rpc start to proxy-[%s] failed", token)
	}
	return nil
}

func (s *Topom) broadcast(fn func(p *models.Proxy, c *proxy.ApiClient) error) map[string]error {
	var lock sync.Mutex
	var wait sync.WaitGroup
	var errs = make(map[string]error)
	for token, _ := range s.proxies {
		wait.Add(1)
		go func(p *models.Proxy, c *proxy.ApiClient) {
			defer wait.Done()
			err := fn(p, c)
			lock.Lock()
			errs[p.Token] = err
			lock.Unlock()
		}(s.proxies[token], s.clients[token])
	}
	wait.Wait()
	return errs
}

func (s *Topom) resyncPrepare() error {
	errs := s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
		if err := c.XPing(); err != nil {
			log.WarnErrorf(err, "[%p] proxy-[%s] resync prepare failed", s, p.Token)
			return err
		}
		return nil
	})
	for t, err := range errs {
		if err != nil {
			return errors.Errorf("proxy-[%s] resync prepare failed", t)
		}
	}
	return nil
}
