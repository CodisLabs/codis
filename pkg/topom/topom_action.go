package topom

import (
	"math"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Action struct {
	*Topom
	SlotId int
}

func (s *Topom) NextAction() *Action {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil
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
		return nil
	}
	return &Action{s, x.Id}
}

func (s *Action) Do() error {
	if err := s.PrepareAction(s.SlotId); err != nil {
		return err
	}
	for {
		n, err := s.ProcessAction(s.SlotId)
		if err != nil {
			return err
		}
		if n == 0 {
			return s.CompleteAction(s.SlotId)
		} else {
			s.NoopInterval()
		}
	}
}

func (s *Action) NoopInterval() {
	var ms int
	for {
		if d := int(s.intvl.Get()) - ms; d <= 0 {
			return
		} else {
			d = utils.MinInt(d, 50)
			time.Sleep(time.Millisecond * time.Duration(d))
			select {
			case <-s.exit.C:
				return
			default:
				ms += d
			}
		}
	}
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
		return errors.Trace(ErrActionNotExists)
	}

	log.Infof("[%p] prepare action on slot-[%d]\n%s", s, slotId, m.Encode())

	switch m.Action.State {
	case models.ActionPending:

		n := &models.SlotMapping{
			Id:      slotId,
			GroupId: m.GroupId,
			Action:  m.Action,
		}
		n.Action.State = models.ActionPreparing

		if err := s.store.SaveSlotMapping(slotId, n); err != nil {
			log.ErrorErrorf(err, "[%p] slot-[%d] update failed", s, slotId)
			return errors.Trace(ErrUpdateStore)
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
			log.ErrorErrorf(err, "[%p] slot-[%d] update failed", s, slotId)
			return errors.Trace(ErrUpdateStore)
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

func (s *Topom) ProcessAction(slotId int) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return 0, ErrClosedTopom
	}

	m, err := s.getSlotMapping(slotId)
	if err != nil {
		return 0, err
	}
	if m.Action.State != models.ActionMigrating {
		return 0, errors.Trace(ErrActionIsNotMigrating)
	}

	if s.isSlotLocked(m) {
		return int(math.MaxInt32), nil
	}

	master := s.getGroupMaster(m.GroupId)
	if master == "" {
		return 0, nil
	}

	c, err := s.redisp.GetClient(master)
	if err != nil {
		return 0, err
	}
	defer s.redisp.PutClient(c)

	return c.MigrateSlot(slotId, s.getGroupMaster(m.Action.TargetId))
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
		return errors.Trace(ErrActionIsNotMigrating)
	}

	log.Infof("[%p] complete action on slot-[%d]\n%s", s, slotId, m.Encode())

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
		log.ErrorErrorf(err, "[%p] slot-[%d] update failed", s, slotId)
		return errors.Trace(ErrUpdateStore)
	}

	rollback = false

	log.Infof("[%p] update slot-[%d]:\n%s", s, slotId, n.Encode())

	return nil
}
