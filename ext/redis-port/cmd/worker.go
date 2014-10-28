package cmd

import (
	"bufio"
	"io"
	"sync"
	"time"
)

import (
	"github.com/wandoulabs/codis/ext/redis-port/rdb"
	"github.com/wandoulabs/codis/ext/redis-port/utils"
)

type ClockTicker struct {
	sig chan int
}

func NewClockTicker(wg *sync.WaitGroup, onTick func(), onClose func()) *ClockTicker {
	wg.Add(1)
	sig := make(chan int)
	go func() {
		defer wg.Done()
		defer onClose()
		for {
			select {
			case <-sig:
				return
			case <-time.After(time.Second):
				onTick()
			}
		}
	}()
	return &ClockTicker{sig}
}

func (t *ClockTicker) Close() {
	close(t.sig)
}

type RdbLoader struct {
	pipe chan *rdb.Entry
}

func NewRdbLoader(wg *sync.WaitGroup, size int, reader *bufio.Reader, nread *AtomicInt64) *RdbLoader {
	wg.Add(1)
	pipe := make(chan *rdb.Entry, size)
	go func() {
		defer close(pipe)
		defer wg.Done()
		l := rdb.NewLoader(reader)
		if err := l.LoadHeader(); err != nil {
			utils.Panic("parse rdb header error = '%s'", err)
		}
		for {
			if entry, offset, err := l.LoadEntry(); err != nil {
				utils.Panic("parse rdb entry error = '%s'", err)
			} else {
				if entry != nil {
					nread.Set(offset)
					pipe <- entry
				} else {
					if err := l.LoadChecksum(); err != nil {
						utils.Panic("parse rdb checksum error = '%s'", err)
					}
					return
				}
			}
		}
	}()
	return &RdbLoader{pipe}
}

func (l *RdbLoader) Pipe() chan *rdb.Entry {
	return l.pipe
}

type BufWriter struct {
	pipe chan string
}

func NewBufWriter(wg *sync.WaitGroup, size int, writer *bufio.Writer, nwrite *AtomicInt64) *BufWriter {
	wg.Add(1)
	pipe := make(chan string, size)
	go func() {
		defer wg.Done()
		for s := range pipe {
			if _, err := writer.WriteString(s); err != nil {
				utils.Panic("write error = '%s'", err)
			}
			if err := writer.Flush(); err != nil {
				utils.Panic("flush error = '%s'", err)
			}
			nwrite.Add(int64(len(s)))
		}
	}()
	return &BufWriter{pipe}
}

func (w *BufWriter) Append(s string) {
	w.pipe <- s
}

func (w *BufWriter) Close() {
	close(w.pipe)
}

func PipeReaderWriter(wg *sync.WaitGroup, r io.Reader, w io.Writer, nread, nwrite *AtomicInt64, total int64) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for total != 0 {
			p := make([]byte, 1024)
			if total > 0 && int64(len(p)) > total {
				p = p[:total]
			}
			if n, err := r.Read(p); err != nil {
				utils.Panic("read full error = '%s'", err)
			} else {
				p = p[:n]
			}
			delta := int64(len(p))
			nread.Add(delta)
			for len(p) != 0 {
				n, err := w.Write(p)
				if err != nil {
					utils.Panic("write error = '%s'", err)
				}
				p = p[n:]
			}
			nwrite.Add(delta)
			if total > 0 {
				total -= delta
			}
		}
	}()
}
