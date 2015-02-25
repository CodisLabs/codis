// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

type stringRow struct {
	*binlogRowHelper

	Value []byte
}

func newStringRow(db uint32, key []byte) *stringRow {
	o := &stringRow{}
	o.lazyInit(newBinlogRowHelper(db, key, StringCode))
	return o
}

func (o *stringRow) lazyInit(h *binlogRowHelper) {
	o.binlogRowHelper = h
	o.dataKeyRefs = nil
	o.metaValueRefs = nil
	o.dataValueRefs = []interface{}{&o.Value}
}

func (o *stringRow) deleteObject(b *Binlog, bt *store.Batch) error {
	bt.Del(o.DataKey())
	bt.Del(o.MetaKey())
	return nil
}

func (o *stringRow) storeObject(b *Binlog, bt *store.Batch, expireat uint64, obj interface{}) error {
	value, ok := obj.(rdb.String)
	if !ok || len(value) == 0 {
		return errors.Trace(ErrObjectValue)
	}

	o.ExpireAt, o.Value = expireat, value
	bt.Set(o.DataKey(), o.DataValue())
	bt.Set(o.MetaKey(), o.MetaValue())
	return nil
}

func (o *stringRow) loadObjectValue(r binlogReader) (interface{}, error) {
	_, err := o.LoadDataValue(r)
	if err != nil {
		return nil, err
	}
	return rdb.String(o.Value), nil
}

func (b *Binlog) loadStringRow(db uint32, key []byte, deleteIfExpired bool) (*stringRow, error) {
	o, err := b.loadBinlogRow(db, key, deleteIfExpired)
	if err != nil {
		return nil, err
	} else if o != nil {
		x, ok := o.(*stringRow)
		if ok {
			return x, nil
		}
		return nil, errors.Trace(ErrNotString)
	}
	return nil, nil
}

// GET key
func (b *Binlog) Get(db uint32, args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, errArguments("len(args) = %d, expect = 1", len(args))
	}

	var key []byte
	for i, ref := range []interface{}{&key} {
		if err := parseArgument(args[i], ref); err != nil {
			return nil, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return nil, err
	}
	defer b.release()

	o, err := b.loadStringRow(db, key, true)
	if err != nil || o == nil {
		return nil, err
	} else {
		_, err := o.LoadDataValue(b)
		if err != nil {
			return nil, err
		}
		return o.Value, nil
	}
}

// APPEND key value
func (b *Binlog) Append(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 2 {
		return 0, errArguments("len(args) = %d, expect = 2", len(args))
	}

	var key, value []byte
	for i, ref := range []interface{}{&key, &value} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	o, err := b.loadStringRow(db, key, true)
	if err != nil {
		return 0, err
	}

	bt := store.NewBatch()
	if o != nil {
		_, err := o.LoadDataValue(b)
		if err != nil {
			return 0, err
		}
		o.Value = append(o.Value, value...)
	} else {
		o = newStringRow(db, key)
		o.Value = value
		bt.Set(o.MetaKey(), o.MetaValue())
	}
	bt.Set(o.DataKey(), o.DataValue())
	fw := &Forward{DB: db, Op: "Append", Args: args}
	return int64(len(o.Value)), b.commit(bt, fw)
}

