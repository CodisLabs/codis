// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bufio"
	"bytes"
	"io"
	"strconv"

	"github.com/CodisLabs/codis/pkg/utils/errors"
)

var (
	itoamap []string
	itobmap [][]byte
)

func init() {
	itoamap = make([]string, 1024*128+1024)
	itobmap = make([][]byte, len(itoamap))
	for i := 0; i < len(itoamap); i++ {
		itoamap[i] = strconv.Itoa(i - 1024)
		itobmap[i] = []byte(itoamap[i])
	}
}

func itoxIndex(i int64) int {
	n := i + 1024
	if i < n {
		if n >= 0 && n < int64(len(itoamap)) {
			return int(n)
		}
	}
	return -1
}

func itoa(i int64) string {
	if n := itoxIndex(i); n >= 0 {
		return itoamap[n]
	}
	return strconv.FormatInt(i, 10)
}

func itob(i int64) []byte {
	if n := itoxIndex(i); n >= 0 {
		return itobmap[n]
	}
	return []byte(strconv.FormatInt(i, 10))
}

type Encoder struct {
	*bufio.Writer

	Err error
}

func NewEncoder(bw *bufio.Writer) *Encoder {
	return &Encoder{Writer: bw}
}

func NewEncoderSize(w io.Writer, size int) *Encoder {
	bw, ok := w.(*bufio.Writer)
	if !ok {
		bw = bufio.NewWriterSize(w, size)
	}
	return &Encoder{Writer: bw}
}

func (e *Encoder) Encode(r *Resp, flush bool) error {
	if e.Err != nil {
		return e.Err
	}
	err := e.encodeResp(r)
	if err == nil && flush {
		err = errors.Trace(e.Flush())
	}
	if err != nil {
		e.Err = err
	}
	return err
}

func Encode(bw *bufio.Writer, r *Resp, flush bool) error {
	return NewEncoder(bw).Encode(r, flush)
}

func EncodeToBytes(r *Resp) ([]byte, error) {
	var b = &bytes.Buffer{}
	err := Encode(bufio.NewWriter(b), r, true)
	return b.Bytes(), err
}

func (e *Encoder) encodeResp(r *Resp) error {
	if err := e.WriteByte(byte(r.Type)); err != nil {
		return errors.Trace(err)
	}
	switch r.Type {
	default:
		return errors.Errorf("bad resp type %s", r.Type)
	case TypeString, TypeError, TypeInt:
		return e.encodeTextBytes(r.Value)
	case TypeBulkBytes:
		return e.encodeBulkBytes(r.Value)
	case TypeArray:
		return e.encodeArray(r.Array)
	}
}

func (e *Encoder) encodeTextBytes(b []byte) error {
	if _, err := e.Write(b); err != nil {
		return errors.Trace(err)
	}
	if _, err := e.WriteString("\r\n"); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (e *Encoder) encodeTextString(s string) error {
	if _, err := e.WriteString(s); err != nil {
		return errors.Trace(err)
	}
	if _, err := e.WriteString("\r\n"); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (e *Encoder) encodeInt(v int64) error {
	return e.encodeTextString(itoa(v))
}

func (e *Encoder) encodeBulkBytes(b []byte) error {
	if b == nil {
		return e.encodeInt(-1)
	} else {
		if err := e.encodeInt(int64(len(b))); err != nil {
			return err
		}
		return e.encodeTextBytes(b)
	}
}

func (e *Encoder) encodeArray(a []*Resp) error {
	if a == nil {
		return e.encodeInt(-1)
	} else {
		if err := e.encodeInt(int64(len(a))); err != nil {
			return err
		}
		for _, r := range a {
			if err := e.encodeResp(r); err != nil {
				return err
			}
		}
		return nil
	}
}
