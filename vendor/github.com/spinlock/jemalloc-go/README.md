# jemalloc
[![Build Status](https://travis-ci.org/spinlock/jemalloc-go.svg)](https://travis-ci.org/spinlock/jemalloc-go)

#### How to setup & install
```bash
$ mkdir -p $GOPATH/src/github.com/spinlock
$ cd $_
$ git clone https://github.com/spinlock/jemalloc-go.git
$ cd jemalloc-go
$ make install
```

#### How to use it

```go
package demo

// #cgo         CPPFLAGS: -I<relative-path>/jemalloc-go
// #cgo  darwin  LDFLAGS: -Wl,-undefined -Wl,dynamic_lookup
// #cgo !darwin  LDFLAGS: -Wl,-unresolved-symbols=ignore-all
// #include <jemalloc/jemalloc.h>
import "C"

import jemalloc "github.com/spinlock/jemalloc-go"

func malloc1(n int) unsafe.Pointer {
    return C.je_malloc(C.size_t(n))
}

func free1(p unsafe.Pointer) {
    C.je_free(p)
}

func malloc2(n int) unsafe.Pointer {
    return jemalloc.Malloc(n)
}

func free2(p unsafe.Pointer) {
    jemalloc.Free(p)
}
```

