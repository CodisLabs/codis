package topom

import (
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/errors"
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
