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
	mu sync.Mutex

	auth string
	pool map[string]*SharedBackendConn

	slots [models.MaxSlotNum]Slot

	online bool
	closed bool
}

func NewRouter(auth string) *Router {
	s := &Router{
		auth: auth,
		pool: make(map[string]*SharedBackendConn),
	}
	for i := 0; i < len(s.slots); i++ {
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
	for i := 0; i < len(s.slots); i++ {
		s.resetSlot(i)
	}
	s.closed = true
}

func (s *Router) GetSlots() []*models.Slot {
	s.mu.Lock()
	defer s.mu.Unlock()
	slots := make([]*models.Slot, 0, len(s.slots))
	for i := 0; i < len(s.slots); i++ {
		slot := &s.slots[i]
		slots = append(slots, &models.Slot{
			Id:          i,
			BackendAddr: slot.backend.Addr(),
			MigrateFrom: slot.migrate.Addr(),
			Locked:      slot.lock.hold,
		})
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
	if id >= 0 && id < len(s.slots) {
		return s.fillSlot(id, addr, from, locked)
	} else {
		return ErrInvalidSlotId
	}
}

func (s *Router) KeepAlive() error {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	slot := &s.slots[hashSlot(hkey)]
	return slot.forward(r, hkey)
}

func (s *Router) getBackendConn(addr string) *SharedBackendConn {
	if bc := s.pool[addr]; bc != nil {
		return bc.Retain()
	} else {
		bc := NewSharedBackendConn(addr, s.auth)
		s.pool[addr] = bc
		return bc
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
		slot.backend = s.getBackendConn(addr)
	}
	if len(from) != 0 {
		slot.migrate = s.getBackendConn(from)
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
