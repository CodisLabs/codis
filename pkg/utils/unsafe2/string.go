// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package unsafe2

import (
	"reflect"
	"unsafe"
)

func CastString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	var ptr = (*reflect.SliceHeader)(unsafe.Pointer(&b))
	var h = &reflect.StringHeader{
		Data: uintptr(ptr.Data), Len: ptr.Len,
	}
	return *(*string)(unsafe.Pointer(h))
}
