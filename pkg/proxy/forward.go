// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"fmt"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

type forwardMethod interface {
	GetId() int
	Forward(s *Slot, r *Request, hkey []byte) error
}

var (
	ErrSlotIsNotReady = errors.New("slot is not ready, may be offline")
	ErrRespIsRequired = errors.New("resp is required")
)

type forwardSync struct {
	forwardHelper
}

func (d *forwardSync) GetId() int {
	return models.ForwardSync
}

func (d *forwardSync) Forward(s *Slot, r *Request, hkey []byte) error {
	s.lock.RLock()
	bc, err := d.process(s, r, hkey)
	s.lock.RUnlock()
	if err != nil {
		return err
	}
	bc.PushBack(r)
	return nil
}

func (d *forwardSync) process(s *Slot, r *Request, hkey []byte) (*BackendConn, error) {
	if s.backend.bc == nil {
		log.Debugf("slot-%04d is not ready: hash key = '%s'",
			s.id, hkey)
		return nil, ErrSlotIsNotReady
	}
	if s.migrate.bc != nil && len(hkey) != 0 {
		if err := d.slotsmgrt(s, hkey, r.Database, r.Seed16()); err != nil {
			log.Debugf("slot-%04d migrate from = %s to %s failed: hash key = '%s', database = %d, error = %s",
				s.id, s.migrate.bc.Addr(), s.backend.bc.Addr(), hkey, r.Database, err)
			return nil, err
		}
	}
	r.Group = &s.refs
	r.Group.Add(1)
	return d.forward2(s, r), nil
}

type forwardSemiAsync struct {
	forwardHelper
}

func (d *forwardSemiAsync) GetId() int {
	return models.ForwardSemiAsync
}

func (d *forwardSemiAsync) Forward(s *Slot, r *Request, hkey []byte) error {
	var loop int
	for {
		s.lock.RLock()
		bc, retry, err := d.process(s, r, hkey)
		s.lock.RUnlock()

		switch {
		case err != nil:
			return err
		case !retry:
			if bc != nil {
				bc.PushBack(r)
			}
			return nil
		}

		var delay time.Duration
		switch {
		case loop < 5:
			delay = 0
		case loop < 20:
			delay = time.Millisecond * time.Duration(loop)
		default:
			delay = time.Millisecond * 20
		}
		time.Sleep(delay)

		if r.IsBroken() {
			return ErrRequestIsBroken
		}
		loop += 1
	}
}

func (d *forwardSemiAsync) process(s *Slot, r *Request, hkey []byte) (_ *BackendConn, retry bool, _ error) {
	if s.backend.bc == nil {
		log.Debugf("slot-%04d is not ready: hash key = '%s'",
			s.id, hkey)
		return nil, false, ErrSlotIsNotReady
	}
	if s.migrate.bc != nil && len(hkey) != 0 {
		resp, moved, err := d.slotsmgrtExecWrapper(s, hkey, r.Database, r.Seed16(), r.Multi)
		switch {
		case err != nil:
			log.Debugf("slot-%04d migrate from = %s to %s failed: hash key = '%s', error = %s",
				s.id, s.migrate.bc.Addr(), s.backend.bc.Addr(), hkey, err)
			return nil, false, err
		case !moved:
			switch {
			case resp != nil:
				r.Resp = resp
				return nil, false, nil
			}
			return nil, true, nil
		}
	}
	r.Group = &s.refs
	r.Group.Add(1)
	return d.forward2(s, r), false, nil
}

type forwardHelper struct {
}

func (d *forwardHelper) slotsmgrt(s *Slot, hkey []byte, database int32, seed uint) error {
	m := &Request{}
	m.Multi = []*redis.Resp{
		redis.NewBulkBytes([]byte("SLOTSMGRTTAGONE")),
		redis.NewBulkBytes(s.backend.bc.host),
		redis.NewBulkBytes(s.backend.bc.port),
		redis.NewBulkBytes([]byte("3000")),
		redis.NewBulkBytes(hkey),
	}
	m.Batch = &sync.WaitGroup{}

	s.migrate.bc.BackendConn(database, seed, true).PushBack(m)

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
		log.Debugf("slot-%04d migrate from %s to %s: hash key = %s, database = %d, resp = %s",
			s.id, s.migrate.bc.Addr(), s.backend.bc.Addr(), hkey, database, resp.Value)
		return nil
	default:
		return fmt.Errorf("bad slotsmgrt resp: should be integer, but got %s", resp.Type)
	}
}

func (d *forwardHelper) slotsmgrtExecWrapper(s *Slot, hkey []byte, database int32, seed uint, multi []*redis.Resp) (_ *redis.Resp, moved bool, _ error) {
	m := &Request{}
	m.Multi = make([]*redis.Resp, 0, 2+len(multi))
	m.Multi = append(m.Multi,
		redis.NewBulkBytes([]byte("SLOTSMGRT-EXEC-WRAPPER")),
		redis.NewBulkBytes(hkey),
	)
	m.Multi = append(m.Multi, multi...)
	m.Batch = &sync.WaitGroup{}

	s.migrate.bc.BackendConn(database, seed, true).PushBack(m)

	m.Batch.Wait()

	if err := m.Err; err != nil {
		return nil, false, err
	}
	switch resp := m.Resp; {
	case resp == nil:
		return nil, false, ErrRespIsRequired
	case resp.IsError():
		return nil, false, fmt.Errorf("bad slotsmgrt-exec-wrapper resp: %s", resp.Value)
	case resp.IsArray():
		if len(resp.Array) != 2 {
			return nil, false, fmt.Errorf("bad slotsmgrt-exec-wrapper resp: array.len = %d",
				len(resp.Array))
		}
		if !resp.Array[0].IsInt() || len(resp.Array[0].Value) != 1 {
			return nil, false, fmt.Errorf("bad slotsmgrt-exec-wrapper resp: type(array[0]) = %s, len(array[0].value) = %d",
				resp.Array[0].Type, len(resp.Array[0].Value))
		}
		switch resp.Array[0].Value[0] - '0' {
		case 0:
			return nil, true, nil
		case 1:
			return nil, false, nil
		case 2:
			return resp.Array[1], false, nil
		default:
			return nil, false, fmt.Errorf("bad slotsmgrt-exec-wrapper resp: [%s] %s",
				resp.Array[0].Value, resp.Array[1].Value)
		}
	default:
		return nil, false, fmt.Errorf("bad slotsmgrt-exec-wrapper resp: should be integer, but got %s", resp.Type)
	}
}

func (d *forwardHelper) forward2(s *Slot, r *Request) *BackendConn {
	var database, seed = r.Database, r.Seed16()
	if s.migrate.bc == nil && !r.IsMasterOnly() && len(s.replicaGroups) != 0 {
		for _, group := range s.replicaGroups {
			var i = seed
			for range group {
				i = (i + 1) % uint(len(group))
				if bc := group[i].BackendConn(database, seed, false); bc != nil {
					return bc
				}
			}
		}
	}
	return s.backend.bc.BackendConn(database, seed, true)
}
