# jemalloc
[![Build Status](https://travis-ci.org/spinlock/jemalloc-go.svg)](https://travis-ci.org/spinlock/jemalloc-go)

```go
package demo

// #cgo         CPPFLAGS: -I <relative-path>/jemalloc-go/jemalloc/include
// #cgo  darwin LDFLAGS: -Wl,-undefined -Wl,dynamic_lookup
// #cgo !darwin LDFLAGS: -Wl,-unresolved-symbols=ignore-all
// #include <jemalloc/jemalloc.h>
import "C"
import _ "github.com/spinlock/jemalloc-go"

func malloc(n int) unsafe.Pointer {
    return C.je_malloc(C.size_t(n))
}

func free(p unsafe.Pointer) {
    C.je_free(p)
}
```

#### How to setup & install
```bash
$ mkdir -p $GOPATH/src/github.com/spinlock
$ cd $_
$ git clone https://github.com/spinlock/jemalloc-go.git
$ cd jemalloc-go
$ make install
```
