// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package pipe

import (
	"os"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

type rfileBuffer struct {
	m    *memBuffer
	f    *os.File
	size uint64
	rpos uint64
	wpos uint64
}

func newRFileBuffer(buffSize, fileSize int, f *os.File) *rfileBuffer {
	m := newMemBuffer(buffSize)
	n := align(fileSize, FileSizeAlign)
	return &rfileBuffer{m: m, f: f, size: uint64(n)}
}

func (p *rfileBuffer) read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if p.m.wpos == p.m.rpos {
		if len(b) >= len(p.m.b) {
			return p.readFromFile(b)
		} else {
			n, err := p.readFromFile(p.m.b[:p.m.size])
			p.m.rpos, p.m.wpos = 0, uint64(n)
			if err != nil || n == 0 {
				return 0, err
			}
		}
	}
	return p.m.read(b)
}

func (p *rfileBuffer) write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if p.rpos != p.wpos {
		return p.writeToFile(b)
	}
	if p.m.wpos != p.m.rpos+p.m.size {
		return p.m.write(b)
	} else {
		return p.writeToFile(b)
	}
}

func (p *rfileBuffer) readFromFile(b []byte) (int, error) {
	if len(b) == 0 || p.f == nil {
		return 0, nil
	}
	maxlen, offset := roffset(len(b), p.size, p.rpos, p.wpos)
	if maxlen == 0 {
		return 0, nil
	}
	n, err := p.f.ReadAt(b[:maxlen], int64(offset))
	p.rpos += uint64(n)
	if p.rpos == p.wpos {
		p.rpos = 0
		p.wpos = 0
		if err == nil {
			err = p.f.Truncate(0)
		}
	}
	return n, errors.Trace(err)
}

func (p *rfileBuffer) writeToFile(b []byte) (int, error) {
	if len(b) == 0 || p.f == nil {
		return 0, nil
	}
	maxlen, offset := woffset(len(b), p.size, p.rpos, p.wpos)
	if maxlen == 0 {
		return 0, nil
	}
	n, err := p.f.WriteAt(b[:maxlen], int64(offset))
	p.wpos += uint64(n)
	return n, errors.Trace(err)
}

func (p *rfileBuffer) buffered() int {
	n := p.m.wpos - p.m.rpos
	return int(p.wpos - p.rpos + n)
}

func (p *rfileBuffer) available() int {
	return int(p.size + p.rpos - p.wpos)
}

func (p *rfileBuffer) rclose() error {
	err := p.m.rclose()
	if p.f != nil {
		p.f.Truncate(0)
		if err := p.f.Close(); err != nil {
			return errors.Trace(err)
		}
	}
	return err
}

func (p *rfileBuffer) wclose() error {
	return p.m.wclose()
}
