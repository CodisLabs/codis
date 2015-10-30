package topom

import (
	"math"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/atomic2"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func (s *Topom) GetGroupModels() []*models.Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getGroupModels()
}

func (s *Topom) getGroupModels() []*models.Group {
	glist := make([]*models.Group, 0, len(s.groups))
	for _, g := range s.groups {
		glist = append(glist, g)
	}
	models.SortGroup(glist, func(g1, g2 *models.Group) bool {
		return g1.Id < g2.Id
	})
	return glist
}

func (s *Topom) getGroup(groupId int) (*models.Group, error) {
	if g := s.groups[groupId]; g != nil {
		return g, nil
	}
	return nil, errors.Errorf("group-[%d] doesn't exist", groupId)
}

func (s *Topom) getGroupMaster(groupId int) string {
	if g := s.groups[groupId]; g != nil && len(g.Servers) != 0 {
		return g.Servers[0]
	}
	return ""
}

func (s *Topom) isGroupPromoting(groupId int) bool {
	if g := s.groups[groupId]; g != nil {
		return g.Promoting
	}
	return false
}

func (s *Topom) lockGroupMaster(groupId int) {
	if l := s.mlocks[groupId]; l != nil {
		if n := l.Incr(); n > 128 {
			log.Warnf("[%p] mlocks-[%d] increase to %d", s, groupId, n)
		}
	}
}

func (s *Topom) unlockGroupMaster(groupId int) {
	if l := s.mlocks[groupId]; l != nil {
		if n := l.Decr(); n < 0 {
			log.Panicf("[%p] mlocks-[%d] decrease to %d", s, groupId, n)
		}
	}
}

func (s *Topom) isGroupMasterLocked(groupId int) bool {
	if l := s.mlocks[groupId]; l != nil {
		return l.Get() != 0
	}
	return false
}

func (s *Topom) CreateGroup(groupId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	if groupId <= 0 || groupId > math.MaxInt16 {
		return errors.Errorf("invalid group id, out of range")
	}
	if s.groups[groupId] != nil {
		return errors.Errorf("group-[%d] already exists", groupId)
	}

	g := &models.Group{
		Id:      groupId,
		Servers: []string{},
	}
	if err := s.store.CreateGroup(groupId, g); err != nil {
		log.ErrorErrorf(err, "[%p] create group-[%d] failed", s, groupId)
		return errors.Errorf("store: create group-[%d] failed", groupId)
	}

	s.groups[groupId] = g
	s.mlocks[groupId] = &atomic2.Int64{}

	log.Infof("[%p] create group-[%d]:\n%s", s, groupId, g.Encode())

	return nil
}

func (s *Topom) RemoveGroup(groupId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	g, err := s.getGroup(groupId)
	if err != nil {
		return err
	}
	if len(g.Servers) != 0 {
		return errors.Errorf("group-[%d] isn't empty", groupId)
	}

	if err := s.store.RemoveGroup(groupId); err != nil {
		log.ErrorErrorf(err, "[%p] remove group-[%d] failed", s, groupId)
		return errors.Errorf("store: remove group-[%d] failed", groupId)
	}

	delete(s.groups, groupId)
	delete(s.mlocks, groupId)

	log.Infof("[%p] remove group-[%d]:\n%s", s, groupId, g.Encode())

	return nil
}

func (s *Topom) GroupAddServer(groupId int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	if addr == "" {
		return errors.Errorf("invalid server address")
	}

	if _, ok := s.stats.servers[addr]; ok {
		return errors.Errorf("server %s already exists", addr)
	}

	if s.isGroupPromoting(groupId) {
		return errors.Errorf("group-[%d] is promoting", groupId)
	}

	g, err := s.getGroup(groupId)
	if err != nil {
		return err
	}

	n := &models.Group{
		Id:      groupId,
		Servers: append(g.Servers, addr),
	}
	if err := s.store.UpdateGroup(groupId, n); err != nil {
		log.ErrorErrorf(err, "[%p] update group-[%d] failed", s, groupId)
		return errors.Errorf("store: update group-[%d] failed", groupId)
	}

	s.groups[groupId] = n
	s.stats.servers[addr] = nil

	log.Infof("[%p] update group-[%d]:\n%s", s, groupId, n.Encode())

	return nil
}

func (s *Topom) GroupDelServer(groupId int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	if addr == "" {
		return errors.Errorf("invalid server address")
	}

	if _, ok := s.stats.servers[addr]; !ok {
		return errors.Errorf("server %s doesn't exist", addr)
	}

	if s.isGroupPromoting(groupId) {
		return errors.Errorf("group-[%d] is promoting", groupId)
	}

	g, err := s.getGroup(groupId)
	if err != nil {
		return err
	}

	servers := []string{}
	for _, x := range g.Servers {
		if x != addr {
			servers = append(servers, x)
		}
	}
	if len(g.Servers) == len(servers) {
		return errors.Errorf("group-[%d] doesn't have server %s", groupId, addr)
	}
	if addr == g.Servers[0] {
		if len(g.Servers) != 1 || len(s.getSlotsByGroup(groupId)) != 0 {
			return errors.Errorf("master of group-[%d] is still busy", groupId)
		}
		if s.isGroupMasterLocked(groupId) {
			return errors.Errorf("master of group-[%d] is locked", groupId)
		}
	}

	n := &models.Group{
		Id:      groupId,
		Servers: servers,
	}
	if err := s.store.UpdateGroup(groupId, n); err != nil {
		log.ErrorErrorf(err, "[%p] update group-[%d] failed", s, groupId)
		return errors.Errorf("store: update group-[%d] failed", groupId)
	}

	s.groups[groupId] = n
	delete(s.stats.servers, addr)

	log.Infof("[%p] update group-[%d]:\n%s", s, groupId, n.Encode())

	return nil
}

