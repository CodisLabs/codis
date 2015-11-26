// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
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
		return errors.Errorf("invalid group id, out of range")
	}
	if ctx.group[gid] != nil {
		return errors.Errorf("group-[%d] already exists", gid)
	}

	g := &models.Group{
		Id:      gid,
		Servers: []*models.GroupServer{},
	}

	if err := s.store.UpdateGroup(g); err != nil {
		log.ErrorErrorf(err, "store: create group-[%d] failed", gid)
		return errors.Errorf("store: create group-[%d] failed", gid)
	}

	log.Infof("create group-[%d]:\n%s", gid, g.Encode())

	return nil
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

	if err := s.store.DeleteGroup(gid); err != nil {
		log.ErrorErrorf(err, "store: remove group-[%d] failed", gid)
		return errors.Errorf("store: remove group-[%d] failed", gid)
	}

	log.Infof("remove group-[%d]:\n%s", gid, g.Encode())

	return nil
}

func (s *Topom) GroupAddServer(gid int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	if addr == "" {
		return errors.Errorf("invalid server address")
	}
	if ctx.getGroupByServer(addr) != nil {
		return errors.Errorf("server-[%s] already exists", addr)
	}

	g, err := ctx.getGroup(gid)
	if err != nil {
		return err
	}
	if g.Promoting.State != models.ActionNothing {
		return errors.Errorf("group-[%d] is promoting", gid)
	}

	g.Servers = append(g.Servers, &models.GroupServer{Addr: addr})

	if err := s.store.UpdateGroup(g); err != nil {
		log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
		return errors.Errorf("store: update group-[%d] failed", gid)
	}

	log.Infof("update group-[%d]:\n%s", gid, g.Encode())

	return nil
}

func (s *Topom) GroupDelServer(gid int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	if addr == "" {
		return errors.Errorf("invalid server address")
	}

	g, err := ctx.getGroup(gid)
	if err != nil {
		return err
	}
	if g.Promoting.State != models.ActionNothing {
		return errors.Errorf("group-[%d] is promoting", gid)
	}

	var index = g.IndexOfServer(addr)

	switch {
	case index < 0:
		return errors.Errorf("group-[%d] doesn't have server %s", gid, addr)
	case index == 0:
		if len(g.Servers) != 1 || ctx.isGroupIsBusy(gid) {
			return errors.Errorf("group-[%d] cann't remove master, still in use", gid)
		}
	default:
		if g.Servers[index].Action.State != models.ActionNothing {
			return errors.Errorf("action of server-[%s] is not empty", addr)
		}
	}

	var slice = make([]*models.GroupServer, 0, len(g.Servers))
	for i, x := range g.Servers {
		if i != index {
			slice = append(slice, x)
		}
	}
	g.Servers = slice

	if err := s.store.UpdateGroup(g); err != nil {
		log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
		return errors.Errorf("store: update group-[%d] failed", gid)
	}

	log.Infof("update group-[%d]:\n%s", gid, g.Encode())

	return nil
}

func (s *Topom) GroupPromoteServer(gid int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	if addr == "" {
		return errors.Errorf("invalid server address")
	}

	g, err := ctx.getGroup(gid)
	if err != nil {
		return err
	}
	if g.Promoting.State != models.ActionNothing {
		return errors.Errorf("group-[%d] is promoting", gid)
	}

	var index = g.IndexOfServer(addr)

	switch {
	case index < 0:
		return errors.Errorf("group-[%d] doesn't have server %s", gid, addr)
	case index == 0:
		return errors.Errorf("group-[%d] cann't promote master again", gid)
	default:
		for _, x := range g.Servers {
			if x.Action.State != models.ActionNothing {
				return errors.Errorf("action of server-[%s] is not empty", addr)
			}
		}
	}

	if s.slotaction.executor.Get() != 0 {
		return errors.Errorf("slots-migration is running, master may be busy")
	}

	g.Promoting.Index = index
	g.Promoting.State = models.ActionPreparing

	if err := s.store.UpdateGroup(g); err != nil {
		log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
		return errors.Errorf("store: update group-[%d] failed", gid)
	}

	log.Infof("update group-[%d]:\n%s", gid, g.Encode())

	return nil
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

	log.Infof("promote-commit master of group-[%d]\n%s", gid, g.Encode())

	switch g.Promoting.State {

	case models.ActionPreparing:

		onForwardError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync group-[%d] to prepared failed", p.Token, gid)
		}
		onRollbackError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync-rollback group-[%d] to preparing failed", p.Token, gid)
		}

		var mappings = ctx.getSlotMappingByGroupId(gid)

		if err := ctx.resyncSlots(onForwardError, ctx.toSlotList(mappings, true)...); err != nil {
			log.Warnf("resync group-[%d] to prepared failed, try to rollback", gid)
			ctx.resyncSlots(onRollbackError, ctx.toSlotList(mappings, false)...)
			log.Warnf("resync group-[%d] to preparing, rollback finished", gid)
			return err
		}

		g.Promoting.State = models.ActionPrepared

		if err := s.store.UpdateGroup(g); err != nil {
			log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
			return errors.Errorf("store: update group-[%d] failed", gid)
		}

		log.Infof("update group-[%d]:\n%s", gid, g.Encode())

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

		g.Servers = slice
		g.Promoting.Index = 0
		g.Promoting.State = models.ActionFinished

		if err := s.store.UpdateGroup(g); err != nil {
			log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
			return errors.Errorf("store: update group-[%d] failed", gid)
		}

		log.Infof("update group-[%d]:\n%s", gid, g.Encode())

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

		var mappings = ctx.getSlotMappingByGroupId(gid)

		if err := ctx.resyncSlots(onForwardError, ctx.toSlotList(mappings, false)...); err != nil {
			log.Warnf("resync group-[%d] to finished failed", gid)
			return err
		}

		g = &models.Group{
			Id:      g.Id,
			Servers: g.Servers,
		}

		if err := s.store.UpdateGroup(g); err != nil {
			log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
			return errors.Errorf("store: update group-[%d] failed", gid)
		}

		log.Infof("update group-[%d]:\n%s", gid, g.Encode())

		return nil

	default:

		log.Panicf("invalid state of group-[%d] = %s", gid, g.Encode())

		return nil

	}
}

