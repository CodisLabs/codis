// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import "fmt"

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
	return &Resp{
		Type:  TypeString,
		Value: value,
	}
}

func NewError(value []byte) *Resp {
	return &Resp{
		Type:  TypeError,
		Value: value,
	}
}

func NewInt(value []byte) *Resp {
	return &Resp{
		Type:  TypeInt,
		Value: value,
	}
}

func NewBulkBytes(value []byte) *Resp {
	return &Resp{
		Type:  TypeBulkBytes,
		Value: value,
	}
}

func NewArray(array []*Resp) *Resp {
	return &Resp{
		Type:  TypeArray,
		Array: array,
	}
}

func (r *Resp) Append(x *Resp) {
	if r.Type == TypeArray {
		r.Array = append(r.Array, x)
	}
}
