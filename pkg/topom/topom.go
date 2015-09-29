package topom

import (
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type Topom struct {
	mu sync.RWMutex

	xauth string
	model *models.Topom
	store models.Store

	exit struct {
		C chan struct{}
	}
	wait sync.WaitGroup

	online bool
	closed bool

	ladmin net.Listener
	redisp *RedisPool

	config *Config

	mappings [models.MaxSlotNum]*models.SlotMapping

	groups  map[int]*models.Group
	proxies map[string]*models.Proxy
	clients map[string]*proxy.ApiClient
}

var ErrClosedTopom = errors.New("use of closed topom")

func NewWithConfig(store models.Store, config *Config) (*Topom, error) {
	s := &Topom{config: config, store: store}
	s.xauth = rpc.NewXAuth(config.ProductName, config.ProductAuth)
	s.model = &models.Topom{
		StartTime: time.Now().String(),
	}
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()

	s.redisp = NewRedisPool(config.ProductAuth, time.Minute*30)

	s.exit.C = make(chan struct{})

	if err := s.setup(); err != nil {
		s.Close()
		return nil, err
	}

	log.Infof("[%p] create new topom: %+v", s, s.model)

	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		s.daemonRedisPool()
	}()

	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		s.daemonMigration()
	}()

	go s.serveAdmin()

	return s, nil
}

func (s *Topom) setup() error {
	if l, err := net.Listen("tcp", s.config.AdminAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.ladmin = l
	}

	if addr, err := utils.ResolveAddr("tcp", s.ladmin.Addr().String()); err != nil {
		return err
	} else {
		s.model.AdminAddr = addr
	}

	if !utils.IsValidName(s.config.ProductName) {
		return errors.New("invalid product name, empty or using invalid character")
	}

	if err := s.store.Acquire(s.model); err != nil {
		return err
	} else {
		s.online = true
	}

	for i := 0; i < len(s.mappings); i++ {
		if m, err := s.store.LoadSlotMapping(i); err != nil {
			return err
		} else {
			s.mappings[i] = m
		}
	}

	if glist, err := s.store.ListGroup(); err != nil {
		return err
	} else {
		s.groups = make(map[int]*models.Group)
		for _, g := range glist {
			s.groups[g.Id] = g
		}
	}

	if plist, err := s.store.ListProxy(); err != nil {
		return err
	} else {
		s.proxies = make(map[string]*models.Proxy)
		s.clients = make(map[string]*proxy.ApiClient)
		for _, p := range plist {
			c := proxy.NewApiClient(p.AdminAddr)
			c.SetXAuth(s.config.ProductName, s.config.ProductAuth, p.Token)
			s.proxies[p.Token] = p
			s.clients[p.Token] = c
		}
	}
	return nil
}

func (s *Topom) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.exit.C)

	s.ladmin.Close()
	s.redisp.Close()

	defer s.store.Close()
	if !s.online {
		return nil
	} else {
		s.wait.Wait()
		return s.store.Release()
	}
}

func (s *Topom) GetXAuth() string {
	return s.xauth
}

func (s *Topom) GetModel() *models.Topom {
	return s.model
}

func (s *Topom) GetConfig() *Config {
	return s.config
}

func (s *Topom) IsOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online && !s.closed
}

func (s *Topom) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Topom) GetSlotMappings() []*models.SlotMapping {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*models.SlotMapping{}, s.mappings[:]...)
}

func (s *Topom) serveAdmin() {
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Infof("[%p] admin start service on %s", s, s.ladmin.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) {
		h := http.NewServeMux()
		h.Handle("/", newApiServer(s))
		hs := &http.Server{Handler: h}
		eh <- hs.Serve(l)
	}(s.ladmin)

	select {
	case <-s.exit.C:
		log.Infof("[%p] admin shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] admin exit on error", s)
	}
}

func (s *Topom) daemonMigration() {
	for {
		select {
		case <-s.exit.C:
			return
		default:
		}
		// TODO
		time.Sleep(time.Second)
	}
}

func (s *Topom) getSlotMapping(slotId int) (*models.SlotMapping, error) {
	if slotId >= 0 && slotId < len(s.mappings) {
		return s.mappings[slotId], nil
	}
	return nil, errors.Errorf("invalid slot id")
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
		if m.GroupId == groupId || m.Action.TargetId == groupId {
			slots = append(slots, s.toSlotState(m))
		}
	}
	return slots
}

func (s *Topom) maxActionIndex() (maxIndex int) {
	for _, m := range s.mappings {
		if m.Action.State != models.ActionNothing {
			if m.Action.Index > maxIndex {
				maxIndex = m.Action.Index
			}
		}
	}
	return
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
	g, err := s.getGroup(targetId)
	if err != nil {
		return err
	}
	switch m.Action.State {
	case models.ActionNothing:
	case models.ActionPending:
	default:
		return errors.Errorf("slot is being migrated")
	}

	n := &models.SlotMapping{
		Id:      m.Id,
		GroupId: m.GroupId,
	}
	n.Action.State = models.ActionPending
	n.Action.Index = s.maxActionIndex() + 1
	n.Action.TargetId = g.Id
	if err := s.store.SaveSlotMapping(m.Id, n); err != nil {
		log.WarnErrorf(err, "slot-[%d] update failed", m.Id)
		return err
	}

	log.Infof("[%p] update slot: %+v", s, n)

	s.mappings[m.Id] = n
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
	switch m.Action.State {
	case models.ActionNothing:
		return nil
	case models.ActionPending:
	default:
		return errors.Errorf("slot is being migrated")
	}

	n := &models.SlotMapping{
		Id:      m.Id,
		GroupId: m.GroupId,
	}
	if err := s.store.SaveSlotMapping(m.Id, n); err != nil {
		log.WarnErrorf(err, "slot-[%d] update failed", m.Id)
		return err
	}

	log.Infof("[%p] update slot: %+v", s, n)

	s.mappings[m.Id] = n
	return nil
}
