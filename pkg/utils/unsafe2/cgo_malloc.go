// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

// +build !cgo_jemalloc

package unsafe2

// #include <stdlib.h>
import "C"

import (
	"unsafe"
)

func cgo_malloc(n int) unsafe.Pointer {
	return C.malloc(C.size_t(n))
}

func cgo_free(ptr unsafe.Pointer) {
	C.free(ptr)
}