func (s *Topom) GroupPromoteServer(groupId int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	if addr == "" {
		return errors.Errorf("invalid server address")
	}

	if s.isGroupMasterLocked(groupId) {
		return errors.Errorf("master of group-[%d] is locked", groupId)
	}
	if s.isGroupPromoting(groupId) {
		return errors.Errorf("group-[%d] is promoting", groupId)
	}

	g, err := s.getGroup(groupId)
	if err != nil {
		return err
	}

	servers := []string{}
	for _, x := range g.Servers {
		if x != addr {
			servers = append(servers, x)
		}
	}
	if len(g.Servers) == len(servers) {
		return errors.Errorf("group-[%d] doesn't have server %s", groupId, addr)
	}
	if addr == g.Servers[0] {
		return errors.Errorf("promote master of group-[%d] again", groupId)
	}

	n := &models.Group{
		Id:        groupId,
		Servers:   append([]string{addr}, servers...),
		Promoting: true,
	}
	if err := s.store.UpdateGroup(groupId, n); err != nil {
		log.ErrorErrorf(err, "[%p] update group-[%d] failed", s, groupId)
		return errors.Errorf("store: update group-[%d] failed", groupId)
	}

	s.groups[groupId] = n

	log.Infof("[%p] update group-[%d]:\n%s", s, groupId, n.Encode())

	return nil
}

func (s *Topom) GroupPromoteCommit(groupId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	g, err := s.getGroup(groupId)
	if err != nil {
		return err
	}
	if !g.Promoting {
		return errors.Errorf("group-[%d] isn't promoting")
	}

	if err := s.resyncGroup(groupId); err != nil {
		return err
	}

	n := &models.Group{
		Id:      groupId,
		Servers: g.Servers,
	}
	s.groups[groupId] = n

	var rollback = true
	defer func() {
		if rollback {
			s.groups[groupId] = g
		}
	}()

	if err := s.resyncGroup(groupId); err != nil {
		return err
	}

	if err := s.store.UpdateGroup(groupId, n); err != nil {
		log.ErrorErrorf(err, "[%p] update group-[%d] failed", s, groupId)
		return errors.Errorf("store: update group-[%d] failed", groupId)
	}

	rollback = false

	log.Infof("[%p] update group-[%d]:\n%s", s, groupId, n.Encode())

	return nil
}

func (s *Topom) GroupRepairMaster(groupId int, addr string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrClosedTopom
	}

	g, err := s.getGroup(groupId)
	if err != nil {
		return err
	}

	var index = -1
	for i, x := range g.Servers {
		if x == addr {
			index = i
		}
	}

	var master = "NO:ONE"
	switch {
	case index < 0:
		return errors.Errorf("group-[%d] doesn't have server %s", groupId, addr)
	case index > 0:
		master = g.Servers[0]
	}

	s.lockGroupMaster(groupId)

	go func() {
		defer s.unlockGroupMaster(groupId)
		if master != "NO:ONE" {
			c, err := s.redisp.GetClient(master)
			if err != nil {
				log.WarnErrorf(err, "server %s create client failed", master)
				return
			}
			defer s.redisp.PutClient(c)
			if err := c.SetMaster("NO:ONE"); err != nil {
				log.WarnErrorf(err, "server %s set master = NO:ONE failed", master)
				return
			}
			log.Infof("[%p] repair-[%d]: server %s set master to NO:ONE OK", s, groupId, master)
		}
		c, err := NewRedisClient(addr, s.config.ProductAuth, time.Minute*15)
		if err != nil {
			log.WarnErrorf(err, "server %s create client failed", addr)
			return
		}
		defer c.Close()
		if err := c.SetMaster(master); err != nil {
			log.WarnErrorf(err, "server %s set master = %s failed", addr, master)
			return
		}
		log.Infof("[%p] repair-[%d]: server %s set master to %s OK", s, groupId, addr, master)
	}()

	log.Infof("[%p] repair-[%d]: server %s set master to %s, pending...", s, groupId, addr, master)

	return nil
}

func (s *Topom) resyncGroup(groupId int) error {
	slots := s.getSlotsByGroup(groupId)
	if len(slots) == 0 {
		return nil
	}
	errs := s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
		if err := c.FillSlots(slots...); err != nil {
			log.WarnErrorf(err, "[%p] proxy-[%s] resync group-[%d] failed", s, p.Token, groupId)
			return err
		}
		return nil
	})
	for t, err := range errs {
		if err != nil {
			return errors.Errorf("proxy-[%s] resync group-[%d] failed", t, groupId)
		}
	}
	return nil
}
