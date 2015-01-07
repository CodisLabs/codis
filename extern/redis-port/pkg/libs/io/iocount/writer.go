// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package iocount

import (
	"io"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/atomic2"
)

type Writer struct {
	p *atomic2.AtomicInt64
	w io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return NewWriterWithCounter(w, nil)
}

func NewWriterWithCounter(w io.Writer, p *atomic2.AtomicInt64) *Writer {
	if p == nil {
		p = &atomic2.AtomicInt64{}
	}
	return &Writer{p: p, w: w}
}

func (w *Writer) Count() int64 {
	return w.p.Get()
}

func (w *Writer) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.p.Add(int64(n))
	return n, err
}
