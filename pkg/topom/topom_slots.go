// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func (s *Topom) GetSlots() ([]*models.Slot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx, err := s.newContext()
	if err != nil {
		return nil, err
	}
	return ctx.toSlotList(ctx.slots, false), nil
}

func (s *Topom) SlotCreateAction(sid int, gid int) error {
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
	if len(g.Servers) == 0 {
		return errors.Errorf("group-[%d] is empty", gid)
	}

	m, err := ctx.getSlotMapping(sid)
	if err != nil {
		return err
	}
	if m.Action.State != models.ActionNothing {
		return errors.Errorf("action of slot-[%d] already exists", sid)
	}
	if m.GroupId == gid {
		return errors.Errorf("slot-[%d] already in group-[%d]", sid, gid)
	}

	m.Action.State = models.ActionPending
	m.Action.Index = ctx.maxSlotActionIndex() + 1
	m.Action.TargetId = gid

	if err := s.store.UpdateSlotMapping(m); err != nil {
		log.ErrorErrorf(err, "store: update slot-[%d] failed", sid)
		return errors.Errorf("store: update slot-[%d] failed", sid)
	}

	log.Infof("update slot-[%d]:\n%s", sid, m.Encode())

	select {
	default:
	case s.slotaction.notify <- true:
	}

	return nil
}

func (s *Topom) SlotRemoveAction(sid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	m, err := ctx.getSlotMapping(sid)
	if err != nil {
		return err
	}
	if m.Action.State == models.ActionNothing {
		return errors.Errorf("action of slot-[%d] doesn't exist", sid)
	}
	if m.Action.State != models.ActionPending {
		return errors.Errorf("action of slot-[%d] cannot be removed", sid)
	}

	m = &models.SlotMapping{
		Id:      m.Id,
		GroupId: m.GroupId,
	}

	if err := s.store.UpdateSlotMapping(m); err != nil {
		log.ErrorErrorf(err, "store: update slot-[%d] failed", sid)
		return errors.Errorf("store: update slot-[%d] failed", sid)
	}

	log.Infof("update slot-[%d]:\n%s", sid, m.Encode())

	return nil
}

func (s *Topom) SlotActionPrepare(sid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	m, err := ctx.getSlotMapping(sid)
	if err != nil {
		return err
	}
	if m.Action.State == models.ActionNothing {
		return nil
	}

	log.Infof("prepare action of slot-[%d]\n%s", sid, m.Encode())

	switch m.Action.State {

	case models.ActionPending:

		m.Action.State = models.ActionPreparing

		if err := s.store.UpdateSlotMapping(m); err != nil {
			log.ErrorErrorf(err, "store: update slot-[%d] failed", sid)
			return errors.Errorf("store: update slot-[%d] failed", sid)
		}

		log.Infof("update slot-[%d]:\n%s", sid, m.Encode())

		fallthrough

	case models.ActionPreparing:

		onForwardError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync slot-[%d] to prepared failed", p.Token, sid)
		}
		onRollbackError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync-rollback slot-[%d] to preparing failed", p.Token, sid)
		}

		if err := ctx.resyncSlots(onForwardError, ctx.toSlot(m, true)); err != nil {
			log.Warnf("resync slot-[%d] to prepared failed, try to rollback", sid)
			ctx.resyncSlots(onRollbackError, ctx.toSlot(m, false))
			log.Warnf("resync slot-[%d] to preparing, rollback finished", sid)
			return err
		}

		m.Action.State = models.ActionPrepared

		if err := s.store.UpdateSlotMapping(m); err != nil {
			log.ErrorErrorf(err, "store: update slot-[%d] failed", sid)
			return errors.Errorf("store: update slot-[%d] failed", sid)
		}

		log.Infof("update slot-[%d]:\n%s", sid, m.Encode())

		fallthrough

	case models.ActionPrepared:

		m.Action.State = models.ActionMigrating

		if err := s.store.UpdateSlotMapping(m); err != nil {
			log.ErrorErrorf(err, "store: update slot-[%d] failed", sid)
			return errors.Errorf("store: update slot-[%d] failed", sid)
		}

		log.Infof("update slot-[%d]:\n%s", sid, m.Encode())

		fallthrough

	case models.ActionMigrating:

		onForwardError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync slot-[%d] to migrating failed", p.Token, sid)
		}

		if err := ctx.resyncSlots(onForwardError, ctx.toSlot(m, false)); err != nil {
			log.Warnf("resync slot-[%d] to migrating failed", sid)
			return err
		}

		log.Infof("update slot-[%d]:\n%s", sid, m.Encode())

		return nil

	case models.ActionFinished:

		return nil

	default:

		log.Panicf("invalid state of slot-[%d] = %s", sid, m.Encode())

		return nil

	}
}

func (s *Topom) SlotActionComplete(sid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	m, err := ctx.getSlotMapping(sid)
	if err != nil {
		return err
	}
	if m.Action.State == models.ActionNothing {
		return nil
	}

	log.Infof("complete action of slot-[%d]\n%s", sid, m.Encode())

	switch m.Action.State {

	case models.ActionPending, models.ActionPreparing, models.ActionPrepared:

		return errors.Errorf("action of slot-[%d] is not migrating", sid)

	case models.ActionMigrating:

		m.Action.State = models.ActionFinished

		if err := s.store.UpdateSlotMapping(m); err != nil {
			log.ErrorErrorf(err, "store: update slot-[%d] failed", sid)
			return errors.Errorf("store: update slot-[%d] failed", sid)
		}

		log.Infof("update slot-[%d]:\n%s", sid, m.Encode())

		fallthrough

	case models.ActionFinished:

		onForwardError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync slot-[%d] to finished failed", p.Token, sid)
		}

		if err := ctx.resyncSlots(onForwardError, ctx.toSlot(m, false)); err != nil {
			log.Warnf("resync slot-[%d] to finished failed", sid)
			return err
		}

		m = &models.SlotMapping{
			Id:      m.Id,
			GroupId: m.Action.TargetId,
		}

		if err := s.store.UpdateSlotMapping(m); err != nil {
			log.ErrorErrorf(err, "store: update slot-[%d] failed", sid)
			return errors.Errorf("store: update slot-[%d] failed", sid)
		}

		log.Infof("update slot-[%d]:\n%s", sid, m.Encode())

		return nil

	default:

		log.Panicf("invalid state of slot-[%d] = %s", sid, m.Encode())

		return nil

	}
}
