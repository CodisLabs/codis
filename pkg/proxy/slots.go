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

	backend, migrate struct {
		id int
		bc *SharedBackendConn
	}
	replicaGroups [][]*SharedBackendConn
}

func (s *Slot) snapshot(replica bool) *models.Slot {
	var m = &models.Slot{
		Id:     s.id,
		Locked: s.lock.hold,

		BackendAddr:   s.backend.bc.Addr(),
		BackendAddrId: s.backend.id,
		MigrateFrom:   s.migrate.bc.Addr(),
		MigrateFromId: s.migrate.id,
	}
	if !replica {
		return m
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

func (s *Slot) prepare(r *Request, hkey []byte) (*SharedBackendConn, error) {
	if s.backend.bc == nil {
		log.Warnf("slot-%04d is not ready: hkey = %s", s.id, hkey)
		return nil, ErrSlotIsNotReady
	}
	if err := s.slotsmgrt(r, hkey); err != nil {
		log.Warnf("slot-%04d migrate from = %s to %s failed: hkey = %s, error = %s",
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

	s.migrate.bc.PushBack(m)

	m.Batch.Wait()

	if err := m.Err; err != nil {
		return err
	}
	switch resp := m.Resp; {
	case resp == nil:
		return ErrRespIsRequired
	case resp.IsError():
		return fmt.Errorf("error resp: %s", resp.Value)
	case resp.IsInt():
		log.Debugf("slot-%04d migrate from %s to %s: hkey = %s, resp = %s",
			s.id, s.migrate.bc.Addr(), s.backend.bc.Addr(), hkey, resp.Value)
		return nil
	default:
		return fmt.Errorf("error resp: should be integer, but got %s", resp.Type)
	}
}

func (s *Slot) forward2(r *Request) (*SharedBackendConn, error) {
	if s.migrate.bc != nil || !r.IsReadOnly() {
		return s.backend.bc, nil
	}
	seed := uint(r.Start)
	for _, group := range s.replicaGroups {
		for i := 0; i < len(group); i++ {
			index := (seed + uint(i)) % uint(len(group))
			if bc := group[index]; bc != nil {
				if bc.IsConnected() {
					return bc, nil
				}
			}
		}
	}
	return s.backend.bc, nil
}
