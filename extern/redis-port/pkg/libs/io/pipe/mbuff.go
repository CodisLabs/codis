// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package pipe

type memBuffer struct {
	b    []byte
	size uint64
	rpos uint64
	wpos uint64
}

func newMemBuffer(buffSize int) *memBuffer {
	n := align(buffSize, BuffSizeAlign)
	b := make([]byte, n)
	return &memBuffer{b: b, size: uint64(n)}
}

func (p *memBuffer) read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	maxlen, offset := roffset(len(b), p.size, p.rpos, p.wpos)
	if maxlen == 0 {
		return 0, nil
	}
	n := copy(b, p.b[offset:offset+maxlen])
	p.rpos += uint64(n)
	if p.rpos == p.wpos {
		p.rpos = 0
		p.wpos = 0
	}
	return n, nil
}

func (p *memBuffer) write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	maxlen, offset := woffset(len(b), p.size, p.rpos, p.wpos)
	if maxlen == 0 {
		return 0, nil
	}
	n := copy(p.b[offset:offset+maxlen], b)
	p.wpos += uint64(n)
	return n, nil
}

func (p *memBuffer) buffered() int {
	return int(p.wpos - p.rpos)
}

func (p *memBuffer) available() int {
	return int(p.size + p.rpos - p.wpos)
}

func (p *memBuffer) rclose() error {
	return nil
}

func (p *memBuffer) wclose() error {
	return nil
}
