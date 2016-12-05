// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

// +build !darwin

package unsafe2

// #cgo         CPPFLAGS: -I ../../../vendor/github.com/spinlock/jemalloc-go/jemalloc/include
// #cgo  darwin LDFLAGS: -Wl,-undefined -Wl,dynamic_lookup
// #cgo !darwin LDFLAGS: -Wl,-unresolved-symbols=ignore-all
// #include <jemalloc/jemalloc.h>
import "C"

import (
	"unsafe"

	_ "github.com/spinlock/jemalloc-go"
)

func cgo_malloc(n int) unsafe.Pointer {
	return C.je_malloc(C.size_t(n))
}

func cgo_free(ptr unsafe.Pointer) {
	C.je_free(ptr)
}
