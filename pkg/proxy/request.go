// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"sync"

	"github.com/CodisLabs/codis/pkg/proxy/redis"
)

type Request struct {
	OpStr string
	Multi []*redis.Resp

	Start int64
	Batch *sync.WaitGroup
	Group *sync.WaitGroup
	Dirty bool

	Coalesce func() error
	Response struct {
		Resp *redis.Resp
		Err  error
	}
}

type RequestAlloc struct {
	alloc struct {
		buf []Request
		off int
	}
	batch struct {
		buf []sync.WaitGroup
		off int
	}
}

func (p *RequestAlloc) New() *Request {
	var d = &p.alloc
	if len(d.buf) == d.off {
		d.buf = make([]Request, 64)
		d.off = 0
	}
	r := &d.buf[d.off]
	d.off += 1
	return r
}

func (p *RequestAlloc) NewBatch() *sync.WaitGroup {
	var d = &p.batch
	if len(d.buf) == d.off {
		d.buf = make([]sync.WaitGroup, 64)
		d.off = 0
	}
	w := &d.buf[d.off]
	d.off += 1
	return w
}
