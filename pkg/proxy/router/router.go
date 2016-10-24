// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"strconv"
	"strings"
	"sync"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

const MaxSlotNum = models.DEFAULT_SLOT_NUM

type Router struct {
	mu sync.Mutex

	auth string
	cmodel string
	pool map[string]*SharedBackendConn

	slots [MaxSlotNum]*Slot

	closed bool
}

func New() *Router {
	return NewWithAuth("", "server")
}

func NewWithAuth(auth string, cmodel string) *Router {
	s := &Router{
		auth: auth,
		cmodel: cmodel,
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

var errClosedRouter = errors.New("use of closed router")

func (s *Router) ResetSlot(i int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errClosedRouter
	}
	s.resetSlot(i)
	return nil
}

func (s *Router) FillSlot(i int, addr, from string, lock bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errClosedRouter
	}
	s.fillSlot(i, addr, from, lock)
	return nil
}

func (s *Router) KeepAlive() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errClosedRouter
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

func (s *Router) getSlotBackendConn(addr string, slot int) *SharedBackendConn {
    key := strconv.Itoa(slot) + ":" + addr
    bc := s.pool[key]
    if bc != nil {
        bc.IncrRefcnt()
    } else {
        bc = NewSharedBackendConn(addr, s.auth)
        s.pool[key] = bc
    }
    return bc
}

func (s *Router) putBackendConn(bc *SharedBackendConn) {
	if bc != nil && bc.Close() {
		delete(s.pool, bc.Addr())
	}
}

func (s *Router) putSlotBackendConn(bc *SharedBackendConn, slot int) {
    if bc != nil && bc.Close() {
        delete(s.pool, (strconv.Itoa(slot) + ":" + bc.addr))
    }
}

func (s *Router) isValidSlot(i int) bool {
	return i >= 0 && i < len(s.slots)
}

func (s *Router) resetSlot(i int) {
	if !s.isValidSlot(i) {
		return
	}
	slot := s.slots[i]
	slot.blockAndWait()

    	if s.cmodel == "slot" {
        	s.putSlotBackendConn(slot.backend.bc, i)
        	s.putSlotBackendConn(slot.migrate.bc, i)
    	} else {
        	s.putBackendConn(slot.backend.bc)
        	s.putBackendConn(slot.migrate.bc)
    	}
	slot.reset()

	slot.unblock()
}

func (s *Router) fillSlot(i int, addr, from string, lock bool) {
	if !s.isValidSlot(i) {
		return
	}
	slot := s.slots[i]
	slot.blockAndWait()

    	if s.cmodel == "slot" {
        	s.putSlotBackendConn(slot.backend.bc, i)
        	s.putSlotBackendConn(slot.migrate.bc, i)
    	} else {
        	s.putBackendConn(slot.backend.bc)
        	s.putBackendConn(slot.migrate.bc)
    	}
	slot.reset()

	if len(addr) != 0 {
		xx := strings.Split(addr, ":")
		if len(xx) >= 1 {
			slot.backend.host = []byte(xx[0])
		}
		if len(xx) >= 2 {
			slot.backend.port = []byte(xx[1])
		}
		slot.backend.addr = addr
        	if s.cmodel == "slot" {
            		slot.backend.bc = s.getSlotBackendConn(addr, i)
        	} else {
            		slot.backend.bc = s.getBackendConn(addr)
        	}
	}
	if len(from) != 0 {
		slot.migrate.from = from
	        if s.cmodel == "slot" {
            		slot.migrate.bc = s.getSlotBackendConn(from, i)
        	} else {
            		slot.migrate.bc = s.getBackendConn(from)
        	}
	}

	if !lock {
		slot.unblock()
	}

	if slot.migrate.bc != nil {
		log.Infof("fill slot %04d, backend.addr = %s, migrate.from = %s",
			i, slot.backend.addr, slot.migrate.from)
	} else {
		log.Infof("fill slot %04d, backend.addr = %s",
			i, slot.backend.addr)
	}
}
