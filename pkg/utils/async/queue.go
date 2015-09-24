package async

import (
	"container/list"
	"sync"
)

type jobQueue struct {
	lock sync.Mutex
	list list.List
	cond *sync.Cond
}

func newJobQueue() *jobQueue {
	q := &jobQueue{}
	q.cond = sync.NewCond(&q.lock)
	return q
}

func (q *jobQueue) PushBack(f func()) {
	if f == nil {
		return
	}
	q.lock.Lock()
	q.list.PushBack(f)
	q.cond.Signal()
	q.lock.Unlock()
}

func (q *jobQueue) RemoveFront() (f func()) {
	q.lock.Lock()
	for f == nil {
		if e := q.list.Front(); e != nil {
			f = q.list.Remove(e).(func())
		} else {
			q.cond.Wait()
		}
	}
	q.lock.Unlock()
	return f
}
