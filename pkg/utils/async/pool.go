package async

import (
	"sync"
	"time"
)

const (
	DefaultPoolSize = 16
)

type Pool struct {
	queues []*jobQueue
}

func NewPool(n int) *Pool {
	if n <= 0 {
		n = DefaultPoolSize
	}
	p := &Pool{make([]*jobQueue, n)}
	for i := 0; i < n; i++ {
		q := newJobQueue()
		go func() {
			for {
				if f := q.RemoveFront(); f != nil {
					f()
				}
			}
		}()
		p.queues[i] = q
	}
	return p
}

func (p *Pool) Call(f func()) {
	qid := time.Now().Nanosecond()
	p.CallWithQueue(qid, f)
}

func (p *Pool) CallWithQueue(qid int, f func()) {
	p.queues[uint(qid)%uint(len(p.queues))].PushBack(f)
}

var defaultPool struct {
	pool *Pool
	init sync.Mutex
}

func lazyInit() *Pool {
	if p := defaultPool.pool; p != nil {
		return p
	} else {
		defaultPool.init.Lock()
		if defaultPool.pool == nil {
			defaultPool.pool = NewPool(DefaultPoolSize)
		}
		p = defaultPool.pool
		defaultPool.init.Unlock()
		return p
	}
}

func Call(f func()) {
	dq := lazyInit()
	dq.Call(f)
}

func CallWithQueue(qid int, f func()) {
	dq := lazyInit()
	dq.CallWithQueue(qid, f)
}
