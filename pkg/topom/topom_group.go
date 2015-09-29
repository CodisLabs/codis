package topom

import (
	"math"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func (s *Topom) daemonRedisPool() {
	var ticker = time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.exit.C:
			return
		case <-ticker.C:
			s.redisp.Cleanup()
		}
	}
}

func (s *Topom) ListGroup() []*models.Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	glist := make([]*models.Group, 0, len(s.groups))
	for _, g := range s.groups {
		glist = append(glist, g)
	}
	return glist
}

func (s *Topom) getGroup(groupId int) (*models.Group, error) {
	if g := s.groups[groupId]; g != nil {
		return g, nil
	}
	return nil, errors.New("group does not exist")
}

func (s *Topom) getGroupMaster(groupId int) string {
	if g := s.groups[groupId]; g != nil && len(g.Servers) != 0 {
		return g.Servers[0]
	}
	return ""
}

func (s *Topom) CreateGroup(groupId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	if groupId <= 0 || groupId > math.MaxInt32 {
		return errors.New("invalid group id")
	}
	if s.groups[groupId] != nil {
		return errors.New("group already exists")
	}

	g := &models.Group{
		Id: groupId,
	}
	if err := s.store.CreateGroup(groupId, g); err != nil {
		log.WarnErrorf(err, "group-[%d] create failed", groupId)
		return errors.New("group create failed")
	}

	log.Infof("[%p] create group-[%d]", s, groupId)

	s.groups[groupId] = g
	return nil
}

func (s *Topom) RemoveGroup(groupId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	_, err := s.getGroup(groupId)
	if err != nil {
		return err
	}
	if len(s.getSlotsByGroup(groupId)) != 0 {
		return errors.New("group is still busy")
	}

	if err := s.store.RemoveGroup(groupId); err != nil {
		log.WarnErrorf(err, "group-[%d] remove failed", groupId)
		return errors.New("group remove failed")
	}

	log.Infof("[%p] remove group-[%d]", s, groupId)

	delete(s.groups, groupId)
	return nil
}

func (s *Topom) repairGroup(groupId int) error {
	g, err := s.getGroup(groupId)
	if err != nil {
		return err
	}
	for i, addr := range g.Servers {
		c, err := s.redisp.GetClient(addr)
		if err != nil {
			log.WarnErrorf(err, "group-[%d] repair failed, server[%d] = %s", groupId, i, addr)
			return errors.New("create redis client failed")
		}
		defer s.redisp.PutClient(c)

		var master = ""
		if i == 0 {
			log.Infof("group-[%d] repair [M] %s", groupId, addr)
		} else {
			master = g.Servers[0]
			log.Infof("group-[%d] repair [M] %s <---> %s [S]", groupId, master, addr)
		}
		if err := c.SlaveOf(master); err != nil {
			log.WarnErrorf(err, "group-[%d] repair failed, server[%d] = %s", groupId, i, addr)
			return errors.New("server set slaveof failed")
		}
	}
	return nil
}

func (s *Topom) resyncGroup(groupId int, locked bool) map[string]error {
	slots := s.getSlotsByGroup(groupId)
	if locked {
		for _, slot := range slots {
			slot.Locked = true
		}
	}
	return s.broadcast(func(p *models.Proxy, c *proxy.ApiClient) error {
		if len(slots) == 0 {
			return nil
		}
		if err := c.FillSlots(slots...); err != nil {
			log.WarnErrorf(err, "proxy-[%s] resync group-[%d] failed", p.Token, groupId)
			return errors.New("proxy resync group failed")
		}
		return nil
	})
}

func (s *Topom) RepairGroup(groupId int) (map[string]error, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosedTopom
	}
	if err := s.repairGroup(groupId); err != nil {
		return nil, err
	}
	return s.resyncGroup(groupId, false), nil
}

func (s *Topom) ResyncGroup(groupId int) (map[string]error, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosedTopom
	}
	_, err := s.getGroup(groupId)
	if err != nil {
		return nil, err
	}
	return s.resyncGroup(groupId, false), nil
}

