// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/bytesize"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
)

var (
	ErrVarbytesLen = errors.Static("invalid varbytes length")
)

const (
	maxVarbytesLen = bytesize.MB * 512
)

type BufReader struct {
	r *bytes.Reader
}

func NewBufReader(p []byte) *BufReader {
	return &BufReader{bytes.NewReader(p)}
}

func (r *BufReader) ReadByte() (byte, error) {
	b, err := r.r.ReadByte()
	return b, errors.Trace(err)
}

func (r *BufReader) ReadBytes(n int) ([]byte, error) {
	p := make([]byte, n)
	_, err := ioutils.ReadFull(r.r, p)
	return p, err
}

func (r *BufReader) ReadVarint() (int64, error) {
	v, err := binary.ReadVarint(r)
	return v, errors.Trace(err)
}

func (r *BufReader) ReadUvarint() (uint64, error) {
	u, err := binary.ReadUvarint(r)
	return u, errors.Trace(err)
}

func (r *BufReader) ReadVarbytes() ([]byte, error) {
	n, err := r.ReadUvarint()
	if err != nil {
		return nil, err
	}
	if n < 0 || n > maxVarbytesLen {
		return nil, errors.Trace(ErrVarbytesLen)
	} else if n == 0 {
		return nil, nil
	}
	return r.ReadBytes(int(n))
}

func (r *BufReader) ReadFloat64() (float64, error) {
	p, err := r.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint64(p)
	return math.Float64frombits(bits), nil
}

func (r *BufReader) Len() int {
	return r.r.Len()
}

type BufWriter struct {
	w *bytes.Buffer
}

func NewBufWriter(p []byte) *BufWriter {
	if p != nil {
		dup := make([]byte, len(p), cap(p))
		copy(dup, p)
		p = dup
	}
	return &BufWriter{bytes.NewBuffer(p)}
}

func (w *BufWriter) WriteByte(b byte) error {
	return errors.Trace(w.w.WriteByte(b))
}

func (w *BufWriter) WriteBytes(p []byte) error {
	_, err := ioutils.WriteFull(w.w, p)
	return err
}

func (w *BufWriter) WriteVarint(v int64) error {
	p := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(p, v)
	_, err := ioutils.WriteFull(w.w, p[:n])
	return err
}

func (w *BufWriter) WriteUvarint(v uint64) error {
	p := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(p, v)
	_, err := ioutils.WriteFull(w.w, p[:n])
	return err
}

func (w *BufWriter) WriteVarbytes(p []byte) error {
	if n := uint64(len(p)); n > maxVarbytesLen {
		return errors.Trace(ErrVarbytesLen)
	} else if err := w.WriteUvarint(n); err != nil {
		return err
	}
	_, err := ioutils.WriteFull(w.w, p)
	return err
}

func (w *BufWriter) WriteFloat64(f float64) error {
	p := make([]byte, 8)
	bits := math.Float64bits(f)
	binary.LittleEndian.PutUint64(p, bits)
	_, err := ioutils.WriteFull(w.w, p)
	return err
}

func (w *BufWriter) Len() int {
	return w.w.Len()
}

func (w *BufWriter) Bytes() []byte {
	return w.w.Bytes()
}
