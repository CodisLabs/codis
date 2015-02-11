// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bufio"
	"bytes"
	"strconv"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
)

type encoder struct {
	w *bufio.Writer
}

var (
	imap []string
)

func init() {
	imap = make([]string, 1024*512+1024)
	for i := 0; i < len(imap); i++ {
		imap[i] = strconv.Itoa(i - 1024)
	}
}

func itos(i int64) string {
	if n := i + 1024; n >= 0 && n < int64(len(imap)) {
		return imap[n]
	} else {
		return strconv.FormatInt(i, 10)
	}
}

func Encode(w *bufio.Writer, r Resp) error {
	e := &encoder{w}
	if err := e.encodeResp(r); err != nil {
		return err
	}
	return errors.Trace(w.Flush())
}

func MustEncode(w *bufio.Writer, r Resp) {
	if err := Encode(w, r); err != nil {
		log.PanicError(err, "encode redis resp failed")
	}
}

func EncodeToBytes(r Resp) ([]byte, error) {
	var b bytes.Buffer
	err := Encode(bufio.NewWriter(&b), r)
	return b.Bytes(), err
}

func EncodeToString(r Resp) (string, error) {
	var b bytes.Buffer
	err := Encode(bufio.NewWriter(&b), r)
	return b.String(), err
}

func MustEncodeToBytes(r Resp) []byte {
	b, err := EncodeToBytes(r)
	if err != nil {
		log.PanicError(err, "encode redis resp to bytes failed")
	}
	return b
}

func (e *encoder) encodeResp(r Resp) error {
	switch x := r.(type) {
	default:
		return errors.Trace(ErrBadRespType)
	case *String:
		if err := e.encodeType(TypeString); err != nil {
			return err
		}
		return e.encodeText(x.Value)
	case *Error:
		if err := e.encodeType(TypeError); err != nil {
			return err
		}
		return e.encodeText(x.Value)
	case *Int:
		if err := e.encodeType(TypeInt); err != nil {
			return err
		}
		return e.encodeInt(x.Value)
	case *BulkBytes:
		if err := e.encodeType(TypeBulkBytes); err != nil {
			return err
		}
		return e.encodeBulkBytes(x.Value)
	case *Array:
		if err := e.encodeType(TypeArray); err != nil {
			return err
		}
		return e.encodeArray(x.Value)
	}
}

func (e *encoder) encodeType(t RespType) error {
	return errors.Trace(e.w.WriteByte(byte(t)))
}

func (e *encoder) encodeText(s string) error {
	if _, err := e.w.WriteString(s); err != nil {
		return errors.Trace(err)
	}
	if _, err := e.w.WriteString("\r\n"); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (e *encoder) encodeInt(v int64) error {
	return e.encodeText(itos(v))
}

func (e *encoder) encodeBulkBytes(b []byte) error {
	if b == nil {
		return e.encodeInt(-1)
	} else {
		if err := e.encodeInt(int64(len(b))); err != nil {
			return err
		}
		if _, err := e.w.Write(b); err != nil {
			return errors.Trace(err)
		}
		if _, err := e.w.WriteString("\r\n"); err != nil {
			return errors.Trace(err)
		}
		return nil
	}
}

func (e *encoder) encodeArray(a []Resp) error {
	if a == nil {
		return e.encodeInt(-1)
	} else {
		if err := e.encodeInt(int64(len(a))); err != nil {
			return err
		}
		for i := 0; i < len(a); i++ {
			if err := e.encodeResp(a[i]); err != nil {
				return err
			}
		}
		return nil
	}
}
