// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package atomic2

import "sync/atomic"

type AtomicInt64 struct {
	v, s int64
}

func (a *AtomicInt64) Get() int64 {
	return atomic.LoadInt64(&a.v)
}

func (a *AtomicInt64) Set(v int64) {
	atomic.StoreInt64(&a.v, v)
}

func (a *AtomicInt64) Reset() int64 {
	return atomic.SwapInt64(&a.v, 0)
}

func (a *AtomicInt64) Add(v int64) int64 {
	return atomic.AddInt64(&a.v, v)
}

func (a *AtomicInt64) Sub(v int64) int64 {
	return a.Add(-v)
}

func (a *AtomicInt64) Snapshot() {
	a.s = a.Get()
}

func (a *AtomicInt64) Delta() int64 {
	return a.Get() - a.s
}

func (a *AtomicInt64) Incr() int64 {
	return a.Add(1)
}

func (a *AtomicInt64) Decr() int64 {
	return a.Add(-1)
}
