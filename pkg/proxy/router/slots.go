package router

import (
	"sync"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/group"
)

type Slot struct {
	Id int

	Info  *models.Slot
	Group *models.ServerGroup

	dst, src struct {
		group *group.Group
	}
	lock sync.RWMutex
	jobs sync.WaitGroup
}

func (s *Slot) Acquire() {
	s.lock.Lock()
}

func (s *Slot) Release() {
	s.lock.Unlock()
}
