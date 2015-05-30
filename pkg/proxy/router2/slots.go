package router

import "sync"

type Slot struct {
	Id int

	jobs sync.WaitGroup
}
