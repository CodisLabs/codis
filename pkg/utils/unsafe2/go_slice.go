// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package unsafe2

type goSlice struct {
	buf []byte

	parent Slice
}

func newGoSlice(n int) Slice {
	return &goSlice{
		buf: make([]byte, n),
	}
}

func newGoSliceFrom(parent Slice, buf []byte) Slice {
	return &goSlice{
		buf: buf, parent: parent,
	}
}

func (s *goSlice) Type() string {
	return "go_slice"
}

func (s *goSlice) Buffer() []byte {
	return s.buf
}

func (s *goSlice) reclaim() {
}

func (s *goSlice) Slice2(beg, end int) Slice {
	return newGoSliceFrom(s.parent, s.buf[beg:end])
}

func (s *goSlice) Slice3(beg, end, cap int) Slice {
	return newGoSliceFrom(s.parent, s.buf[beg:end:cap])
}

func (s *goSlice) Parent() Slice {
	return s.parent
}
