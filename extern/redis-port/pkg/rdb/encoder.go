// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rdb

import (
	"bytes"
	"io"

	"github.com/cupcake/rdb"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

type objectEncoder interface {
	encodeType(enc *rdb.Encoder) error
	encodeValue(enc *rdb.Encoder) error
}

func (o String) encodeType(enc *rdb.Encoder) error {
	t := rdb.ValueType(rdbTypeString)
	return errors.Trace(enc.EncodeType(t))
}

func (o String) encodeValue(enc *rdb.Encoder) error {
	if err := enc.EncodeString([]byte(o)); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (o Hash) encodeType(enc *rdb.Encoder) error {
	t := rdb.ValueType(rdbTypeHash)
	return errors.Trace(enc.EncodeType(t))
}

func (o Hash) encodeValue(enc *rdb.Encoder) error {
	if err := enc.EncodeLength(uint32(len(o))); err != nil {
		return errors.Trace(err)
	}
	for _, e := range o {
		if err := enc.EncodeString(e.Field); err != nil {
			return errors.Trace(err)
		}
		if err := enc.EncodeString(e.Value); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (o List) encodeType(enc *rdb.Encoder) error {
	t := rdb.ValueType(rdbTypeList)
	return errors.Trace(enc.EncodeType(t))
}

func (o List) encodeValue(enc *rdb.Encoder) error {
	if err := enc.EncodeLength(uint32(len(o))); err != nil {
		return errors.Trace(err)
	}
	for _, e := range o {
		if err := enc.EncodeString(e); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (o ZSet) encodeType(enc *rdb.Encoder) error {
	t := rdb.ValueType(rdbTypeZSet)
	return errors.Trace(enc.EncodeType(t))
}

func (o ZSet) encodeValue(enc *rdb.Encoder) error {
	if err := enc.EncodeLength(uint32(len(o))); err != nil {
		return errors.Trace(err)
	}
	for _, e := range o {
		if err := enc.EncodeString(e.Member); err != nil {
			return errors.Trace(err)
		}
		if err := enc.EncodeFloat(e.Score); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (o Set) encodeType(enc *rdb.Encoder) error {
	t := rdb.ValueType(rdbTypeSet)
	return errors.Trace(enc.EncodeType(t))
}

func (o Set) encodeValue(enc *rdb.Encoder) error {
	if err := enc.EncodeLength(uint32(len(o))); err != nil {
		return errors.Trace(err)
	}
	for _, e := range o {
		if err := enc.EncodeString(e); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func EncodeDump(obj interface{}) ([]byte, error) {
	o, ok := obj.(objectEncoder)
	if !ok {
		return nil, errors.New("unsupported object type")
	}
	var b bytes.Buffer
	enc := rdb.NewEncoder(&b)
	if err := o.encodeType(enc); err != nil {
		return nil, err
	}
	if err := o.encodeValue(enc); err != nil {
		return nil, err
	}
	if err := enc.EncodeDumpFooter(); err != nil {
		return nil, errors.Trace(err)
	}
	return b.Bytes(), nil
}

type Encoder struct {
	enc *rdb.Encoder
	db  int64
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		enc: rdb.NewEncoder(w),
		db:  -1,
	}
}

func (e *Encoder) EncodeHeader() error {
	return errors.Trace(e.enc.EncodeHeader())
}

func (e *Encoder) EncodeFooter() error {
	return errors.Trace(e.enc.EncodeFooter())
}

func (e *Encoder) EncodeObject(db uint32, key []byte, expireat uint64, obj interface{}) error {
	o, ok := obj.(objectEncoder)
	if !ok {
		return errors.New("unsupported object type")
	}
	if e.db == -1 || uint32(e.db) != db {
		e.db = int64(db)
		if err := e.enc.EncodeDatabase(int(db)); err != nil {
			return errors.Trace(err)
		}
	}
	if expireat != 0 {
		if err := e.enc.EncodeExpiry(expireat); err != nil {
			return errors.Trace(err)
		}
	}
	if err := o.encodeType(e.enc); err != nil {
		return err
	}
	if err := e.enc.EncodeString(key); err != nil {
		return errors.Trace(err)
	}
	if err := o.encodeValue(e.enc); err != nil {
		return err
	}
	return nil
}
