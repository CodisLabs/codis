// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"fmt"
	"sync"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

type Slot struct {
	id   int
	lock struct {
		hold bool
		sync.RWMutex
	}
	refs sync.WaitGroup

	switched bool

	backend, migrate struct {
		id int
		bc *sharedBackendConn
	}
	replicaGroups [][]*sharedBackendConn
}

func (s *Slot) snapshot() *models.Slot {
	var m = &models.Slot{
		Id:     s.id,
		Locked: s.lock.hold,

		BackendAddr:        s.backend.bc.Addr(),
		BackendAddrGroupId: s.backend.id,
		MigrateFrom:        s.migrate.bc.Addr(),
		MigrateFromGroupId: s.migrate.id,
	}
	for i := range s.replicaGroups {
		var group []string
		for _, bc := range s.replicaGroups[i] {
			group = append(group, bc.Addr())
		}
		m.ReplicaGroups = append(m.ReplicaGroups, group)
	}
	return m
}

func (s *Slot) blockAndWait() {
	if !s.lock.hold {
		s.lock.hold = true
		s.lock.Lock()
	}
	s.refs.Wait()
}

func (s *Slot) unblock() {
	if !s.lock.hold {
		return
	}
	s.lock.hold = false
	s.lock.Unlock()
}

func (s *Slot) forward(r *Request, hkey []byte) error {
	s.lock.RLock()
	bc, err := s.prepare(r, hkey)
	s.lock.RUnlock()
	if err != nil {
		return err
	} else {
		bc.PushBack(r)
		return nil
	}
}

var (
	ErrSlotIsNotReady = errors.New("slot is not ready, may be offline")
	ErrRespIsRequired = errors.New("resp is required")
)

func (s *Slot) prepare(r *Request, hkey []byte) (*BackendConn, error) {
	if s.backend.bc == nil {
		log.Debugf("slot-%04d is not ready: hash key = '%s'", s.id, hkey)
		return nil, ErrSlotIsNotReady
	}
	if err := s.slotsmgrt(r, hkey); err != nil {
		log.Debugf("slot-%04d migrate from = %s to %s failed: hash key = '%s', error = %s",
			s.id, s.migrate.bc.Addr(), s.backend.bc.Addr(), hkey, err)
		return nil, err
	} else {
		r.Group = &s.refs
		r.Group.Add(1)
		return s.forward2(r)
	}
}

func (s *Slot) slotsmgrt(r *Request, hkey []byte) error {
	if s.migrate.bc == nil || len(hkey) == 0 {
		return nil
	}

	m := &Request{}
	m.Multi = []*redis.Resp{
		redis.NewBulkBytes([]byte("SLOTSMGRTTAGONE")),
		redis.NewBulkBytes(s.backend.bc.host),
		redis.NewBulkBytes(s.backend.bc.port),
		redis.NewBulkBytes([]byte("3000")),
		redis.NewBulkBytes(hkey),
	}
	m.Batch = &sync.WaitGroup{}

	s.migrate.bc.BackendConn(r.Seed16(), true).PushBack(m)

	m.Batch.Wait()

	if err := m.Err; err != nil {
		return err
	}
	switch resp := m.Resp; {
	case resp == nil:
		return ErrRespIsRequired
	case resp.IsError():
		return fmt.Errorf("bad slotsmgrt resp: %s", resp.Value)
	case resp.IsInt():
		log.Debugf("slot-%04d migrate from %s to %s: hash key = %s, resp = %s",
			s.id, s.migrate.bc.Addr(), s.backend.bc.Addr(), hkey, resp.Value)
		return nil
	default:
		return fmt.Errorf("bad slotsmgrt resp: should be integer, but got %s", resp.Type)
	}
}

func (s *Slot) forward2(r *Request) (*BackendConn, error) {
	var seed = r.Seed16()
	if s.migrate.bc == nil && r.IsReadOnly() {
		for _, group := range s.replicaGroups {
			var i = seed
			for _ = range group {
				i = (i + 1) % uint(len(group))
				if bc := group[i].BackendConn(seed, false); bc != nil {
					return bc, nil
				}
			}
		}
	}
	return s.backend.bc.BackendConn(seed, true), nil
}
