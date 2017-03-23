// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package atomic2

type Bool struct {
	c Int64
}

func (b *Bool) Bool() bool {
	return b.IsTrue()
}

func (b *Bool) IsTrue() bool {
	return b.c.Int64() != 0
}

func (b *Bool) IsFalse() bool {
	return b.c.Int64() == 0
}

func (b *Bool) toInt64(v bool) int64 {
	if v {
		return 1
	} else {
		return 0
	}
}

func (b *Bool) Set(v bool) {
	b.c.Set(b.toInt64(v))
}

func (b *Bool) CompareAndSwap(o, n bool) bool {
	return b.c.CompareAndSwap(b.toInt64(o), b.toInt64(n))
}

func (b *Bool) Swap(v bool) bool {
	return b.c.Swap(b.toInt64(v)) != 0
}
