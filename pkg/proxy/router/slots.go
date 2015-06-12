// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"fmt"
	"sync"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/redis"
	"github.com/wandoulabs/codis/pkg/utils/errors"
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

const SlotsMgrtTagOne = "SLOTSMGRTTAGONE"

func (s *Slot) prepare(r *Request, key []byte) (*SharedBackendConn, error) {
	if s.backend.bc == nil {
		log.Infof("slot-%04d is not ready: key = %s", s.Id, key)
		return nil, ErrSlotIsNotReady
	}
	if err := s.slotsmgrt(r, key); err != nil {
		log.Warnf("slot-%04d migrate from = %s to %s failed: key = %s, error = %s",
			s.Id, s.migrate.from, s.backend.addr, key, err)
		return nil, err
	} else {
		r.slot = s
		r.Wait.Add(1)
		s.jobs.Add(1)
		return s.backend.bc, nil
	}
}

func (s *Slot) slotsmgrt(r *Request, key []byte) error {
	if len(key) == 0 || s.migrate.bc == nil {
		return nil
	}
	m := &Request{
		Owner: r.Owner,
		OpSeq: -r.OpSeq,
		OpStr: SlotsMgrtTagOne,
		Start: r.Start,
		Wait:  &sync.WaitGroup{},
		Resp: redis.NewArray([]*redis.Resp{
			redis.NewBulkBytes([]byte(SlotsMgrtTagOne)),
			redis.NewBulkBytes(s.backend.host),
			redis.NewBulkBytes(s.backend.port),
			redis.NewBulkBytes([]byte("3000")),
			redis.NewBulkBytes(key),
		}),
	}
	m.Wait.Add(1)

	s.migrate.bc.PushBack(m)

	m.Wait.Wait()

	resp, err := m.Response.Resp, m.Response.Err
	if err != nil {
		return err
	}
	if resp == nil {
		return ErrRespIsRequired
	}
	if resp.IsError() {
		return errors.New(fmt.Sprintf("error resp: %s", resp.Value))
	}
	if resp.IsInt() {
		log.Debugf("slot-%04d migrate from %s to %s: key = %s, resp = %s",
			s.Id, s.migrate.from, s.backend.addr, key, resp.Value)
		return nil
	} else {
		return errors.New(fmt.Sprintf("error resp: should be integer, but got %s", resp.Type))
	}
}
