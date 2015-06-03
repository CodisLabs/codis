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

	bc   *BackendConn
	addr struct {
		host []byte
		port []byte
		full string
	}
	from string

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

func (s *Slot) reset() (bc *BackendConn) {
	s.Info, s.Group = nil, nil
	s.bc, bc = nil, s.bc
	s.addr.host = nil
	s.addr.port = nil
	s.addr.full = ""
	s.from = ""
	return bc
}

func (s *Slot) forward(r *Request, key []byte) error {
	s.lock.RLock()
	bc, err := s.prepare(r, key)
	s.lock.RUnlock()
	if err != nil {
		return err
	} else {
		r.wait.Add(1)
		bc.PushBack(r)
		return nil
	}
}

var ErrSlotIsNotReady = errors.New("slot is not ready, may be offline")

func (s *Slot) prepare(r *Request, key []byte) (*BackendConn, error) {
	if s.bc == nil {
		log.Infof("slot-%04d is not ready: from = %s, addr = %s, key = %s",
			s.Id, s.from, s.addr.full, key)
		return nil, ErrSlotIsNotReady
	}
	if len(s.from) != 0 {
		if n, err := redis.SlotsMgrtTagOne(s.from, s.addr.host, s.addr.port, key); err != nil {
			log.InfoErrorf(err, "slot-%04d slotsmgrttagone from %s to %s error, key = %s",
				s.Id, s.from, s.addr.full, key)
			return nil, err
		} else {
			log.Debugf("slot-%04d slotsmgrttagone from %s to %s: n = %d, key = %s",
				s.Id, s.from, s.addr.full, n, key)
		}
	}
	s.jobs.Add(1)
	r.slot = s
	return s.bc, nil
}
