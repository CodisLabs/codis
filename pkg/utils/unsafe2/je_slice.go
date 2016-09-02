// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package unsafe2

// #cgo         CPPFLAGS: -I ../../../vendor/github.com/spinlock/jemalloc-go/jemalloc/include
// #cgo  darwin LDFLAGS: -Wl,-undefined -Wl,dynamic_lookup
// #cgo !darwin LDFLAGS: -Wl,-unresolved-symbols=ignore-all
// #include <jemalloc/jemalloc.h>
import "C"

import (
	"reflect"
	"runtime"
	"unsafe"

	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"

	_ "github.com/spinlock/jemalloc-go"
)

var allocOffheapBytes atomic2.Int64

func OffheapBytes() int {
	return int(allocOffheapBytes.Get())
}

type jeSlice struct {
	ptr unsafe.Pointer
	buf []byte
}

func newJeSlice(n int, force bool) Slice {
	after := int(allocOffheapBytes.Add(int64(n)))
	if !force && after > MaxOffheapBytes() {
		allocOffheapBytes.Sub(int64(n))
		return nil
	}
	p := C.je_malloc(C.size_t(n))
	if p == nil {
		allocOffheapBytes.Sub(int64(n))
		return nil
	}
	s := &jeSlice{
		ptr: p,
		buf: *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(p), Len: n, Cap: n,
		})),
	}
	runtime.SetFinalizer(s, (*jeSlice).reclaim)
	return s
}

func (s *jeSlice) Buffer() []byte {
	return s.buf
}

func (s *jeSlice) reclaim() {
	if s.ptr == nil {
		return
	}
	C.je_free(s.ptr)
	allocOffheapBytes.Sub(int64(len(s.buf)))
	s.ptr = nil
	s.buf = nil
	runtime.SetFinalizer(s, nil)
}
