package gorocks

/*
#cgo LDFLAGS: -lrocksdb
#include "rocksdb/c.h"
*/
import "C"

func GetrocksdbMajorVersion() int {
	// return int(C.rocksdb_major_version())
	return 1
}

func GetrocksdbMinorVersion() int {
	// return int(C.rocksdb_minor_version())
	return 0
}
