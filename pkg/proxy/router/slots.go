package router

import (
	"strings"
	"sync"

	"github.com/wandoulabs/codis/pkg/models"
)

type Slot struct {
	Id    int
	Info  *models.Slot
	Group *models.ServerGroup

	from string
	addr struct {
		host []byte
		port []byte
	}
	migrating bool

	bc   *BackendConn
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

func (s *Slot) update(addr, from string, bc *BackendConn) *BackendConn {
	s.addr.host = nil
	s.addr.port = nil
	s.from = ""
	if len(addr) != 0 {
		ss := strings.Split(addr, ":")
		if len(ss) >= 1 {
			s.addr.host = []byte(ss[0])
		}
		if len(ss) >= 2 {
			s.addr.port = []byte(ss[1])
		}
	}
	s.migrating = len(from) != 0
	s.bc, bc = bc, s.bc
	return bc
}

func (s *Slot) forward(r *Request) error {
	s.lock.RLock()
	s.jobs.Add(1)
	s.lock.RUnlock()
	panic("not finish yet")
}
