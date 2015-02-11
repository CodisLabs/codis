package gorocks

// #include "rocksdb/c.h"
import "C"

func boolToUchar(b bool) C.uchar {
	uc := C.uchar(0)
	if b {
		uc = C.uchar(1)
	}
	return uc
}

func ucharToBool(uc C.uchar) bool {
	if uc == C.uchar(0) {
		return false
	}
	return true
}

func boolToInt(b bool) C.int {
	if b {
		return C.int(1)
	} else {
		return C.int(0)
	}
}
