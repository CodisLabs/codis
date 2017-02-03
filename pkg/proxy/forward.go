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
		if err := d.slotsmgrt(s, hkey, r.Seed16()); err != nil {
			log.Debugf("slot-%04d migrate from = %s to %s failed: hash key = '%s', error = %s",
				s.id, s.migrate.bc.Addr(), s.backend.bc.Addr(), hkey, err)
			return nil, err
		}
	}
	r.Group = &s.refs
	r.Group.Add(1)
	return d.forward2(s, r, r.Seed16()), nil
}

type forwardSemiAsync struct {
	forwardHelper
}

func (d *forwardSemiAsync) GetId() int {
	return models.ForwardSemiAsync
}

func (d *forwardSemiAsync) Forward(s *Slot, r *Request, hkey []byte) error {
	s.lock.RLock()
	bc, done, err := d.process(s, r, hkey)
	s.lock.RUnlock()
	if err != nil || done {
		return err
	}
	bc.PushBack(r)
	return nil
}

func (d *forwardSemiAsync) process(s *Slot, r *Request, hkey []byte) (*BackendConn, bool, error) {
	if s.backend.bc == nil {
		log.Debugf("slot-%04d is not ready: hash key = '%s'",
			s.id, hkey)
		return nil, false, ErrSlotIsNotReady
	}
	if s.migrate.bc != nil && len(hkey) != 0 {
		if done, err := d.slotsmgrtExecWrapperUntil(s, hkey, r); err != nil {
			log.Debugf("slot-%04d migrate from = %s to %s failed: hash key = '%s', error = %s",
				s.id, s.migrate.bc.Addr(), s.backend.bc.Addr(), hkey, err)
			return nil, false, err
		} else if done {
			return nil, true, nil
		}
	}
	r.Group = &s.refs
	r.Group.Add(1)
	return d.forward2(s, r, r.Seed16()), false, nil
}

func (d *forwardSemiAsync) slotsmgrtExecWrapperUntil(s *Slot, hkey []byte, r *Request) (bool, error) {
	for i := 0; !r.IsBroken(); i++ {
		resp, redirect, err := d.slotsmgrtExecWrapper(s, hkey, r.Seed16(), r.Multi)
		switch {
		case err != nil || redirect:
			return false, err
		case resp != nil:
			r.Resp = resp
			return true, nil
		}
		if i < 5 {
			continue
		}
		var d time.Duration
		switch {
		case i < 10:
			d = time.Millisecond * 10
		case i < 50:
			d = time.Millisecond * time.Duration(i)
		default:
			d = time.Millisecond * 100
		}
		time.Sleep(d)
	}
	return false, ErrRequestIsBroken
}

type forwardHelper struct {
}

func (d *forwardHelper) slotsmgrt(s *Slot, hkey []byte, seed uint) error {
	m := &Request{}
	m.Multi = []*redis.Resp{
		redis.NewBulkBytes([]byte("SLOTSMGRTTAGONE")),
		redis.NewBulkBytes(s.backend.bc.host),
		redis.NewBulkBytes(s.backend.bc.port),
		redis.NewBulkBytes([]byte("3000")),
		redis.NewBulkBytes(hkey),
	}
	m.Batch = &sync.WaitGroup{}

	s.migrate.bc.BackendConn(seed, true).PushBack(m)

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

func (d *forwardHelper) slotsmgrtExecWrapper(s *Slot, hkey []byte, seed uint, multi []*redis.Resp) (_ *redis.Resp, redirect bool, _ error) {
	m := &Request{}
	m.Multi = make([]*redis.Resp, 0, 2+len(multi))
	m.Multi = append(m.Multi,
		redis.NewBulkBytes([]byte("SLOTSMGRT-EXEC-WRAPPER")),
		redis.NewBulkBytes(hkey),
	)
	m.Multi = append(m.Multi, multi...)
	m.Batch = &sync.WaitGroup{}

	s.migrate.bc.BackendConn(seed, true).PushBack(m)

	m.Batch.Wait()

	if err := m.Err; err != nil {
		return nil, false, err
	}
	switch resp := m.Resp; {
	case resp == nil:
		return nil, false, ErrRespIsRequired
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

func (d *forwardHelper) forward2(s *Slot, r *Request, seed uint) *BackendConn {
	if s.migrate.bc == nil && r.IsReadOnly() {
		for _, group := range s.replicaGroups {
			var i = seed
			for _ = range group {
				i = (i + 1) % uint(len(group))
				if bc := group[i].BackendConn(seed, false); bc != nil {
					return bc
				}
			}
		}
	}
	return s.backend.bc.BackendConn(seed, true)
}
