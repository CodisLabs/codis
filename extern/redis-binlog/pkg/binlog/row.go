// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"fmt"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
)

var (
	ErrMetaKey = errors.Static("invalid meta key")
	ErrDataKey = errors.Static("invalid data key")

	ErrNotMatched = errors.Static("unmatched raw bytes")

	ErrObjectCode  = errors.Static("invalid object code")
	ErrObjectValue = errors.Static("invalid object value")

	ErrNotString = errors.Static("not string")
	ErrNotHash   = errors.Static("not hash")
	ErrNotList   = errors.Static("not list")
	ErrNotZSet   = errors.Static("not zset")
	ErrNotSet    = errors.Static("not set")
)

func EncodeMetaKey(db uint32, key []byte) []byte {
	if len(key) == 0 {
		log.Errorf("encode nil meta key")
	}
	tag, slot := HashKeyToSlot(key)
	if len(tag) == len(key) {
		key = nil
	}
	w := NewBufWriter(nil)
	encodeRawBytes(w, MetaCode, &db, &slot, &tag, &key)
	return w.Bytes()
}

func DecodeMetaKey(p []byte) (db uint32, key []byte, err error) {
	var tag []byte
	var slot uint32
	r := NewBufReader(p)
	err = decodeRawBytes(r, err, MetaCode, &db, &slot, &tag, &key)
	err = decodeRawBytes(r, err)
	if err != nil {
		return
	}
	if len(key) == 0 {
		key = tag
	}
	if len(key) == 0 {
		log.Errorf("decode nil meta key")
	}
	return
}

func EncodeMetaKeyPrefixSlot(db uint32, slot uint32) []byte {
	w := NewBufWriter(nil)
	encodeRawBytes(w, MetaCode, &db, &slot)
	return w.Bytes()
}

func EncodeMetaKeyPrefixTag(db uint32, tag []byte) []byte {
	slot := HashTagToSlot(tag)
	w := NewBufWriter(nil)
	encodeRawBytes(w, MetaCode, &db, &slot, &tag)
	return w.Bytes()
}

func EncodeDataKeyPrefix(db uint32, key []byte) []byte {
	if len(key) == 0 {
		log.Errorf("encode nil data key")
	}
	w := NewBufWriter(nil)
	encodeRawBytes(w, DataCode, &db, &key)
	return w.Bytes()
}

type binlogRow interface {
	Code() ObjectCode

	MetaKey() []byte
	MetaValue() []byte
	ParseMetaValue(p []byte) error

	DataKey() []byte
	DataValue() []byte
	ParseDataValue(p []byte) error

	LoadDataValue(r binlogReader) (bool, error)
	TestDataValue(r binlogReader) (bool, error)

	GetExpireAt() uint64
	SetExpireAt(expireat uint64)
	IsExpired() bool

	lazyInit(h *binlogRowHelper)
	storeObject(b *Binlog, bt *store.Batch, expireat uint64, obj interface{}) error
	deleteObject(b *Binlog, bt *store.Batch) error
	loadObjectValue(r binlogReader) (interface{}, error)
}

type binlogRowHelper struct {
	code          ObjectCode
	metaKey       []byte
	dataKeyPrefix []byte

	ExpireAt uint64

	dataKeyRefs   []interface{}
	metaValueRefs []interface{}
	dataValueRefs []interface{}
}

func loadBinlogRow(r binlogReader, db uint32, key []byte) (binlogRow, error) {
	metaKey := EncodeMetaKey(db, key)
	p, err := r.getRowValue(metaKey)
	if err != nil || p == nil {
		return nil, err
	}
	if len(p) == 0 {
		return nil, errors.Trace(ErrObjectCode)
	}
	var o binlogRow
	var code = ObjectCode(p[0])
	switch code {
	default:
		return nil, errors.Trace(ErrObjectCode)
	case StringCode:
		o = new(stringRow)
	case HashCode:
		o = new(hashRow)
	case ListCode:
		o = new(listRow)
	case ZSetCode:
		o = new(zsetRow)
	case SetCode:
		o = new(setRow)
	}
	o.lazyInit(&binlogRowHelper{
		code:          code,
		metaKey:       metaKey,
		dataKeyPrefix: EncodeDataKeyPrefix(db, key),
	})
	return o, o.ParseMetaValue(p)
}

func newBinlogRowHelper(db uint32, key []byte, code ObjectCode) *binlogRowHelper {
	return &binlogRowHelper{
		code:          code,
		metaKey:       EncodeMetaKey(db, key),
		dataKeyPrefix: EncodeDataKeyPrefix(db, key),
	}
}

func (o *binlogRowHelper) Code() ObjectCode {
	return o.code
}

func (o *binlogRowHelper) MetaKey() []byte {
	return o.metaKey
}

func (o *binlogRowHelper) MetaValue() []byte {
	w := NewBufWriter(nil)
	encodeRawBytes(w, o.code, &o.ExpireAt)
	encodeRawBytes(w, o.metaValueRefs...)
	return w.Bytes()
}

