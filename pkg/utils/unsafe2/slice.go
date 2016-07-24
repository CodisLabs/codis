package unsafe2

import (
	"github.com/CodisLabs/codis/pkg/utils/math2"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
)

type Slice interface {
	Buffer() []byte
	reclaim()
}

var maxOffheapBytes atomic2.Int64

func MaxOffheapBytes() int {
	return int(maxOffheapBytes.Get())
}

func SetMaxOffheapBytes(n int) {
	maxOffheapBytes.Set(int64(math2.MaxInt(n, 0)))
}

const MinOffheapSlice = 1024 * 16

func MakeSlice(n int) Slice {
	if n >= MinOffheapSlice {
		if s := newJeSlice(n, false); s != nil {
			return s
		}
	}
	return newGoSlice(n)
}

func MakeOffheapSlice(n int) Slice {
	if n >= 0 {
		return newJeSlice(n, true)
	}
	panic("make slice with negative size")
}

func FreeSlice(s Slice) {
	if s != nil {
		s.reclaim()
	}
}
