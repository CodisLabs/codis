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
	jobs sync.WaitGroup
}