func (s *Topom) GroupAddNewServer(groupId int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	for _, g := range s.groups {
		for _, x := range g.Servers {
			if x == addr {
				return errors.New("server already exists")
			}
		}
	}
	g, err := s.getGroup(groupId)
	if err != nil {
		return err
	}

	c, err := s.redisp.GetClient(addr)
	if err != nil {
		log.WarnErrorf(err, "group-[%d] add server failed, server = %s", groupId, addr)
		return errors.New("create redis client failed")
	}
	defer s.redisp.PutClient(c)

	if _, err := c.SlotsInfo(); err != nil {
		log.WarnErrorf(err, "group-[%d] add server failed, server = %s", groupId, addr)
		return errors.New("server check slots failed")
	}
	if err := c.SlaveOf(s.getGroupMaster(groupId)); err != nil {
		log.WarnErrorf(err, "group-[%d] add server failed, server = %s", groupId, addr)
		return errors.New("server set slaveof failed")
	}

	log.Infof("[%p] group-[%d] add server = %s", s, groupId, addr)

	n := &models.Group{
		Id:      groupId,
		Servers: append(g.Servers, addr),
	}
	if err := s.store.UpdateGroup(groupId, n); err != nil {
		log.WarnErrorf(err, "group-[%d] update failed", groupId)
		return errors.New("group update failed")
	}

	log.Infof("[%p] update group: %+v", s, n)

	s.groups[groupId] = n
	return nil
}

func (s *Topom) GroupRemoveServer(groupId int, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}

	g, err := s.getGroup(groupId)
	if err != nil {
		return err
	}
	slist := []string{}
	for _, x := range g.Servers {
		if x != addr {
			slist = append(slist, x)
		}
	}
	if len(slist) == len(g.Servers) {
		return errors.New("server does not exist")
	}
	if addr == g.Servers[0] {
		if len(g.Servers) != 1 || len(s.getSlotsByGroup(groupId)) != 0 {
			return errors.New("server is still busy")
		}
	}

	log.Infof("[%p] group-[%d] remove server = %s", s, groupId, addr)

	n := &models.Group{
		Id:      groupId,
		Servers: slist,
	}
	if err := s.store.UpdateGroup(groupId, n); err != nil {
		log.WarnErrorf(err, "group-[%d] update failed", groupId)
		return errors.New("group update failed")
	}

	log.Infof("[%p] update group: %+v", s, n)

	s.groups[groupId] = n
	return nil
}

func (s *Topom) GroupPromoteServer(groupId int, addr string, force bool) (map[string]error, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosedTopom
	}

	g, err := s.getGroup(groupId)
	if err != nil {
		return nil, err
	}
	slist := []string{}
	for _, x := range g.Servers {
		if x != addr {
			slist = append(slist, x)
		}
	}
	if len(slist) == len(g.Servers) {
		return nil, errors.New("server does not exist")
	}
	if addr == g.Servers[0] {
		return nil, errors.New("server promote again")
	} else {
		slist = append([]string{addr}, slist[1:]...)
	}

	log.Infof("group-[%d] will be locked by force", groupId)

	if errs := s.resyncGroup(groupId, true); len(errs) != 0 {
		if !force {
			return errs, nil
		}
		log.Warnf("group-[%d] force promote with resync errors", groupId)
	}

	log.Infof("[%p] group-[%d] promote server = %s", s, groupId, addr)

	n := &models.Group{
		Id:      groupId,
		Servers: slist,
	}
	if err := s.store.UpdateGroup(groupId, n); err != nil {
		log.WarnErrorf(err, "group-[%d] update failed", groupId)
		return nil, errors.New("group update failed")
	}

	log.Infof("[%p] update group: %+v", s, n)

	s.groups[groupId] = n

	if err := s.repairGroup(groupId); err != nil {
		log.WarnErrorf(err, "group-[%d] repair failed", groupId)
		return nil, errors.New("group repair failed")
	}

	log.Infof("group-[%d] will be recovered", groupId)

	return s.resyncGroup(groupId, false), nil
}
