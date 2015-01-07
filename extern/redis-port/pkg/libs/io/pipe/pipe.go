// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package pipe

import (
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/utils/bytesize"
)

type pipe struct {
	rl sync.Mutex
	wl sync.Mutex
	mu sync.Mutex

	rwait *sync.Cond
	wwait *sync.Cond

	rerr error
	werr error

	buff struct {
		p    []byte
		size uint64
		rpos uint64
		wpos uint64
	}
	file struct {
		f    *os.File
		size uint64
		rpos uint64
		wpos uint64
	}
}

func newPipe(buffSize, fileSize uint64, f *os.File) (*PipeReader, *PipeWriter) {
	p := &pipe{}
	p.rwait = sync.NewCond(&p.mu)
	p.wwait = sync.NewCond(&p.mu)
	p.buff.p = make([]byte, buffSize)
	p.buff.size = buffSize
	p.file.f = f
	p.file.size = fileSize
	r := &PipeReader{p}
	w := &PipeWriter{p}
	return r, w
}

func roffset(blen int, size, rpos, wpos uint64) (maxlen, offset uint64) {
	maxlen = uint64(blen)
	if n := wpos - rpos; n < maxlen {
		maxlen = n
	}
	offset = rpos % size
	if n := size - offset; n < maxlen {
		maxlen = n
	}
	return
}

func woffset(blen int, size, rpos, wpos uint64) (maxlen, offset uint64) {
	maxlen = uint64(blen)
	if n := size + rpos - wpos; n < maxlen {
		maxlen = n
	}
	offset = wpos % size
	if n := size - offset; n < maxlen {
		maxlen = n
	}
	return
}

func (p *pipe) readFromFile(b []byte) (int, error) {
	if len(b) == 0 || p.file.f == nil {
		return 0, nil
	}
	maxlen, offset := roffset(len(b), p.file.size, p.file.rpos, p.file.wpos)
	if maxlen == 0 {
		return 0, nil
	}

	n, err := p.file.f.ReadAt(b[:maxlen], int64(offset))
	p.file.rpos += uint64(n)
	if p.file.rpos == p.file.wpos {
		p.file.rpos = 0
		p.file.wpos = 0
		if err == nil {
			err = p.file.f.Truncate(0)
		}
	}
	return n, errors.Trace(err)
}

func (p *pipe) writeToFile(b []byte) (int, error) {
	if len(b) == 0 || p.file.f == nil {
		return 0, nil
	}
	maxlen, offset := woffset(len(b), p.file.size, p.file.rpos, p.file.wpos)
	if maxlen == 0 {
		return 0, nil
	}

	n, err := p.file.f.WriteAt(b[:maxlen], int64(offset))
	p.file.wpos += uint64(n)
	return n, errors.Trace(err)
}

func (p *pipe) read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if p.buff.wpos == 0 {
		if len(b) > len(p.buff.p) {
			return p.readFromFile(b)
		} else {
			n, err := p.readFromFile(p.buff.p)
			p.buff.wpos += uint64(n)
			if err != nil || n == 0 {
				return 0, err
			}
		}
	}

	maxlen, offset := roffset(len(b), p.buff.size, p.buff.rpos, p.buff.wpos)
	if maxlen == 0 {
		return 0, nil
	}

	n := copy(b, p.buff.p[offset:offset+maxlen])
	p.buff.rpos += uint64(n)
	if p.buff.rpos == p.buff.wpos {
		p.buff.rpos = 0
		p.buff.wpos = 0
	}
	return n, nil
}

func (p *pipe) write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if p.file.wpos != 0 {
		return p.writeToFile(b)
	}

	maxlen, offset := woffset(len(b), p.buff.size, p.buff.rpos, p.buff.wpos)
	if maxlen == 0 {
		return p.writeToFile(b)
	}

	n := copy(p.buff.p[offset:offset+maxlen], b)
	p.buff.wpos += uint64(n)
	return n, nil
}

func (p *pipe) Read(b []byte) (int, error) {
	p.rl.Lock()
	defer p.rl.Unlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		if p.rerr != nil {
			return 0, errors.Trace(io.ErrClosedPipe)
		}
		n, err := p.read(b)
		if err != nil || n != 0 {
			p.wwait.Signal()
			return n, err
		}
		if p.werr != nil || len(b) == 0 {
			return 0, p.werr
		}
		p.rwait.Wait()
	}
}

func (p *pipe) Write(b []byte) (int, error) {
	p.wl.Lock()
	defer p.wl.Unlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		if p.werr != nil {
			return 0, errors.Trace(io.ErrClosedPipe)
		}
		if p.rerr != nil || len(b) == 0 {
			return 0, p.rerr
		}
		n, err := p.write(b)
		if err != nil || n != 0 {
			p.rwait.Signal()
			return n, err
		}
		p.wwait.Wait()
	}
}

func (p *pipe) rclose(err error) error {
	if err == nil {
		err = errors.Trace(io.ErrClosedPipe)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rerr = err
	p.rwait.Signal()
	p.wwait.Signal()
	if p.file.f != nil {
		return errors.Trace(p.file.f.Close())
	}
	return nil
}

func (p *pipe) wclose(err error) error {
	if err == nil {
		err = errors.Trace(io.EOF)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.werr = err
	p.rwait.Signal()
	p.wwait.Signal()
	return nil
}

const (
	BuffSizeAlign = bytesize.KB * 4
	FileSizeAlign = bytesize.MB * 4
)

func align(size, unit int) int {
	if size < unit {
		return unit
	}
	return (size + unit - 1) / unit * unit
}

func Pipe() (*PipeReader, *PipeWriter) {
	return PipeWithSize(BuffSizeAlign)
}

func PipeWithSize(buffSize int) (*PipeReader, *PipeWriter) {
	return PipeWithFile(0, buffSize, nil)
}

func PipeWithFile(buffSize, fileSize int, f *os.File) (*PipeReader, *PipeWriter) {
	buffSize = align(buffSize, BuffSizeAlign)
	fileSize = align(fileSize, FileSizeAlign)
	return newPipe(uint64(buffSize), uint64(fileSize), f)
}

func OpenFile(fileName string, exclusive bool) (*os.File, error) {
	flag := os.O_CREATE | os.O_RDWR | os.O_TRUNC
	if exclusive {
		flag |= os.O_EXCL
	}
	f, err := os.OpenFile(fileName, flag, 0600)
	return f, errors.Trace(err)
}

func OpenTempFile(dir, prefix string) (*os.File, error) {
	f, err := ioutil.TempFile(dir, prefix)
	return f, errors.Trace(err)
}
