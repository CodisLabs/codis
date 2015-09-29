package topom

import (
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

func (s *Topom) GetSlotMappings() []*models.SlotMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]*models.SlotMapping{}, s.mappings[:]...)
}

func (s *Topom) getSlotMapping(slotId int) (*models.SlotMapping, error) {
	if slotId >= 0 && slotId < len(s.mappings) {
		return s.mappings[slotId], nil
	}
	return nil, errors.New("invalid slot id")
}

func (s *Topom) toSlotState(m *models.SlotMapping) *models.Slot {
	slot := &models.Slot{
		Id: m.Id,
	}
	switch m.Action.State {
	case models.ActionNothing:
		fallthrough
	case models.ActionPending:
		slot.BackendAddr = s.getGroupMaster(m.GroupId)
	case models.ActionPreparing:
		slot.Locked = true
		fallthrough
	case models.ActionMigrating:
		slot.BackendAddr = s.getGroupMaster(m.Action.TargetId)
		slot.MigrateFrom = s.getGroupMaster(m.GroupId)
	}
	return slot
}
