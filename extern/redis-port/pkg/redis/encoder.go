// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bytes"
	"io"
	"strconv"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/utils"
)

type encoder struct {
	w io.Writer
}

var (
	crlf []byte
	inil []byte
	imap [][]byte
)

func init() {
	crlf = []byte("\r\n")
	inil = []byte(strconv.Itoa(-1))
	imap = make([][]byte, 1024*512)
	for i := 0; i < len(imap); i++ {
		imap[i] = []byte(strconv.Itoa(i))
	}
}

func itobytes(i int64) []byte {
	if i == -1 {
		return inil
	} else if i >= 0 && i < int64(len(imap)) {
		return imap[i]
	} else {
		return []byte(strconv.FormatInt(i, 10))
	}
}

func Encode(w io.Writer, r Resp) error {
	e := &encoder{w}
	return e.encodeResp(r)
}

func MustEncode(w io.Writer, r Resp) {
	if err := Encode(w, r); err != nil {
		utils.ErrorPanic(err, "encode redis resp failed")
	}
}

func EncodeToBytes(r Resp) ([]byte, error) {
	var b bytes.Buffer
	err := Encode(&b, r)
	return b.Bytes(), err
}

func EncodeToString(r Resp) (string, error) {
	var b bytes.Buffer
	err := Encode(&b, r)
	return b.String(), err
}

func MustEncodeToBytes(r Resp) []byte {
	b, err := EncodeToBytes(r)
	if err != nil {
		utils.ErrorPanic(err, "encode redis resp to bytes failed")
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
		return e.encodeString([]byte(x.Value))
	case *Error:
		if err := e.encodeType(TypeError); err != nil {
			return err
		}
		return e.encodeString([]byte(x.Value))
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
	_, err := e.w.Write([]byte{byte(t)})
	return errors.Trace(err)
}

func (e *encoder) encodeString(b []byte) error {
	if _, err := ioutils.WriteFull(e.w, b); err != nil {
		return err
	}
	if _, err := ioutils.WriteFull(e.w, crlf); err != nil {
		return err
	}
	return nil
}

func (e *encoder) encodeInt(v int64) error {
	return e.encodeString(itobytes(v))
}

func (e *encoder) encodeBulkBytes(b []byte) error {
	if b == nil {
		return e.encodeInt(-1)
	} else {
		if err := e.encodeInt(int64(len(b))); err != nil {
			return err
		}
		if err := e.encodeString(b); err != nil {
			return err
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
