// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"sync"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

type Router struct {
	mu sync.RWMutex

	pool map[string]*SharedBackendConn

	slots [models.MaxSlotNum]Slot

	dispFunc

	config *Config
	online bool
	closed bool
}

func NewRouter(config *Config) *Router {
	s := &Router{config: config}
	s.pool = make(map[string]*SharedBackendConn)
	for i := range s.slots {
		s.slots[i].id = i
	}
	if config.BackendReadReplica {
		s.dispFunc = dispReadReplica
	}
	return s
}

func (s *Router) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.online = true
}

func (s *Router) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true

	for i := range s.slots {
		s.fillSlot(&models.Slot{Id: i})
	}
}

func (s *Router) GetGroupIds() map[int]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	var groupIds = make(map[int]bool)
	for i := range s.slots {
		if gid := s.slots[i].backend.id; gid != 0 {
			groupIds[gid] = true
		}
		if gid := s.slots[i].migrate.id; gid != 0 {
			groupIds[gid] = true
		}
	}
	return groupIds
}

func (s *Router) GetSlots() []*models.Slot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	slots := make([]*models.Slot, len(s.slots))
	for i := range s.slots {
		slots[i] = s.slots[i].snapshot(true)
	}
	return slots
}

func (s *Router) GetSlot(id int) *models.Slot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id < 0 || id >= len(s.slots) {
		return nil
	}
	slot := &s.slots[id]
	return slot.snapshot(true)
}

var (
	ErrClosedRouter  = errors.New("use of closed router")
	ErrInvalidSlotId = errors.New("use of invalid slot id")
)

func (s *Router) FillSlot(m *models.Slot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedRouter
	}
	return s.fillSlot(m)
}

func (s *Router) KeepAlive() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrClosedRouter
	}
	for _, bc := range s.pool {
		bc.KeepAlive()
	}
	return nil
}

func (s *Router) isOnline() bool {
	return s.online && !s.closed
}

func (s *Router) dispatch(r *Request) error {
	hkey := getHashKey(r.Multi, r.OpStr)
	var id = Hash(hkey) % uint32(len(s.slots))
	slot := &s.slots[id]
	return slot.forward(s.dispFunc, r, hkey)
}

func (s *Router) dispatchSlot(r *Request, id int) error {
	if id < 0 || id >= len(s.slots) {
		return ErrInvalidSlotId
	}
	slot := &s.slots[id]
	return slot.forward(s.dispFunc, r, nil)
}

func (s *Router) dispatchAddr(r *Request, addr string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bc := s.getBackendConn(addr, false)
	if bc == nil {
		return false
	} else {
		bc.PushBack(r)
		s.putBackendConn(bc)
		return true
	}
}

func (s *Router) getBackendConn(addr string, create bool) *SharedBackendConn {
	if bc := s.pool[addr]; bc != nil {
		return bc.Retain()
	} else if create {
		bc := NewSharedBackendConn(addr, s.config)
		s.pool[addr] = bc
		return bc
	} else {
		return nil
	}
}

func (s *Router) putBackendConn(bc *SharedBackendConn) {
	if bc != nil && bc.Release() {
		delete(s.pool, bc.Addr())
	}
}

func (s *Router) fillSlot(m *models.Slot) error {
	id := m.Id
	if id < 0 || id >= len(s.slots) {
		return ErrInvalidSlotId
	}
	slot := &s.slots[id]
	slot.blockAndWait()

	s.putBackendConn(slot.backend.bc)
	slot.backend.bc = nil
	slot.backend.id = 0
	s.putBackendConn(slot.migrate.bc)
	slot.migrate.bc = nil
	slot.migrate.id = 0
	for i := range slot.replicaGroups {
		for _, bc := range slot.replicaGroups[i] {
			s.putBackendConn(bc)
		}
	}
	slot.replicaGroups = nil

	if addr := m.BackendAddr; len(addr) != 0 {
		slot.backend.bc = s.getBackendConn(addr, true)
		slot.backend.id = m.BackendAddrId
	}
	if from := m.MigrateFrom; len(from) != 0 {
		slot.migrate.bc = s.getBackendConn(from, true)
		slot.migrate.id = m.MigrateFromId
	}
	if s.dispFunc != nil {
		for i := range m.ReplicaGroups {
			var group []*SharedBackendConn
			for _, addr := range m.ReplicaGroups[i] {
				group = append(group, s.getBackendConn(addr, true))
			}
			slot.replicaGroups = append(slot.replicaGroups, group)
		}
	}

	if !m.Locked {
		slot.unblock()
	}
	if !s.closed {
		if slot.migrate.bc != nil {
			log.Warnf("fill slot %04d, backend.addr = %s, migrate.from = %s, locked = %t",
				id, slot.backend.bc.Addr(), slot.migrate.bc.Addr(), slot.lock.hold)
		} else {
			log.Warnf("fill slot %04d, backend.addr = %s, locked = %t",
				id, slot.backend.bc.Addr(), slot.lock.hold)
		}
	}
	return nil
}

func (s *Router) SwitchMasters(masters map[int]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedRouter
	}

	for i := range s.slots {
		s.switchMasters(i, masters)
	}
	return nil
}

func (s *Router) switchMasters(id int, masters map[int]string) error {
	slot := &s.slots[id]

	var backendAddr string
	if slot.backend.id != 0 {
		backendAddr = masters[slot.backend.id]
	}
	var migrateFrom string
	if slot.migrate.id != 0 {
		migrateFrom = masters[slot.migrate.id]
	}

	var changed bool

	if backendAddr != "" {
		if slot.backend.bc.Addr() != backendAddr {
			changed = true
		}
	}
	if migrateFrom != "" {
		if slot.migrate.bc.Addr() != migrateFrom {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	log.Warnf("slot %04d will switch master", id)

	var m = slot.snapshot(false)

	if backendAddr != "" {
		m.BackendAddr = backendAddr
	}
	if migrateFrom != "" {
		m.MigrateFrom = migrateFrom
	}
	return s.fillSlot(m)
}

func (s *Router) SetSentinels(servers []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	panic("todo")
}
