package topom

import (
	"net"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils"
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

func (s *Topom) resyncSlotMapping(slotId int) error {
	m, err := s.getSlotMapping(slotId)
	if err != nil {
		return err
	}
	slot := s.toSlotState(m)
	errs := s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
		if err := c.FillSlots(slot); err != nil {
			log.WarnErrorf(err, "proxy-[%s] resync slot-[%d] failed", p.Token, m.Id)
			return errors.New("proxy resync slot failed")
		}
		return nil
	})
	if len(errs) != 0 {
		return errors.New("resync slot mapping failed")
	}
	return nil
}

func (s *Topom) migrationPrepare(slotId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	m, err := s.getSlotMapping(slotId)
	if err != nil {
		return err
	}
	switch m.Action.State {
	default:
		return errors.New("invalid action state")
	case models.ActionPreparing:
		return s.resyncSlotMapping(slotId)
	case models.ActionPending:
	}

	n := &models.SlotMapping{
		Id:      slotId,
		GroupId: m.GroupId,
		Action:  m.Action,
	}
	n.Action.State = models.ActionPreparing

	if err := s.store.SaveSlotMapping(slotId, n); err != nil {
		return err
	}

	log.Infof("[%p] update slot-[%d]: \n%s", s, slotId, n.ToJson())

	s.mappings[slotId] = n
	return s.resyncSlotMapping(slotId)
}

func (s *Topom) migrationComplete(slotId int) error {
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
		return errors.New("invalid action state")
	}

	n := &models.SlotMapping{
		Id:      slotId,
		GroupId: m.Action.TargetId,
	}
	s.mappings[slotId] = n

	if err := s.resyncSlotMapping(slotId); err != nil {
		s.mappings[slotId] = m
		return err
	}

	if err := s.store.SaveSlotMapping(slotId, n); err != nil {
		s.mappings[slotId] = m
		return err
	}

	log.Infof("[%p] update slot-[%d]: \n%s", s, slotId, n.ToJson())
	return nil
}

func (s *Topom) migrationProcess(slotId int) (int, error) {
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
		return 0, errors.New("invalid action state")
	}
	if s.isSlotLocked(m) {
		return 0, errors.New("slot is locked")
	}

	c, err := s.redisp.GetClient(s.getGroupMaster(m.GroupId))
	if err != nil {
		return 0, err
	}
	defer s.redisp.PutClient(c)

	host, port, err := net.SplitHostPort(s.getGroupMaster(m.Action.TargetId))
	if err != nil {
		return 0, err
	}
	return c.SlotsMgrtTagSlot(host, port, slotId)
}
