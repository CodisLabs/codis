// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"errors"
	"sync"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/redis"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Slot struct {
	Id    int
	Info  *models.Slot
	Group *models.ServerGroup

	backend struct {
		addr string
		host []byte
		port []byte
		bc   *SharedBackendConn
	}
	migrate struct {
		from string
		bc   *SharedBackendConn
	}

	jobs sync.WaitGroup
	lock struct {
		hold bool
		sync.RWMutex
	}
}

func (s *Slot) blockAndWait() {
	if !s.lock.hold {
		s.lock.hold = true
		s.lock.Lock()
	}
	s.jobs.Wait()
}

func (s *Slot) unblock() {
	if !s.lock.hold {
		return
	}
	s.lock.hold = false
	s.lock.Unlock()
}

func (s *Slot) reset() {
	s.Info, s.Group = nil, nil
	s.backend.addr = ""
	s.backend.host = nil
	s.backend.port = nil
	s.backend.bc = nil
	s.migrate.from = ""
	s.migrate.bc = nil
}

func (s *Slot) forward(r *Request, key []byte) error {
	s.lock.RLock()
	bc, err := s.prepare(r, key)
	s.lock.RUnlock()
	if err != nil {
		return err
	} else {
		bc.PushBack(r)
		return nil
	}
}

var ErrSlotIsNotReady = errors.New("slot is not ready, may be offline")

func (s *Slot) prepare(r *Request, key []byte) (*SharedBackendConn, error) {
	if s.backend.bc == nil {
		log.Infof("slot-%04d is not ready: key = %s", s.Id, key)
		return nil, ErrSlotIsNotReady
	}
	if len(key) != 0 && s.migrate.bc != nil {
		if n, err := redis.SlotsMgrtTagOne(s.migrate.from, s.backend.host, s.backend.port, key); err != nil {
			log.InfoErrorf(err, "slot-%04d slotsmgrttagone from %s to %s error, key = %s",
				s.Id, s.migrate.from, s.backend.addr, key)
			return nil, err
		} else {
			log.Debugf("slot-%04d slotsmgrttagone from %s to %s: n = %d, key = %s",
				s.Id, s.migrate.from, s.backend.addr, n, key)
		}
	}
	r.slot = s
	s.jobs.Add(1)
	r.Wait.Add(1)
	return s.backend.bc, nil
}
