// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
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
		log.WarnErrorf(err, "proxy@%s fetch model failed", addr)
		return errors.Errorf("proxy@%s fetch model failed", addr)
	}
	c := s.newProxyClient(p)

	if err := c.XPing(); err != nil {
		log.WarnErrorf(err, "proxy@%s check xauth failed", addr)
		return errors.Errorf("proxy@%s check xauth failed", addr)
	}

	if ctx.proxy[p.Token] != nil {
		log.Warnf("proxy@%s with token = %s already exists", addr, p.Token)
		return errors.Errorf("proxy-[%s] already exists", p.Token)
	} else {
		p.Id = ctx.maxProxyId() + 1
	}

	if err := s.store.UpdateProxy(p); err != nil {
		log.ErrorErrorf(err, "store: create proxy-[%s] failed", p.Token)
		return errors.Errorf("store: create proxy-[%s] failed", p.Token)
	}

	log.Infof("create proxy-[%s]:\n%s", p.Token, p.Encode())

	return s.reinitProxy(p.Token)
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
			return errors.Errorf("proxy-[%s] shutdown failed", token)
		}
	}

	if err := s.store.DeleteProxy(token); err != nil {
		log.ErrorErrorf(err, "store: remove proxy-[%s] failed", token)
		return errors.Errorf("store: remove proxy-[%s] failed", token)
	}

	log.Infof("remove proxy-[%s]:\n%s", token, p.Encode())

	return nil
}

func (s *Topom) ReinitProxy(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reinitProxy(token)
}

func (s *Topom) reinitProxy(token string) error {
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	p, err := ctx.getProxy(token)
	if err != nil {
		return err
	}
	c := s.newProxyClient(p)

	if err := c.FillSlots(ctx.toSlotList(ctx.slots, false)...); err != nil {
		log.WarnErrorf(err, "proxy-[%s] fillslots failed", token)
		return errors.Errorf("proxy-[%s] fillslots failed", token)
	}

	if err := c.Start(); err != nil {
		log.WarnErrorf(err, "proxy-[%s] start failed", token)
		return errors.Errorf("proxy-[%s] start failed", token)
	}
	return nil
}
