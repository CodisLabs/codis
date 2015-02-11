// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package pipe

import (
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/bytesize"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

type buffer interface {
	read(b []byte) (int, error)
	write(b []byte) (int, error)
	rclose() error
	wclose() error

	buffered() int
	available() int
}

type pipe struct {
	rl sync.Mutex
	wl sync.Mutex
	mu sync.Mutex

	rwait *sync.Cond
	wwait *sync.Cond

	rerr error
	werr error

	store buffer
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

func newPipe(store buffer) (Reader, Writer) {
	p := &pipe{}
	p.rwait = sync.NewCond(&p.mu)
	p.wwait = sync.NewCond(&p.mu)
	p.store = store
	r := &reader{p}
	w := &writer{p}
	return r, w
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
		n, err := p.store.read(b)
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
		n, err := p.store.write(b)
		if err != nil || n != 0 {
			p.rwait.Signal()
			return n, err
		}
		p.wwait.Wait()
	}
}

func (p *pipe) RClose(err error) error {
	if err == nil {
		err = errors.Trace(io.ErrClosedPipe)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rerr == nil {
		p.rerr = err
	}
	p.rwait.Signal()
	p.wwait.Signal()
	return p.store.rclose()
}

func (p *pipe) WClose(err error) error {
	if err == nil {
		err = errors.Trace(io.EOF)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.werr == nil {
		p.werr = err
	}
	p.rwait.Signal()
	p.wwait.Signal()
	return p.store.wclose()
}

func (p *pipe) Buffered() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rerr != nil {
		return 0, p.rerr
	}
	n := p.store.buffered()
	if p.werr != nil && n == 0 {
		return 0, p.werr
	}
	return n, nil
}

func (p *pipe) Available() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.werr != nil {
		return 0, p.werr
	}
	if p.rerr != nil {
		return 0, p.rerr
	}
	n := p.store.available()
	return n, nil
}

func Pipe() (Reader, Writer) {
	return newPipe(newMemBuffer(0))
}

func PipeSize(buffSize int) (Reader, Writer) {
	return newPipe(newMemBuffer(buffSize))
}

func PipeFile(buffSize, fileSize int, f *os.File) (Reader, Writer) {
	if f == nil {
		return newPipe(newMemBuffer(buffSize))
	} else {
		return newPipe(newRFileBuffer(buffSize, fileSize, f))
	}
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
