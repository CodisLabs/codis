package topom

import (
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func (s *Topom) GetSlotMappings() []*models.SlotMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	return nil, errors.New("invalid slot id")
}

func (s *Topom) isGroupLocked(groupId int) bool {
	if g := s.groups[groupId]; g != nil {
		return g.Promoting
	}
	return false
}

func (s *Topom) isSlotLocked(m *models.SlotMapping) bool {
	switch m.Action.State {
	case models.ActionNothing:
		fallthrough
	case models.ActionPending:
		return s.isGroupLocked(m.GroupId)
	case models.ActionPreparing:
		return true
	case models.ActionMigrating:
		return s.isGroupLocked(m.GroupId) || s.isGroupLocked(m.Action.TargetId)
	}
	return false
}

func (s *Topom) isSlotInGroup(m *models.SlotMapping, groupId int) bool {
	switch m.Action.State {
	case models.ActionNothing:
		fallthrough
	case models.ActionPending:
		return m.GroupId == groupId
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
		if s.isSlotInGroup(m, groupId) {
			slots = append(slots, s.toSlotState(m))
		}
	}
	return slots
}

func (s *Topom) maxActionIndex() int {
	var maxIndex int
	for _, m := range s.mappings {
		if m.Action.State != models.ActionNothing {
			if m.Action.Index > maxIndex {
				maxIndex = m.Action.Index
			}
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

	g, err := s.getGroup(targetId)
	if err != nil {
		return err
	}
	if len(g.Servers) == 0 {
		return errors.New("group is empty")
	}

	m, err := s.getSlotMapping(slotId)
	if err != nil {
		return err
	}
	if m.Action.State != models.ActionNothing {
		return errors.New("slot already has action")
	}

	n := &models.SlotMapping{
		Id:      slotId,
		GroupId: m.GroupId,
	}
	n.Action.State = models.ActionPending
	n.Action.Index = s.maxActionIndex() + 1
	n.Action.TargetId = targetId

	if err := s.store.SaveSlotMapping(slotId, n); err != nil {
		log.WarnErrorf(err, "slot-[%d] update failed", slotId)
		return errors.New("slot update failed")
	}

	log.Infof("[%p] update slot-[%d]: \n%s", s, slotId, n.ToJson())

	s.mappings[slotId] = n
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

	if m.Action.State != models.ActionPending {
		return errors.New("slot state is not pending")
	}

	n := &models.SlotMapping{
		Id:      slotId,
		GroupId: m.GroupId,
	}
	if err := s.store.SaveSlotMapping(slotId, n); err != nil {
		log.WarnErrorf(err, "slot-[%d] update failed", slotId)
		return errors.New("slot update failed")
	}

	log.Infof("[%p] update slot-[%d]: \n%s", s, slotId, n.ToJson())

	s.mappings[slotId] = n
	return nil
}
