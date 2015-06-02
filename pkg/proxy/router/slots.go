package router

import (
	"sync"

	"github.com/wandoulabs/codis/pkg/models"
)

type Slot struct {
	Id    int
	Info  *models.Slot
	Group *models.ServerGroup

	bc   *BackendConn
	addr struct {
		host []byte
		port []byte
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
	s.from = ""
	return bc
}

func (s *Slot) forward(r *Request) error {
	s.lock.RLock()
	s.jobs.Add(1)
	s.lock.RUnlock()
	panic("not finish yet")
}
