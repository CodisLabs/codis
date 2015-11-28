// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func (s *Topom) GetSlots() ([]*models.Slot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return nil, err
	}
	return ctx.toSlotSlice(ctx.slots, false), nil
}

func (s *Topom) SlotCreateAction(sid int, gid int) error {
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
	if m.Action.State != models.ActionNothing {
		return errors.Errorf("slot-[%d] action already exists", sid)
	}
	if m.GroupId == gid {
		return errors.Errorf("slot-[%d] already in group-[%d]", sid, gid)
	}

	g, err := ctx.getGroup(gid)
	if err != nil {
		return err
	}
	if len(g.Servers) == 0 {
		return errors.Errorf("group-[%d] is empty", gid)
	}

	s.dirtySlotsCache(sid)

	m.Action.State = models.ActionPending
	m.Action.Index = ctx.maxSlotActionIndex() + 1
	m.Action.TargetId = gid

	return s.storeUpdateSlotMapping(m)
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
	if m.Action.State != models.ActionPending {
		return errors.Errorf("slot-[%d] action can't be removed", sid)
	}

	s.dirtySlotsCache(sid)

	m = &models.SlotMapping{
		Id:      m.Id,
		GroupId: m.GroupId,
	}

	return s.storeUpdateSlotMapping(m)
}

func (s *Topom) SlotActionPrepare() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return -1, err
	}

	m := ctx.minSlotActionIndex()
	if m == nil {
		return -1, nil
	}

	log.Infof("slot-[%d] action prepare:\n%s", m.Id, m.Encode())

	switch m.Action.State {

	case models.ActionPending:

		s.dirtySlotsCache(m.Id)

		m.Action.State = models.ActionPreparing

		if err := s.storeUpdateSlotMapping(m); err != nil {
			return -1, err
		}

		fallthrough

	case models.ActionPreparing:

		onForwardError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync slot-[%d] to prepared failed", p.Token, m.Id)
		}
		onRollbackError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync-rollback slot-[%d] to preparing failed", p.Token, m.Id)
		}

		if err := ctx.resyncSlots(onForwardError, ctx.toSlot(m, true)); err != nil {
			log.Warnf("resync slot-[%d] to prepared failed, rollback", m.Id)
			ctx.resyncSlots(onRollbackError, ctx.toSlot(m, false))
			log.Warnf("resync-rollback slot-[%d] to preparing finished", m.Id)
			return -1, err
		}

		s.dirtySlotsCache(m.Id)

		m.Action.State = models.ActionPrepared

		if err := s.storeUpdateSlotMapping(m); err != nil {
			return -1, err
		}

		fallthrough

	case models.ActionPrepared:

		s.dirtySlotsCache(m.Id)

		m.Action.State = models.ActionMigrating

		if err := s.storeUpdateSlotMapping(m); err != nil {
			return -1, err
		}

		fallthrough

	case models.ActionMigrating:

		onForwardError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync slot-[%d] to migrating failed", p.Token, m.Id)
		}

		if err := ctx.resyncSlots(onForwardError, ctx.toSlot(m, false)); err != nil {
			log.Warnf("resync slot-[%d] to migrating failed", m.Id)
			return -1, err
		}

		return m.Id, nil

	case models.ActionFinished:

		return m.Id, nil

	default:

		return -1, errors.Errorf("slot-[%d] action state is invalid", m.Id)

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

	log.Infof("slot-[%d] action complete:\n%s", sid, m.Encode())

	switch m.Action.State {

	case models.ActionMigrating:

		s.dirtySlotsCache(sid)

		m.Action.State = models.ActionFinished

		if err := s.storeUpdateSlotMapping(m); err != nil {
			return err
		}

		fallthrough

	case models.ActionFinished:

		onForwardError := func(p *models.Proxy, err error) {
			log.WarnErrorf(err, "proxy-[%s] resync slot-[%d] to finished failed", p.Token, sid)
		}

		if err := ctx.resyncSlots(onForwardError, ctx.toSlot(m, false)); err != nil {
			log.Warnf("resync slot-[%d] to finished failed", sid)
			return err
		}

		s.dirtySlotsCache(sid)

		m = &models.SlotMapping{
			Id:      m.Id,
			GroupId: m.Action.TargetId,
		}

		if err := s.storeUpdateSlotMapping(m); err != nil {
			return err
		}

		return nil

	default:

		return errors.Errorf("slot-[%d] action state is invalid", sid)

	}
}
