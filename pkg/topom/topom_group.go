// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"net"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func (s *Topom) CreateGroup(gid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	if gid <= 0 || gid > models.MaxGroupId {
		return errors.Errorf("invalid group id = %d, out of range", gid)
	}
	if ctx.group[gid] != nil {
		return errors.Errorf("group-[%d] already exists", gid)
	}

	s.dirtyGroupCache(gid)

	g := &models.Group{
		Id:      gid,
		Servers: []*models.GroupServer{},
	}

	return s.storeCreateGroup(g)
}

func (s *Topom) RemoveGroup(gid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	g, err := ctx.getGroup(gid)
	if err != nil {
		return err
	}
	if len(g.Servers) != 0 {
		return errors.Errorf("group-[%d] isn't empty", gid)
	}

	s.dirtyGroupCache(gid)

	return s.storeRemoveGroup(g)
}

func (s *Topom) GroupAddServer(gid int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	if _, _, err := net.SplitHostPort(addr); err != nil {
		return errors.Errorf("invalid server address: %s", err)
	}
	if g, _, _ := ctx.getGroupByServer(addr); g != nil {
		return errors.Errorf("server-[%s] already exists", addr)
	}

	g, err := ctx.getGroup(gid)
	if err != nil {
		return err
	}
	if g.Promoting.State != models.ActionNothing {
		return errors.Errorf("group-[%d] is promoting", gid)
	}

	s.dirtyGroupCache(gid)

	g.Servers = append(g.Servers, &models.GroupServer{Addr: addr})

	return s.storeUpdateGroup(g)
}

func (s *Topom) GroupDelServer(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	g, index, err := ctx.getGroupByServer(addr)
	if err != nil {
		return err
	}
	if g.Promoting.State != models.ActionNothing {
		return errors.Errorf("group-[%d] is promoting", g.Id)
	}

	if index == 0 {
		if len(g.Servers) != 1 || ctx.isGroupInUse(g.Id) {
			return errors.Errorf("group-[%d] can't remove master, still in use", g.Id)
		}
	}
	if g.Servers[index].Action.State != models.ActionNothing {
		return errors.Errorf("server-[%s] action is not empty", addr)
	}

	var slice = make([]*models.GroupServer, 0, len(g.Servers))
	for i, x := range g.Servers {
		if i != index {
			slice = append(slice, x)
		}
	}

	s.dirtyGroupCache(g.Id)

	g.Servers = slice

	return s.storeUpdateGroup(g)
}

func (s *Topom) GroupPromoteServer(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	g, index, err := ctx.getGroupByServer(addr)
	if err != nil {
		return err
	}
	if g.Promoting.State != models.ActionNothing {
		return errors.Errorf("group-[%d] is promoting", g.Id)
	}

	if index == 0 {
		return errors.Errorf("group-[%d] can't promote master", g.Id)
	}
	for _, x := range g.Servers {
		if x.Action.State != models.ActionNothing {
			return errors.Errorf("server-[%s] action is not empty", x.Addr)
		}
	}

	if n := s.action.executor.Get(); n != 0 {
		return errors.Errorf("slots-migration is running = %d", n)
	}

	s.dirtyGroupCache(g.Id)

	g.Promoting.Index = index
	g.Promoting.State = models.ActionPreparing

	return s.storeUpdateGroup(g)
}