func (s *Topom) GroupCreateSyncAction(gid int, addr string) error {
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

	var index = g.IndexOfServer(addr)

	switch {
	case index < 0:
		return errors.Errorf("group-[%d] doesn't have server %s", gid, addr)
	default:
		if g.Servers[index].Action.State != models.ActionNothing {
			return errors.Errorf("action of server-[%s] already exists", addr)
		}
	}

	g.Servers[index].Action.Index = ctx.maxGroupSyncActionIndex() + 1
	g.Servers[index].Action.State = models.ActionPending

	if err := s.store.UpdateGroup(g); err != nil {
		log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
		return errors.Errorf("store: update group-[%d] failed", gid)
	}

	log.Infof("update group-[%d]:\n%s", gid, g.Encode())

	return nil
}

func (s *Topom) GroupRemoveSyncAction(gid int, addr string) error {
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

	var index = g.IndexOfServer(addr)

	switch {
	case index < 0:
		return errors.Errorf("group-[%d] doesn't have server %s", gid, addr)
	default:
		if g.Servers[index].Action.State != models.ActionPending {
			return errors.Errorf("action of server-[%s] cannot be removed", addr)
		}
	}

	g.Servers[index] = &models.GroupServer{Addr: addr}

	if err := s.store.UpdateGroup(g); err != nil {
		log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
		return errors.Errorf("store: update group-[%d] failed", gid)
	}

	log.Infof("update group-[%d]:\n%s", gid, g.Encode())

	return nil
}

func (s *Topom) GroupSyncActionPrepare(gid int, addr string) error {
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

	var index = g.IndexOfServer(addr)

	switch {
	case index < 0:
		return nil
	default:
		if g.Servers[index].Action.State == models.ActionNothing {
			return nil
		}
	}

	log.Infof("prepare sync action of server-[%s]", addr)

	switch g.Servers[index].Action.State {

	case models.ActionPending:

		g.Servers[index].Action.State = models.ActionSyncing

		if err := s.store.UpdateGroup(g); err != nil {
			log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
			return errors.Errorf("store: update group-[%d] failed", gid)
		}

		log.Infof("update group-[%d]:\n%s", gid, g.Encode())

		fallthrough

	case models.ActionSyncing:

		return nil

	default:

		log.Panicf("invalid state of group-[%d] = %s", gid, g.Encode())

		return nil

	}
}

func (s *Topom) GroupSyncActionComplete(gid int, addr string) error {
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

	var index = g.IndexOfServer(addr)

	switch {
	case index < 0:
		return nil
	default:
		if g.Servers[index].Action.State == models.ActionNothing {
			return nil
		}
	}

	log.Infof("complete sync action of server-[%s]", addr)

	switch g.Servers[index].Action.State {

	case models.ActionPending:

		return errors.Errorf("action of server-[%s] is not syncing", addr)

	case models.ActionSyncing:

		g.Servers[index] = &models.GroupServer{Addr: addr}

		if err := s.store.UpdateGroup(g); err != nil {
			log.ErrorErrorf(err, "store: update group-[%d] failed", gid)
			return errors.Errorf("store: update group-[%d] failed", gid)
		}

		log.Infof("update group-[%d]:\n%s", gid, g.Encode())

		return nil

	default:

		log.Panicf("invalid state of group-[%d] = %s", gid, g.Encode())

		return nil

	}
}