// SET key value
func (b *Binlog) Set(db uint32, args ...interface{}) error {
	if len(args) != 2 {
		return errArguments("len(args) = %d, expect = 2", len(args))
	}

	var key, value []byte
	for i, ref := range []interface{}{&key, &value} {
		if err := parseArgument(args[i], ref); err != nil {
			return errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return err
	}
	defer b.release()

	bt := store.NewBatch()
	_, err := b.deleteIfExists(bt, db, key)
	if err != nil {
		return err
	}
	o := newStringRow(db, key)
	o.Value = value
	bt.Set(o.DataKey(), o.DataValue())
	bt.Set(o.MetaKey(), o.MetaValue())
	fw := &Forward{DB: db, Op: "Set", Args: args}
	return b.commit(bt, fw)
}

// PSETEX key milliseconds value
func (b *Binlog) PSetEX(db uint32, args ...interface{}) error {
	if len(args) != 3 {
		return errArguments("len(args) = %d, expect = 3", len(args))
	}

	var key, value []byte
	var ttlms int64
	for i, ref := range []interface{}{&key, &ttlms, &value} {
		if err := parseArgument(args[i], ref); err != nil {
			return errArguments("parse args[%d] failed, %s", i, err)
		}
	}
	if ttlms == 0 {
		return errArguments("invalid ttlms = %d", ttlms)
	}
	expireat := uint64(0)
	if v, ok := TTLmsToExpireAt(ttlms); ok && v > 0 {
		expireat = v
	} else {
		return errArguments("invalid ttlms = %d", ttlms)
	}

	if err := b.acquire(); err != nil {
		return err
	}
	defer b.release()

	bt := store.NewBatch()
	_, err := b.deleteIfExists(bt, db, key)
	if err != nil {
		return err
	}
	if !IsExpired(expireat) {
		o := newStringRow(db, key)
		o.ExpireAt, o.Value = expireat, value
		bt.Set(o.DataKey(), o.DataValue())
		bt.Set(o.MetaKey(), o.MetaValue())
		fw := &Forward{DB: db, Op: "PSetEX", Args: args}
		return b.commit(bt, fw)
	} else {
		fw := &Forward{DB: db, Op: "Del", Args: []interface{}{key}}
		return b.commit(bt, fw)
	}
}

// SETEX key seconds value
func (b *Binlog) SetEX(db uint32, args ...interface{}) error {
	if len(args) != 3 {
		return errArguments("len(args) = %d, expect = 3", len(args))
	}

	var key, value []byte
	var ttls int64
	for i, ref := range []interface{}{&key, &ttls, &value} {
		if err := parseArgument(args[i], ref); err != nil {
			return errArguments("parse args[%d] failed, %s", i, err)
		}
	}
	if ttls == 0 {
		return errArguments("invalid ttls = %d", ttls)
	}
	expireat := uint64(0)
	if v, ok := TTLsToExpireAt(ttls); ok && v > 0 {
		expireat = v
	} else {
		return errArguments("invalid ttls = %d", ttls)
	}

	if err := b.acquire(); err != nil {
		return err
	}
	defer b.release()

	bt := store.NewBatch()
	_, err := b.deleteIfExists(bt, db, key)
	if err != nil {
		return err
	}
	if !IsExpired(expireat) {
		o := newStringRow(db, key)
		o.ExpireAt, o.Value = expireat, value
		bt.Set(o.DataKey(), o.DataValue())
		bt.Set(o.MetaKey(), o.MetaValue())
		fw := &Forward{DB: db, Op: "SetEX", Args: args}
		return b.commit(bt, fw)
	} else {
		fw := &Forward{DB: db, Op: "Del", Args: []interface{}{key}}
		return b.commit(bt, fw)
	}
}

// SETNX key value
func (b *Binlog) SetNX(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 2 {
		return 0, errArguments("len(args) = %d, expect = 2", len(args))
	}

	var key, value []byte
	for i, ref := range []interface{}{&key, &value} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	o, err := b.loadBinlogRow(db, key, true)
	if err != nil || o != nil {
		return 0, err
	} else {
		o := newStringRow(db, key)
		o.Value = value
		bt := store.NewBatch()
		bt.Set(o.DataKey(), o.DataValue())
		bt.Set(o.MetaKey(), o.MetaValue())
		fw := &Forward{DB: db, Op: "Set", Args: args}
		return 1, b.commit(bt, fw)
	}
}

// GETSET key value
func (b *Binlog) GetSet(db uint32, args ...interface{}) ([]byte, error) {
	if len(args) != 2 {
		return nil, errArguments("len(args) = %d, expect = 2", len(args))
	}

	var key, value []byte
	for i, ref := range []interface{}{&key, &value} {
		if err := parseArgument(args[i], ref); err != nil {
			return nil, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return nil, err
	}
	defer b.release()

	o, err := b.loadStringRow(db, key, true)
	if err != nil {
		return nil, err
	}

	bt := store.NewBatch()
	if o != nil {
		_, err := o.LoadDataValue(b)
		if err != nil {
			return nil, err
		}
		if o.ExpireAt != 0 {
			o.ExpireAt = 0
			bt.Set(o.MetaKey(), o.MetaValue())
		}
	} else {
		o = newStringRow(db, key)
		bt.Set(o.MetaKey(), o.MetaValue())
	}
	o.Value, value = value, o.Value
	bt.Set(o.DataKey(), o.DataValue())
	fw := &Forward{DB: db, Op: "Set", Args: args}
	return value, b.commit(bt, fw)
}

func (b *Binlog) incrInt(db uint32, key []byte, delta int64) (int64, error) {
	o, err := b.loadStringRow(db, key, true)
	if err != nil {
		return 0, err
	}

	bt := store.NewBatch()
	if o != nil {
		_, err := o.LoadDataValue(b)
		if err != nil {
			return 0, err
		}
		v, err := ParseInt(o.Value)
		if err != nil {
			return 0, err
		}
		delta += v
	} else {
		o = newStringRow(db, key)
		bt.Set(o.MetaKey(), o.MetaValue())
	}
	o.Value = FormatInt(delta)
	bt.Set(o.DataKey(), o.DataValue())
	fw := &Forward{DB: db, Op: "IncrBy", Args: []interface{}{key, delta}}
	return delta, b.commit(bt, fw)
}

func (b *Binlog) incrFloat(db uint32, key []byte, delta float64) (float64, error) {
	o, err := b.loadStringRow(db, key, true)
	if err != nil {
		return 0, err
	}

	bt := store.NewBatch()
	if o != nil {
		_, err := o.LoadDataValue(b)
		if err != nil {
			return 0, err
		}
		v, err := ParseFloat(o.Value)
		if err != nil {
			return 0, err
		}
		delta += v
	} else {
		o = newStringRow(db, key)
		bt.Set(o.MetaKey(), o.MetaValue())
	}
	o.Value = FormatFloat(delta)
	bt.Set(o.DataKey(), o.DataValue())
	fw := &Forward{DB: db, Op: "IncrByFloat", Args: []interface{}{key, delta}}
	return delta, b.commit(bt, fw)
}

// INCR key
func (b *Binlog) Incr(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 1 {
		return 0, errArguments("len(args) = %d, expect = 1", len(args))
	}

	var key []byte
	for i, ref := range []interface{}{&key} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	return b.incrInt(db, key, 1)
}

// INCRBY key delta
func (b *Binlog) IncrBy(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 2 {
		return 0, errArguments("len(args) = %d, expect = 2", len(args))
	}

	var key []byte
	var delta int64
	for i, ref := range []interface{}{&key, &delta} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	return b.incrInt(db, key, delta)
}

// DECR key
func (b *Binlog) Decr(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 1 {
		return 0, errArguments("len(args) = %d, expect = 1", len(args))
	}

	var key []byte
	for i, ref := range []interface{}{&key} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	return b.incrInt(db, key, -1)
}

// DECRBY key delta
func (b *Binlog) DecrBy(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 2 {
		return 0, errArguments("len(args) = %d, expect = 2", len(args))
	}

	var key []byte
	var delta int64
	for i, ref := range []interface{}{&key, &delta} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	return b.incrInt(db, key, -delta)
}

// INCRBYFLOAT key delta
func (b *Binlog) IncrByFloat(db uint32, args ...interface{}) (float64, error) {
	if len(args) != 2 {
		return 0, errArguments("len(args) = %d, expect = 2", len(args))
	}

	var key []byte
	var delta float64
	for i, ref := range []interface{}{&key, &delta} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	return b.incrFloat(db, key, delta)
}

// SETBIT key offset value
func (b *Binlog) SetBit(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 3 {
		return 0, errArguments("len(args) = %d, expect = 3", len(args))
	}

	var key []byte
	var offset, value uint64
	for i, ref := range []interface{}{&key, &offset, &value} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}
	if offset > maxVarbytesLen {
		return 0, errArguments("offset = %d", offset)
	}

	var bit bool = value != 0

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	o, err := b.loadStringRow(db, key, true)
	if err != nil {
		return 0, err
	}

	bt := store.NewBatch()
	if o != nil {
		_, err := o.LoadDataValue(b)
		if err != nil {
			return 0, err
		}
	} else {
		o = newStringRow(db, key)
		bt.Set(o.MetaKey(), o.MetaValue())
	}
	ipos := offset / 8
	if n := int(ipos) + 1; n > len(o.Value) {
		o.Value = append(o.Value, make([]byte, n-len(o.Value))...)
	}
	mask := byte(1 << (offset % 8))
	orig := o.Value[ipos] & mask
	if bit {
		o.Value[ipos] |= mask
	} else {
		o.Value[ipos] &= ^mask
	}
	bt.Set(o.DataKey(), o.DataValue())

	var n int64 = 0
	if orig != 0 {
		n = 1
	}
	fw := &Forward{DB: db, Op: "SetBit", Args: args}
	return n, b.commit(bt, fw)
}

// SETRANGE key offset value
func (b *Binlog) SetRange(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 3 {
		return 0, errArguments("len(args) = %d, expect = 3", len(args))
	}

	var key, value []byte
	var offset uint64
	for i, ref := range []interface{}{&key, &offset, &value} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}
	if offset > maxVarbytesLen {
		return 0, errArguments("offset = %d", offset)
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	o, err := b.loadStringRow(db, key, true)
	if err != nil {
		return 0, err
	}

	bt := store.NewBatch()
	if o != nil {
		_, err := o.LoadDataValue(b)
		if err != nil {
			return 0, err
		}
	} else {
		o = newStringRow(db, key)
		bt.Set(o.MetaKey(), o.MetaValue())
	}
	if n := int(offset) + len(value); n > len(o.Value) {
		o.Value = append(o.Value, make([]byte, n-len(o.Value))...)
	}
	copy(o.Value[offset:], value)
	bt.Set(o.DataKey(), o.DataValue())
	fw := &Forward{DB: db, Op: "SetRange", Args: args}
	return int64(len(o.Value)), b.commit(bt, fw)
}

// MSET key value [key value ...]
func (b *Binlog) MSet(db uint32, args ...interface{}) error {
	if len(args) == 0 || len(args)%2 != 0 {
		return errArguments("len(args) = %d, expect != 0 && mod 2 = 0", len(args))
	}

	pairs := make([][]byte, len(args))
	for i := 0; i < len(args); i++ {
		if err := parseArgument(args[i], &pairs[i]); err != nil {
			return errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return err
	}
	defer b.release()

	ms := &markSet{}
	bt := store.NewBatch()
	for i := len(pairs)/2 - 1; i >= 0; i-- {
		key, value := pairs[i*2], pairs[i*2+1]
		if !ms.Has(key) {
			_, err := b.deleteIfExists(bt, db, key)
			if err != nil {
				return err
			}
			o := newStringRow(db, key)
			o.Value = value
			bt.Set(o.DataKey(), o.DataValue())
			bt.Set(o.MetaKey(), o.MetaValue())
			ms.Set(key)
		}
	}
	fw := &Forward{DB: db, Op: "MSet", Args: args}
	return b.commit(bt, fw)
}

// MSETNX key value [key value ...]
func (b *Binlog) MSetNX(db uint32, args ...interface{}) (int64, error) {
	if len(args) == 0 || len(args)%2 != 0 {
		return 0, errArguments("len(args) = %d, expect != 0 && mod 2 = 0", len(args))
	}

	pairs := make([][]byte, len(args))
	for i := 0; i < len(args); i++ {
		if err := parseArgument(args[i], &pairs[i]); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	for i := 0; i < len(pairs); i += 2 {
		o, err := b.loadBinlogRow(db, pairs[i], true)
		if err != nil || o != nil {
			return 0, err
		}
	}

	ms := &markSet{}
	bt := store.NewBatch()
	for i := len(pairs)/2 - 1; i >= 0; i-- {
		key, value := pairs[i*2], pairs[i*2+1]
		if !ms.Has(key) {
			o := newStringRow(db, key)
			o.Value = value
			bt.Set(o.DataKey(), o.DataValue())
			bt.Set(o.MetaKey(), o.MetaValue())
			ms.Set(key)
		}
	}
	fw := &Forward{DB: db, Op: "MSet", Args: args}
	return 1, b.commit(bt, fw)
}

// MGET key [key ...]
func (b *Binlog) MGet(db uint32, args ...interface{}) ([][]byte, error) {
	if len(args) == 0 {
		return nil, errArguments("len(args) = %d, expect != 0", len(args))
	}

	keys := make([][]byte, len(args))
	for i := 0; i < len(args); i++ {
		if err := parseArgument(args[i], &keys[i]); err != nil {
			return nil, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return nil, err
	}
	defer b.release()

	for _, key := range keys {
		_, err := b.loadBinlogRow(db, key, true)
		if err != nil {
			return nil, err
		}
	}

	values := make([][]byte, len(keys))
	for i, key := range keys {
		o, err := b.loadStringRow(db, key, false)
		if err != nil {
			return nil, err
		}
		if o != nil {
			_, err := o.LoadDataValue(b)
			if err != nil {
				return nil, err
			}
			values[i] = o.Value
		}
	}
	return values, nil
}

// GETBIT key offset
func (b *Binlog) GetBit(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 2 {
		return 0, errArguments("len(args) = %d, expect = 2", len(args))
	}

	var key []byte
	var offset uint64
	for i, ref := range []interface{}{&key, &offset} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}
	if offset > maxVarbytesLen {
		return 0, errArguments("offset = %d", offset)
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	o, err := b.loadStringRow(db, key, true)
	if err != nil || o == nil {
		return 0, err
	}

	if _, err := o.LoadDataValue(b); err != nil {
		return 0, err
	}

	ipos := offset / 8
	if n := int(ipos) + 1; n > len(o.Value) {
		return 0, nil
	}
	mask := byte(1 << (offset % 8))
	orig := o.Value[ipos] & mask
	if orig != 0 {
		return 1, nil
	} else {
		return 0, nil
	}
}

// GETRANGE key beg end
func (b *Binlog) GetRange(db uint32, args ...interface{}) ([]byte, error) {
	if len(args) != 3 {
		return nil, errArguments("len(args) = %d, expect = 3", len(args))
	}

	var key []byte
	var beg, end int64
	for i, ref := range []interface{}{&key, &beg, &end} {
		if err := parseArgument(args[i], ref); err != nil {
			return nil, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return nil, err
	}
	defer b.release()

	o, err := b.loadStringRow(db, key, true)
	if err != nil {
		return nil, err
	}

	if o != nil {
		_, err := o.LoadDataValue(b)
		if err != nil {
			return nil, err
		}
		min, max := int64(0), int64(len(o.Value))
		beg = maxIntValue(adjustIndex(beg, min, max), min)
		end = minIntValue(adjustIndex(end, min, max), max-1)
		if beg <= end {
			return o.Value[beg : end+1], nil
		}
	}
	return nil, nil
}

func adjustIndex(index int64, min, max int64) int64 {
	if index >= 0 {
		return index + min
	} else {
		return index + max
	}
}

func minIntValue(v1, v2 int64) int64 {
	if v1 < v2 {
		return v1
	} else {
		return v2
	}
}

func maxIntValue(v1, v2 int64) int64 {
	if v1 < v2 {
		return v2
	} else {
		return v1
	}
}

// STRLEN key
func (b *Binlog) Strlen(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 1 {
		return 0, errArguments("len(args) = %d, expect = 1", len(args))
	}

	var key []byte
	for i, ref := range []interface{}{&key} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	o, err := b.loadStringRow(db, key, true)
	if err != nil {
		return 0, err
	}

	if o != nil {
		_, err := o.LoadDataValue(b)
		if err != nil {
			return 0, err
		}
		return int64(len(o.Value)), nil
	}
	return 0, nil
}