func (s *Topom) GroupPromoteCommit(gid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	g, err := ctx.getGroup(gid)
	if err != nil {
		return err
	}
	if g.Promoting.State == models.ActionNothing {
		return nil
	}

	switch g.Promoting.State {

	case models.ActionPreparing:

		onForwardError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync group-[%d] to prepared failed", p.Token, gid)
		}
		onRollbackError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync-rollback group-[%d] to preparing failed", p.Token, gid)
		}

		slots := ctx.getSlotMappingByGroupId(gid)

		if err := ctx.resyncSlots(onForwardError, ctx.toSlotSlice(slots, true)...); err != nil {
			log.Warnf("resync group-[%d] to prepared failed, rollback", gid)
			ctx.resyncSlots(onRollbackError, ctx.toSlotSlice(slots, false)...)
			log.Warnf("resync-rollback group-[%d] to preparing finished", gid)
			return err
		}

		s.dirtyGroupCache(gid)

		g.Promoting.State = models.ActionPrepared

		if err := s.storeUpdateGroup(g); err != nil {
			return err
		}

		fallthrough

	case models.ActionPrepared:

		var index = g.Promoting.Index
		var slice = make([]*models.GroupServer, 0, len(g.Servers))
		for i, x := range g.Servers {
			if i != index {
				slice = append(slice, x)
			}
		}
		slice[0] = &models.GroupServer{Addr: g.Servers[index].Addr}

		s.dirtyGroupCache(gid)

		g.Servers = slice
		g.Promoting.Index = 0
		g.Promoting.State = models.ActionFinished

		if err := s.storeUpdateGroup(g); err != nil {
			return err
		}

		fallthrough

	case models.ActionFinished:

		var master = g.Servers[0].Addr
		if c, err := NewRedisClient(master, s.config.ProductAuth, time.Second); err != nil {
			log.WarnErrorf(err, "create redis client to %s failed", master)
		} else {
			if err := c.SetMaster(""); err != nil {
				log.WarnErrorf(err, "redis %s set master to NO:ONE failed", master)
			}
			c.Close()
		}

		onForwardError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync group-[%d] to finished failed", p.Token, gid)
		}

		slots := ctx.getSlotMappingByGroupId(gid)

		if err := ctx.resyncSlots(onForwardError, ctx.toSlotSlice(slots, false)...); err != nil {
			log.Warnf("resync group-[%d] to finished failed", gid)
			return err
		}

		s.dirtyGroupCache(gid)

		g = &models.Group{
			Id:      g.Id,
			Servers: g.Servers,
		}

		if err := s.storeUpdateGroup(g); err != nil {
			return err
		}

		return nil

	default:

		log.Panicf("invalid state of group-[%d] = %s", gid, g.Encode())

		return nil

	}
}

func (s *Topom) GroupCreateSyncAction(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	g, index, err := ctx.getGroupByServer(addr)
	if err != nil {
		return err
	}
	if g.Servers[index].Action.State != models.ActionNothing {
		return errors.Errorf("server-[%s] action is not empty", addr)
	}

	s.dirtyGroupCache(g.Id)

	g.Servers[index].Action.Index = ctx.maxGroupSyncActionIndex() + 1
	g.Servers[index].Action.State = models.ActionPending

	return s.storeUpdateGroup(g)
}

func (s *Topom) GroupRemoveSyncAction(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	g, index, err := ctx.getGroupByServer(addr)
	if err != nil {
		return err
	}
	if g.Servers[index].Action.State != models.ActionPending {
		return errors.Errorf("server-[%d] action can't be removed", addr)
	}

	s.dirtyGroupCache(g.Id)

	g.Servers[index] = &models.GroupServer{Addr: addr}

	return s.storeUpdateGroup(g)
}

func (s *Topom) GroupSyncActionPrepare(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	g, index, err := ctx.getGroupByServer(addr)
	if err != nil {
		return err
	}
	if g.Servers[index].Action.State == models.ActionNothing {
		return nil
	}

	log.Infof("server-[%s] action prepare:\n%s", addr, g.Encode())

	switch g.Servers[index].Action.State {

	case models.ActionPending:

		s.dirtyGroupCache(g.Id)

		g.Servers[index].Action.State = models.ActionSyncing

		if err := s.storeUpdateGroup(g); err != nil {
			return err
		}

		return nil

	case models.ActionSyncing:

		return nil

	default:

		log.Panicf("invalid server-[%s] action state:\n%s", addr, g.Encode())

		return nil

	}
}

func (s *Topom) GroupSyncActionComplete(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	g, index, err := ctx.getGroupByServer(addr)
	if err != nil {
		return err
	}
	if g.Servers[index].Action.State == models.ActionNothing {
		return nil
	}

	log.Infof("server-[%s] action complete:\n%s", addr, g.Encode())

	switch g.Servers[index].Action.State {

	case models.ActionPending:

		return errors.Errorf("action of server-[%s] is not syncing", addr)

	case models.ActionSyncing:

		s.dirtyGroupCache(g.Id)

		g.Servers[index] = &models.GroupServer{Addr: addr}

		if err := s.storeUpdateGroup(g); err != nil {
			return err
		}

		return nil

	default:

		log.Panicf("invalid server-[%s] action state:\n%s", addr, g.Encode())

		return nil

	}
}
