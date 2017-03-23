// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

func (s *Topom) dirtySlotsCache(sid int) {
	s.cache.hooks.PushBack(func() {
		if s.cache.slots != nil {
			s.cache.slots[sid] = nil
		}
	})
}

func (s *Topom) dirtyGroupCache(gid int) {
	s.cache.hooks.PushBack(func() {
		if s.cache.group != nil {
			s.cache.group[gid] = nil
		}
	})
}

func (s *Topom) dirtyProxyCache(token string) {
	s.cache.hooks.PushBack(func() {
		if s.cache.proxy != nil {
			s.cache.proxy[token] = nil
		}
	})
}

func (s *Topom) dirtySentinelCache() {
	s.cache.hooks.PushBack(func() {
		s.cache.sentinel = nil
	})
}

func (s *Topom) dirtyCacheAll() {
	s.cache.hooks.PushBack(func() {
		s.cache.slots = nil
		s.cache.group = nil
		s.cache.proxy = nil
		s.cache.sentinel = nil
	})
}

func (s *Topom) refillCache() error {
	for i := s.cache.hooks.Len(); i != 0; i-- {
		e := s.cache.hooks.Front()
		s.cache.hooks.Remove(e).(func())()
	}
	if slots, err := s.refillCacheSlots(s.cache.slots); err != nil {
		log.ErrorErrorf(err, "store: load slots failed")
		return errors.Errorf("store: load slots failed")
	} else {
		s.cache.slots = slots
	}
	if group, err := s.refillCacheGroup(s.cache.group); err != nil {
		log.ErrorErrorf(err, "store: load group failed")
		return errors.Errorf("store: load group failed")
	} else {
		s.cache.group = group
	}
	if proxy, err := s.refillCacheProxy(s.cache.proxy); err != nil {
		log.ErrorErrorf(err, "store: load proxy failed")
		return errors.Errorf("store: load proxy failed")
	} else {
		s.cache.proxy = proxy
	}
	if sentinel, err := s.refillCacheSentinel(s.cache.sentinel); err != nil {
		log.ErrorErrorf(err, "store: load sentinel failed")
		return errors.Errorf("store: load sentinel failed")
	} else {
		s.cache.sentinel = sentinel
	}
	return nil
}

func (s *Topom) refillCacheSlots(slots []*models.SlotMapping) ([]*models.SlotMapping, error) {
	if slots == nil {
		return s.store.SlotMappings()
	}
	for i, _ := range slots {
		if slots[i] != nil {
			continue
		}
		m, err := s.store.LoadSlotMapping(i, false)
		if err != nil {
			return nil, err
		}
		if m != nil {
			slots[i] = m
		} else {
			slots[i] = &models.SlotMapping{Id: i}
		}
	}
	return slots, nil
}

func (s *Topom) refillCacheGroup(group map[int]*models.Group) (map[int]*models.Group, error) {
	if group == nil {
		return s.store.ListGroup()
	}
	for i, _ := range group {
		if group[i] != nil {
			continue
		}
		g, err := s.store.LoadGroup(i, false)
		if err != nil {
			return nil, err
		}
		if g != nil {
			group[i] = g
		} else {
			delete(group, i)
		}
	}
	return group, nil
}

func (s *Topom) refillCacheProxy(proxy map[string]*models.Proxy) (map[string]*models.Proxy, error) {
	if proxy == nil {
		return s.store.ListProxy()
	}
	for t, _ := range proxy {
		if proxy[t] != nil {
			continue
		}
		p, err := s.store.LoadProxy(t, false)
		if err != nil {
			return nil, err
		}
		if p != nil {
			proxy[t] = p
		} else {
			delete(proxy, t)
		}
	}
	return proxy, nil
}

func (s *Topom) refillCacheSentinel(sentinel *models.Sentinel) (*models.Sentinel, error) {
	if sentinel != nil {
		return sentinel, nil
	}
	p, err := s.store.LoadSentinel(false)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
	}
	return &models.Sentinel{}, nil
}

func (s *Topom) storeUpdateSlotMapping(m *models.SlotMapping) error {
	log.Warnf("update slot-[%d]:\n%s", m.Id, m.Encode())
	if err := s.store.UpdateSlotMapping(m); err != nil {
		log.ErrorErrorf(err, "store: update slot-[%d] failed", m.Id)
		return errors.Errorf("store: update slot-[%d] failed", m.Id)
	}
	return nil
}

func (s *Topom) storeCreateGroup(g *models.Group) error {
	log.Warnf("create group-[%d]:\n%s", g.Id, g.Encode())
	if err := s.store.UpdateGroup(g); err != nil {
		log.ErrorErrorf(err, "store: create group-[%d] failed", g.Id)
		return errors.Errorf("store: create group-[%d] failed", g.Id)
	}
	return nil
}

func (s *Topom) storeUpdateGroup(g *models.Group) error {
	log.Warnf("update group-[%d]:\n%s", g.Id, g.Encode())
	if err := s.store.UpdateGroup(g); err != nil {
		log.ErrorErrorf(err, "store: update group-[%d] failed", g.Id)
		return errors.Errorf("store: update group-[%d] failed", g.Id)
	}
	return nil
}

func (s *Topom) storeRemoveGroup(g *models.Group) error {
	log.Warnf("remove group-[%d]:\n%s", g.Id, g.Encode())
	if err := s.store.DeleteGroup(g.Id); err != nil {
		log.ErrorErrorf(err, "store: remove group-[%d] failed", g.Id)
		return errors.Errorf("store: remove group-[%d] failed", g.Id)
	}
	return nil
}

func (s *Topom) storeCreateProxy(p *models.Proxy) error {
	log.Warnf("create proxy-[%s]:\n%s", p.Token, p.Encode())
	if err := s.store.UpdateProxy(p); err != nil {
		log.ErrorErrorf(err, "store: create proxy-[%s] failed", p.Token)
		return errors.Errorf("store: create proxy-[%s] failed", p.Token)
	}
	return nil
}

func (s *Topom) storeUpdateProxy(p *models.Proxy) error {
	log.Warnf("update proxy-[%s]:\n%s", p.Token, p.Encode())
	if err := s.store.UpdateProxy(p); err != nil {
		log.ErrorErrorf(err, "store: update proxy-[%s] failed", p.Token)
		return errors.Errorf("store: update proxy-[%s] failed", p.Token)
	}
	return nil
}

func (s *Topom) storeRemoveProxy(p *models.Proxy) error {
	log.Warnf("remove proxy-[%s]:\n%s", p.Token, p.Encode())
	if err := s.store.DeleteProxy(p.Token); err != nil {
		log.ErrorErrorf(err, "store: remove proxy-[%s] failed", p.Token)
		return errors.Errorf("store: remove proxy-[%s] failed", p.Token)
	}
	return nil
}

func (s *Topom) storeUpdateSentinel(p *models.Sentinel) error {
	log.Warnf("update sentinel:\n%s", p.Encode())
	if err := s.store.UpdateSentinel(p); err != nil {
		log.ErrorErrorf(err, "store: update sentinel failed")
		return errors.Errorf("store: update sentinel failed")
	}
	return nil
}