func (o *binlogRowHelper) ParseMetaValue(p []byte) (err error) {
	r := NewBufReader(p)
	err = decodeRawBytes(r, err, o.code, &o.ExpireAt)
	err = decodeRawBytes(r, err, o.metaValueRefs...)
	err = decodeRawBytes(r, err)
	return
}

func (o *binlogRowHelper) DataKey() []byte {
	if len(o.dataKeyRefs) != 0 {
		w := NewBufWriter(o.DataKeyPrefix())
		encodeRawBytes(w, o.dataKeyRefs...)
		return w.Bytes()
	} else {
		return o.DataKeyPrefix()
	}
}

func (o *binlogRowHelper) DataKeyPrefix() []byte {
	return o.dataKeyPrefix
}

func (o *binlogRowHelper) ParseDataKeySuffix(p []byte) (err error) {
	r := NewBufReader(p)
	err = decodeRawBytes(r, err, o.dataKeyRefs...)
	err = decodeRawBytes(r, err)
	return
}

func (o *binlogRowHelper) DataValue() []byte {
	w := NewBufWriter(nil)
	encodeRawBytes(w, o.code)
	encodeRawBytes(w, o.dataValueRefs...)
	return w.Bytes()
}

func (o *binlogRowHelper) ParseDataValue(p []byte) (err error) {
	r := NewBufReader(p)
	err = decodeRawBytes(r, err, o.code)
	err = decodeRawBytes(r, err, o.dataValueRefs...)
	err = decodeRawBytes(r, err)
	return
}

func (o *binlogRowHelper) LoadDataValue(r binlogReader) (bool, error) {
	p, err := r.getRowValue(o.DataKey())
	if err != nil || p == nil {
		return false, err
	}
	return true, o.ParseDataValue(p)
}

func (o *binlogRowHelper) TestDataValue(r binlogReader) (bool, error) {
	p, err := r.getRowValue(o.DataKey())
	if err != nil || p == nil {
		return false, err
	}
	return true, nil
}

func (o *binlogRowHelper) GetExpireAt() uint64 {
	return o.ExpireAt
}

func (o *binlogRowHelper) SetExpireAt(expireat uint64) {
	o.ExpireAt = expireat
}

func (o *binlogRowHelper) IsExpired() bool {
	return IsExpired(o.ExpireAt)
}

func IsExpired(expireat uint64) bool {
	return expireat != 0 && expireat <= nowms()
}

const (
	MetaCode = byte('#')
	DataCode = byte('&')
)

type ObjectCode byte

const (
	StringCode ObjectCode = 'K'
	HashCode   ObjectCode = 'H'
	ListCode   ObjectCode = 'L'
	ZSetCode   ObjectCode = 'Z'
	SetCode    ObjectCode = 'S'
)

func (c ObjectCode) String() string {
	switch c {
	case StringCode:
		return "string"
	case HashCode:
		return "hash"
	case ListCode:
		return "list"
	case ZSetCode:
		return "zset"
	case SetCode:
		return "set"
	case 0:
		return "none"
	default:
		return fmt.Sprintf("unknown %02x", byte(c))
	}
}

func encodeRawBytes(w *BufWriter, refs ...interface{}) {
	for _, i := range refs {
		var err error
		switch x := i.(type) {
		case byte:
			err = w.WriteByte(x)
		case ObjectCode:
			err = w.WriteByte(byte(x))
		case *uint32:
			err = w.WriteUvarint(uint64(*x))
		case *uint64:
			err = w.WriteUvarint(*x)
		case *int64:
			err = w.WriteVarint(*x)
		case *float64:
			err = w.WriteFloat64(*x)
		case *[]byte:
			err = w.WriteVarbytes(*x)
		default:
			log.Panicf("unsupported type in row value: %+v", x)
		}
		if err != nil {
			log.PanicErrorf(err, "encode raw bytes failed")
		}
	}
}

func decodeRawBytes(r *BufReader, err error, refs ...interface{}) error {
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		if r.Len() != 0 {
			return errors.Trace(ErrNotMatched)
		}
		return nil
	}
	for _, i := range refs {
		switch x := i.(type) {
		case byte:
			if v, err := r.ReadByte(); err != nil {
				return err
			} else if v != x {
				return errors.Errorf("read byte %d, expect %d", v, x)
			}
		case ObjectCode:
			if v, err := r.ReadByte(); err != nil {
				return err
			} else if v != byte(x) {
				return errors.Errorf("read code [%s], expect [%s]", ObjectCode(v), x)
			}
		case *[]byte:
			p, err := r.ReadVarbytes()
			if err != nil {
				return err
			}
			*x = p
		case *uint32:
			v, err := r.ReadUvarint()
			if err != nil {
				return err
			}
			*x = uint32(v)
		case *uint64:
			v, err := r.ReadUvarint()
			if err != nil {
				return err
			}
			*x = v
		case *int64:
			v, err := r.ReadVarint()
			if err != nil {
				return err
			}
			*x = v
		case *float64:
			v, err := r.ReadFloat64()
			if err != nil {
				return err
			}
			*x = v
		default:
			log.Panicf("unsupported type in row value: %+v", x)
		}
	}
	return nil
}
