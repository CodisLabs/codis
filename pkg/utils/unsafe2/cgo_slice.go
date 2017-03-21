// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package unsafe2

import (
	"reflect"
	"runtime"
	"unsafe"

	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
)

var allocOffheapBytes atomic2.Int64

func OffheapBytes() int64 {
	return allocOffheapBytes.Int64()
}

type cgoSlice struct {
	ptr unsafe.Pointer
	buf []byte
}

func newCGoSlice(n int, force bool) Slice {
	after := allocOffheapBytes.Add(int64(n))
	if !force && after > MaxOffheapBytes() {
		allocOffheapBytes.Sub(int64(n))
		return nil
	}
	p := cgo_malloc(n)
	if p == nil {
		allocOffheapBytes.Sub(int64(n))
		return nil
	}
	s := &cgoSlice{
		ptr: p,
		buf: *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(p), Len: n, Cap: n,
		})),
	}
	runtime.SetFinalizer(s, (*cgoSlice).reclaim)
	return s
}

func (s *cgoSlice) Type() string {
	return "cgo_slice"
}

func (s *cgoSlice) Buffer() []byte {
	return s.buf
}

func (s *cgoSlice) reclaim() {
	if s.ptr == nil {
		return
	}
	cgo_free(s.ptr)
	allocOffheapBytes.Sub(int64(len(s.buf)))
	s.ptr = nil
	s.buf = nil
	runtime.SetFinalizer(s, nil)
}

func (s *cgoSlice) Slice2(beg, end int) Slice {
	return newGoSliceFrom(s, s.Buffer()[beg:end])
}

func (s *cgoSlice) Slice3(beg, end, cap int) Slice {
	return newGoSliceFrom(s, s.Buffer()[beg:end:cap])
}

func (s *cgoSlice) Parent() Slice {
	return nil
}
