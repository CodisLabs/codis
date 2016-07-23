package unsafe2

// #cgo         CPPFLAGS: -I ../../../vendor/github.com/spinlock/jemalloc-go/jemalloc/include
// #cgo  darwin LDFLAGS: -Wl,-undefined -Wl,dynamic_lookup
// #cgo !darwin LDFLAGS: -Wl,-unresolved-symbols=ignore-all
// #include <jemalloc/jemalloc.h>
import "C"

import _ "github.com/spinlock/jemalloc-go"
