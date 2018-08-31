// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"fmt"
	"sync"
)

type RespType byte

const (
	TypeString    RespType = '+'
	TypeError     RespType = '-'
	TypeInt       RespType = ':'
	TypeBulkBytes RespType = '$'
	TypeArray     RespType = '*'
)

func (t RespType) String() string {
	switch t {
	case TypeString:
		return "<string>"
	case TypeError:
		return "<error>"
	case TypeInt:
		return "<int>"
	case TypeBulkBytes:
		return "<bulkbytes>"
	case TypeArray:
		return "<array>"
	default:
		return fmt.Sprintf("<unknown-0x%02x>", byte(t))
	}
}

type Resp struct {
	Type RespType

	Value []byte
	Array []*Resp
}

var respPool = &sync.Pool{
	New: func() interface{} {
		return &Resp{Array: make([]*Resp, 0, 8)}
	},
}

func AcquireResp() *Resp {
	return respPool.Get().(*Resp)
}

func ReleaseResp(r *Resp) {
	r.Type = 0
	r.Value = nil
	if r.Array == nil {
		r.Array = make([]*Resp, 0, 8)
	} else {
		for i := 0; i < len(r.Array); i++ {
			ReleaseResp(r.Array[i])
			r.Array[i] = nil
		}
		r.Array = r.Array[:0]
	}
	respPool.Put(r)
}

func (r *Resp) IsString() bool {
	return r.Type == TypeString
}

func (r *Resp) IsError() bool {
	return r.Type == TypeError
}

func (r *Resp) IsInt() bool {
	return r.Type == TypeInt
}

func (r *Resp) IsBulkBytes() bool {
	return r.Type == TypeBulkBytes
}

func (r *Resp) IsArray() bool {
	return r.Type == TypeArray
}

func NewString(value []byte) *Resp {
	r := &Resp{}
	r.Type = TypeString
	r.Value = value
	return r
}

func NewError(value []byte) *Resp {
	r := &Resp{}
	r.Type = TypeError
	r.Value = value
	return r
}

func NewErrorf(format string, args ...interface{}) *Resp {
	return NewError([]byte(fmt.Sprintf(format, args...)))
}

func NewInt(value []byte) *Resp {
	r := &Resp{}
	r.Type = TypeInt
	r.Value = value
	return r
}

func NewBulkBytes(value []byte) *Resp {
	r := &Resp{}
	r.Type = TypeBulkBytes
	r.Value = value
	return r
}

func NewArray(array []*Resp) *Resp {
	r := &Resp{}
	r.Type = TypeArray
	r.Array = array
	return r
}
