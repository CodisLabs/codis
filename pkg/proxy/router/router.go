// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"net"
	"sync"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Router struct {
	mu sync.Mutex

	auth string
	pool map[string]*SharedBackendConn

	slots [models.MaxSlotNum]*Slot

	closed bool
}

func New() *Router {
	return NewWithAuth("")
}

func NewWithAuth(auth string) *Router {
	s := &Router{
		auth: auth,
		pool: make(map[string]*SharedBackendConn),
	}
	for i := 0; i < len(s.slots); i++ {
		s.slots[i] = &Slot{id: i}
	}
	return s
}

func (s *Router) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	for i := 0; i < len(s.slots); i++ {
		s.resetSlot(i)
	}
	s.closed = true
	return nil
}

func (s *Router) GetSlots() []*models.Slot {
	s.mu.Lock()
	defer s.mu.Unlock()
	slots := make([]*models.Slot, 0, len(s.slots))
	for i, slot := range s.slots {
		slots = append(slots, &models.Slot{
			Id:          i,
			BackendAddr: slot.backend.addr,
			MigrateFrom: slot.migrate.from,
			Locked:      slot.lock.hold,
		})
	}
	return slots
}

var (
	ErrClosedRouter  = errors.New("use of closed router")
	ErrInvalidSlotId = errors.New("use of invalid slot id")
)

func (s *Router) FillSlot(i int, addr, from string, locked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedRouter
	}
	if i >= 0 && i < len(s.slots) {
		s.fillSlot(i, addr, from, locked)
		return nil
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

func (s *Router) Dispatch(r *Request) error {
	hkey := getHashKey(r.Resp, r.OpStr)
	slot := s.slots[hashSlot(hkey)]
	return slot.forward(r, hkey)
}

func (s *Router) getBackendConn(addr string) *SharedBackendConn {
	bc := s.pool[addr]
	if bc != nil {
		bc.IncrRefcnt()
	} else {
		bc = NewSharedBackendConn(addr, s.auth)
		s.pool[addr] = bc
	}
	return bc
}

func (s *Router) putBackendConn(bc *SharedBackendConn) {
	if bc != nil && bc.Close() {
		delete(s.pool, bc.Addr())
	}
}

func (s *Router) resetSlot(i int) {
	slot := s.slots[i]
	slot.blockAndWait()

	s.putBackendConn(slot.backend.bc)
	s.putBackendConn(slot.migrate.bc)
	slot.reset()

	slot.unblock()
}

func (s *Router) fillSlot(i int, addr, from string, locked bool) {
	slot := s.slots[i]
	slot.blockAndWait()

	s.putBackendConn(slot.backend.bc)
	s.putBackendConn(slot.migrate.bc)
	slot.reset()

	if len(addr) != 0 {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			log.ErrorErrorf(err, "split host-port failed, address = %s", addr)
		} else {
			slot.backend.host = []byte(host)
			slot.backend.port = []byte(port)
		}
		slot.backend.addr = addr
		slot.backend.bc = s.getBackendConn(addr)
	}
	if len(from) != 0 {
		slot.migrate.from = from
		slot.migrate.bc = s.getBackendConn(from)
	}

	if !locked {
		slot.unblock()
	}

	if slot.migrate.bc != nil {
		log.Infof("fill slot %04d, backend.addr = %s, migrate.from = %s, locked = %t",
			i, slot.backend.addr, slot.migrate.from, locked)
	} else {
		log.Infof("fill slot %04d, backend.addr = %s, locked = %t",
			i, slot.backend.addr, locked)
	}
}
