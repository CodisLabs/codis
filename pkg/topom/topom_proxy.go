// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/sync2"
)

func (s *Topom) CreateProxy(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	p, err := proxy.NewApiClient(addr).Model()
	if err != nil {
		return errors.Errorf("proxy@%s fetch model failed, %s", addr, err)
	}
	c := s.newProxyClient(p)

	if err := c.XPing(); err != nil {
		return errors.Errorf("proxy@%s check xauth failed, %s", addr, err)
	}
	if ctx.proxy[p.Token] != nil {
		return errors.Errorf("proxy-[%s] already exists", p.Token)
	} else {
		p.Id = ctx.maxProxyId() + 1
	}
	defer s.dirtyProxyCache(p.Token)

	if err := s.storeCreateProxy(p); err != nil {
		return err
	} else {
		return s.reinitProxy(ctx, p, c)
	}
}

func (s *Topom) OnlineProxy(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	p, err := proxy.NewApiClient(addr).Model()
	if err != nil {
		return errors.Errorf("proxy@%s fetch model failed", addr)
	}
	c := s.newProxyClient(p)

	if err := c.XPing(); err != nil {
		return errors.Errorf("proxy@%s check xauth failed", addr)
	}
	defer s.dirtyProxyCache(p.Token)

	if d := ctx.proxy[p.Token]; d != nil {
		p.Id = d.Id
		if err := s.storeUpdateProxy(p); err != nil {
			return err
		}
	} else {
		p.Id = ctx.maxProxyId() + 1
		if err := s.storeCreateProxy(p); err != nil {
			return err
		}
	}
	return s.reinitProxy(ctx, p, c)
}

func (s *Topom) RemoveProxy(token string, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	p, err := ctx.getProxy(token)
	if err != nil {
		return err
	}
	c := s.newProxyClient(p)

	if err := c.Shutdown(); err != nil {
		log.WarnErrorf(err, "proxy-[%s] shutdown failed, force remove = %t", token, force)
		if !force {
			return errors.Errorf("proxy-[%s] shutdown failed", p.Token)
		}
	}
	defer s.dirtyProxyCache(p.Token)

	return s.storeRemoveProxy(p)
}

func (s *Topom) ReinitProxy(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	p, err := ctx.getProxy(token)
	if err != nil {
		return err
	}
	c := s.newProxyClient(p)

	return s.reinitProxy(ctx, p, c)
}

func (s *Topom) newProxyClient(p *models.Proxy) *proxy.ApiClient {
	c := proxy.NewApiClient(p.AdminAddr)
	c.SetXAuth(s.config.ProductName, s.config.ProductAuth, p.Token)
	return c
}

func (s *Topom) reinitProxy(ctx *context, p *models.Proxy, c *proxy.ApiClient) error {
	log.Warnf("proxy-[%s] reinit:\n%s", p.Token, p.Encode())
	if err := c.FillSlots(ctx.toSlotSlice(ctx.slots, p)...); err != nil {
		log.ErrorErrorf(err, "proxy-[%s] fillslots failed", p.Token)
		return errors.Errorf("proxy-[%s] fillslots failed", p.Token)
	}
	if err := c.Start(); err != nil {
		log.ErrorErrorf(err, "proxy-[%s] start failed", p.Token)
		return errors.Errorf("proxy-[%s] start failed", p.Token)
	}
	if err := c.SetSentinels(ctx.sentinel); err != nil {
		log.ErrorErrorf(err, "proxy-[%s] set sentinels failed", p.Token)
		return errors.Errorf("proxy-[%s] set sentinels failed", p.Token)
	}
	return nil
}

func (s *Topom) resyncSlotMappingsByGroupId(ctx *context, gid int) error {
	return s.resyncSlotMappings(ctx, ctx.getSlotMappingsByGroupId(gid)...)
}

func (s *Topom) resyncSlotMappings(ctx *context, slots ...*models.SlotMapping) error {
	if len(slots) == 0 {
		return nil
	}
	var fut sync2.Future
	for _, p := range ctx.proxy {
		fut.Add()
		go func(p *models.Proxy) {
			err := s.newProxyClient(p).FillSlots(ctx.toSlotSlice(slots, p)...)
			if err != nil {
				log.ErrorErrorf(err, "proxy-[%s] resync slots failed", p.Token)
			}
			fut.Done(p.Token, err)
		}(p)
	}
	for t, v := range fut.Wait() {
		switch err := v.(type) {
		case error:
			if err != nil {
				return errors.Errorf("proxy-[%s] resync slots failed", t)
			}
		}
	}
	return nil
}
