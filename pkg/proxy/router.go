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

	config *Config
	online bool
	closed bool
}

func NewRouter(config *Config) *Router {
	s := &Router{config: config}
	s.pool = make(map[string]*SharedBackendConn)
	for i := range s.slots {
		s.slots[i].id = uint32(i)
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
	for i := range s.slots {
		s.resetSlot(i)
	}
	s.closed = true
}

func (s *Router) GetSlots() []*models.Slot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	slots := make([]*models.Slot, len(s.slots))
	for i := range slots {
		m := &s.slots[i]
		slots[i] = &models.Slot{
			Id:          i,
			BackendAddr: m.backend.Addr(),
			MigrateFrom: m.migrate.Addr(),
			Locked:      m.lock.hold,
		}
	}
	return slots
}

var (
	ErrClosedRouter  = errors.New("use of closed router")
	ErrInvalidSlotId = errors.New("use of invalid slot id")
)

func (s *Router) FillSlot(id int, addr, from string, locked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedRouter
	}
	if id < 0 || id >= len(s.slots) {
		return ErrInvalidSlotId
	}
	return s.fillSlot(id, addr, from, locked)
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
	return slot.forward(r, hkey)
}

func (s *Router) dispatchSlot(r *Request, id int) error {
	if id < 0 || id >= len(s.slots) {
		return ErrInvalidSlotId
	}
	slot := &s.slots[id]
	return slot.forward(r, nil)
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

func (s *Router) resetSlot(id int) {
	slot := &s.slots[id]
	slot.blockAndWait()

	s.putBackendConn(slot.backend)
	s.putBackendConn(slot.migrate)
	slot.reset()

	slot.unblock()
}

func (s *Router) fillSlot(id int, addr, from string, locked bool) error {
	slot := &s.slots[id]
	slot.blockAndWait()

	s.putBackendConn(slot.backend)
	s.putBackendConn(slot.migrate)
	slot.reset()

	if len(addr) != 0 {
		slot.backend = s.getBackendConn(addr, true)
	}
	if len(from) != 0 {
		slot.migrate = s.getBackendConn(from, true)
	}

	if !locked {
		slot.unblock()
	}

	if slot.migrate != nil {
		log.Warnf("fill slot %04d, backend.addr = %s, migrate.from = %s, locked = %t",
			id, slot.backend.Addr(), slot.migrate.Addr(), locked)
	} else {
		log.Warnf("fill slot %04d, backend.addr = %s, locked = %t",
			id, slot.backend.Addr(), locked)
	}
	return nil
}
