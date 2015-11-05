// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func (s *Topom) GetSlotMappings() []*models.SlotMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getSlotMappings()
}

func (s *Topom) GetSlots() []*models.Slot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getSlots()
}

func (s *Topom) getSlotMappings() []*models.SlotMapping {
	mappings := make([]*models.SlotMapping, len(s.mappings))
	for i, m := range s.mappings {
		mappings[i] = m
	}
	return mappings
}

func (s *Topom) getSlotMapping(slotId int) (*models.SlotMapping, error) {
	if slotId >= 0 && slotId < len(s.mappings) {
		return s.mappings[slotId], nil
	}
	return nil, errors.Errorf("invalid slot id, out of range")
}

func (s *Topom) isSlotLocked(m *models.SlotMapping) bool {
	switch m.Action.State {
	case models.ActionNothing:
		fallthrough
	case models.ActionPending:
		return s.isGroupPromoting(m.GroupId)
	case models.ActionPreparing:
		return true
	case models.ActionMigrating:
		return s.isGroupPromoting(m.GroupId) || s.isGroupPromoting(m.Action.TargetId)
	}
	return false
}

func (s *Topom) isSlotUseGroup(m *models.SlotMapping, groupId int) bool {
	switch m.Action.State {
	case models.ActionNothing:
		return m.GroupId == groupId
	case models.ActionPending:
		fallthrough
	case models.ActionPreparing:
		fallthrough
	case models.ActionMigrating:
		return m.GroupId == groupId || m.Action.TargetId == groupId
	}
	return false
}

func (s *Topom) toSlotState(m *models.SlotMapping) *models.Slot {
	slot := &models.Slot{
		Id:     m.Id,
		Locked: s.isSlotLocked(m),
	}
	switch m.Action.State {
	case models.ActionNothing:
		fallthrough
	case models.ActionPending:
		slot.BackendAddr = s.getGroupMaster(m.GroupId)
	case models.ActionPreparing:
		fallthrough
	case models.ActionMigrating:
		slot.BackendAddr = s.getGroupMaster(m.Action.TargetId)
		slot.MigrateFrom = s.getGroupMaster(m.GroupId)
	}
	return slot
}

func (s *Topom) getSlots() []*models.Slot {
	slots := make([]*models.Slot, 0, len(s.mappings))
	for _, m := range s.mappings {
		slots = append(slots, s.toSlotState(m))
	}
	return slots
}

func (s *Topom) getSlotsByGroup(groupId int) []*models.Slot {
	slots := make([]*models.Slot, 0, len(s.mappings))
	for _, m := range s.mappings {
		if s.isSlotUseGroup(m, groupId) {
			slots = append(slots, s.toSlotState(m))
		}
	}
	return slots
}

func (s *Topom) maxActionIndex() int {
	var maxIndex int
	for _, m := range s.mappings {
		if m.Action.State != models.ActionNothing {
			maxIndex = utils.MaxInt(maxIndex, m.Action.Index)
		}
	}
	return maxIndex
}

func (s *Topom) SlotCreateAction(slotId int, targetId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	m, err := s.getSlotMapping(slotId)
	if err != nil {
		return err
	}
	if m.Action.State != models.ActionNothing {
		return errors.Errorf("action of slot-[%d] already exists", slotId)
	}

	g, err := s.getGroup(targetId)
	if err != nil {
		return err
	}
	if len(g.Servers) == 0 {
		return errors.Errorf("group-[%d] is empty", targetId)
	}
	if m.GroupId == targetId {
		return errors.Errorf("slot-[%d] already in group-[%d]", slotId, targetId)
	}

	n := &models.SlotMapping{
		Id:      slotId,
		GroupId: m.GroupId,
	}
	n.Action.State = models.ActionPending
	n.Action.Index = s.maxActionIndex() + 1
	n.Action.TargetId = targetId

	if err := s.store.SaveSlotMapping(slotId, n); err != nil {
		log.ErrorErrorf(err, "[%p] update slot-[%d] failed", s, slotId)
		return errors.Errorf("store: update slot-[%d] failed", slotId)
	}

	s.mappings[slotId] = n

	log.Infof("[%p] update slot-[%d]:\n%s", s, slotId, n.Encode())

	select {
	case s.action.notify <- true:
	default:
	}

	return nil
}

func (s *Topom) SlotRemoveAction(slotId int) error {
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
		return errors.Errorf("action of slot-[%d] is empty", slotId)
	}
	if m.Action.State != models.ActionPending {
		return errors.Errorf("action of slot-[%d] is not pending", slotId)
	}

	n := &models.SlotMapping{
		Id:      slotId,
		GroupId: m.GroupId,
	}
	if err := s.store.SaveSlotMapping(slotId, n); err != nil {
		log.ErrorErrorf(err, "[%p] update slot-[%d] failed", s, slotId)
		return errors.Errorf("store: update slot-[%d] failed", slotId)
	}

	s.mappings[slotId] = n

	log.Infof("[%p] update slot-[%d]:\n%s", s, slotId, n.Encode())

	return nil
}

func (s *Topom) resyncSlotMapping(slotId int) error {
	m, err := s.getSlotMapping(slotId)
	if err != nil {
		return err
	}
	slot := s.toSlotState(m)
	errs := s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
		if err := c.FillSlots(slot); err != nil {
			log.WarnErrorf(err, "[%p] proxy-[%s] resync slot-[%d] failed", s, p.Token, slotId)
			return err
		}
		return nil
	})
	for t, err := range errs {
		if err != nil {
			return errors.Errorf("proxy-[%s] resync slot-[%d] failed", t, slotId)
		}
	}
	return nil
}
