// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package iocount

import (
	"io"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
)

type Reader struct {
	p *atomic2.AtomicInt64
	r io.Reader
}

func NewReader(r io.Reader) *Reader {
	return NewReaderWithCounter(r, nil)
}

func NewReaderWithCounter(r io.Reader, p *atomic2.AtomicInt64) *Reader {
	if p == nil {
		p = &atomic2.AtomicInt64{}
	}
	return &Reader{p: p, r: r}
}

func (r *Reader) Count() int64 {
	return r.p.Get()
}

func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.p.Add(int64(n))
	return n, err
}
