// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func (s *Topom) ProcessAction(slotId int) (err error) {
	defer func() {
		if err != nil {
			s.action.progress.failed.Set(true)
		} else {
			s.action.progress.remain.Set(0)
			s.action.progress.failed.Set(false)
		}
	}()
	if err := s.PrepareAction(slotId); err != nil {
		return err
	}
	for {
		for s.GetActionDisabled() {
			time.Sleep(time.Millisecond * 10)
		}
		n, err := s.MigrateSlot(slotId)
		if err != nil {
			return err
		}
		switch {
		case n > 0:
			s.action.progress.remain.Set(int64(n))
			s.action.progress.failed.Set(false)
			s.NoopInterval()
		case n < 0:
			time.Sleep(time.Millisecond * 10)
		default:
			return s.CompleteAction(slotId)
		}
	}
}

func (s *Topom) NextActionSlotId() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return -1
	}

	var x *models.SlotMapping
	for _, m := range s.mappings {
		if m.Action.State != models.ActionNothing {
			if x == nil || x.Action.Index > m.Action.Index {
				x = m
			}
		}
	}
	if x == nil {
		return -1
	}
	return x.Id
}

func (s *Topom) NoopInterval() int {
	var ms int
	for !s.IsClosed() {
		if d := s.GetActionInterval() - ms; d <= 0 {
			return ms
		} else {
			d = utils.MinInt(d, 50)
			time.Sleep(time.Millisecond * time.Duration(d))
			ms += d
		}
	}
	return ms
}

func (s *Topom) PrepareAction(slotId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	m, err := s.getSlotMapping(slotId)
	if err != nil {
		return err
	}
	if m.Action.State == models.ActionNothing {
		return errors.Errorf("action of slot-[%d] is nothing", slotId)
	}

	log.Infof("[%p] prepare action of slot-[%d]\n%s", s, slotId, m.Encode())

	switch m.Action.State {
	case models.ActionPending:

		n := &models.SlotMapping{
			Id:      slotId,
			GroupId: m.GroupId,
			Action:  m.Action,
		}
		n.Action.State = models.ActionPreparing

		if err := s.store.SaveSlotMapping(slotId, n); err != nil {
			log.ErrorErrorf(err, "[%p] update slot-[%d] failed", s, slotId)
			return errors.Errorf("store: update slot-[%d] failed", slotId)
		}

		s.mappings[slotId] = n

		log.Infof("[%p] update slot-[%d]:\n%s", s, slotId, n.Encode())

		fallthrough

	case models.ActionPreparing:

		if err := s.resyncSlotMapping(slotId); err != nil {
			return err
		}

		n := &models.SlotMapping{
			Id:      slotId,
			GroupId: m.GroupId,
			Action:  m.Action,
		}
		n.Action.State = models.ActionMigrating

		if err := s.store.SaveSlotMapping(slotId, n); err != nil {
			log.ErrorErrorf(err, "[%p] update slot-[%d] failed", s, slotId)
			return errors.Errorf("store: update slot-[%d] failed", slotId)
		}

		s.mappings[slotId] = n

		log.Infof("[%p] update slot-[%d]:\n%s", s, slotId, n.Encode())

		fallthrough

	case models.ActionMigrating:

		if err := s.resyncSlotMapping(slotId); err != nil {
			return err
		}

	}
	return nil
}

func (s *Topom) CompleteAction(slotId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	m, err := s.getSlotMapping(slotId)
	if err != nil {
		return err
	}
	if m.Action.State != models.ActionMigrating {
		return errors.Errorf("action of slot-[%d] is not migrating", slotId)
	}

	log.Infof("[%p] complete action of slot-[%d]\n%s", s, slotId, m.Encode())

	n := &models.SlotMapping{
		Id:      slotId,
		GroupId: m.Action.TargetId,
	}
	s.mappings[slotId] = n

	var rollback = true
	defer func() {
		if rollback {
			s.mappings[slotId] = m
		}
	}()

	if err := s.resyncSlotMapping(slotId); err != nil {
		return err
	}

	if err := s.store.SaveSlotMapping(slotId, n); err != nil {
		log.ErrorErrorf(err, "[%p] update slot-[%d] failed", s, slotId)
		return errors.Errorf("store: update slot-[%d] failed", slotId)
	}

	rollback = false

	log.Infof("[%p] update slot-[%d]:\n%s", s, slotId, n.Encode())

	return nil
}

type actionTask struct {
	From, Dest struct {
		Master  string
		GroupId int
	}
	Locked bool
}

func (s *Topom) newActionTask(slotId int) (*actionTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosedTopom
	}

	m, err := s.getSlotMapping(slotId)
	if err != nil {
		return nil, err
	}
	if m.Action.State != models.ActionMigrating {
		return nil, errors.Errorf("action of slot-[%d] is not migrating", slotId)
	}

	t := &actionTask{
		Locked: s.isSlotLocked(m),
	}
	t.From.Master = s.getGroupMaster(m.GroupId)
	t.From.GroupId = m.GroupId
	t.Dest.Master = s.getGroupMaster(m.Action.TargetId)
	t.Dest.GroupId = m.Action.TargetId

	s.lockGroupMaster(t.From.GroupId)
	s.lockGroupMaster(t.Dest.GroupId)
	return t, nil
}

func (s *Topom) releaseActionTask(t *actionTask) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.unlockGroupMaster(t.From.GroupId)
	s.unlockGroupMaster(t.Dest.GroupId)
}

func (s *Topom) MigrateSlot(slotId int) (int, error) {
	t, err := s.newActionTask(slotId)
	if err != nil {
		return 0, err
	}
	defer s.releaseActionTask(t)

	if t.Locked {
		return -1, nil
	}
	if t.From.Master == "" {
		return 0, nil
	}

	c, err := s.redisp.GetClient(t.From.Master)
	if err != nil {
		return 0, err
	}
	defer s.redisp.PutClient(c)
	return c.MigrateSlot(slotId, t.Dest.Master)
}
