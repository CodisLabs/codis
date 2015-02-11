// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"bytes"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

type setRow struct {
	*binlogRowHelper

	Size   int64
	Member []byte
}

func newSetRow(db uint32, key []byte) *setRow {
	o := &setRow{}
	o.lazyInit(newBinlogRowHelper(db, key, SetCode))
	return o
}

func (o *setRow) lazyInit(h *binlogRowHelper) {
	o.binlogRowHelper = h
	o.dataKeyRefs = []interface{}{&o.Member}
	o.metaValueRefs = []interface{}{&o.Size}
	o.dataValueRefs = nil
}

func (o *setRow) deleteObject(b *Binlog, bt *store.Batch) error {
	it := b.getIterator()
	defer b.putIterator(it)
	for pfx := it.SeekTo(o.DataKeyPrefix()); it.Valid(); it.Next() {
		key := it.Key()
		if !bytes.HasPrefix(key, pfx) {
			break
		}
		bt.Del(key)
	}
	bt.Del(o.MetaKey())
	return it.Error()
}

func (o *setRow) storeObject(b *Binlog, bt *store.Batch, expireat uint64, obj interface{}) error {
	set, ok := obj.(rdb.Set)
	if !ok || len(set) == 0 {
		return errors.Trace(ErrObjectValue)
	}
	for i, m := range set {
		if len(m) != 0 {
			continue
		}
		return errArguments("set[%d], len(member) = %d", i, len(m))
	}

	ms := &markSet{}
	for _, o.Member = range set {
		ms.Set(o.Member)
		bt.Set(o.DataKey(), o.DataValue())
	}
	o.Size, o.ExpireAt = ms.Len(), expireat
	bt.Set(o.MetaKey(), o.MetaValue())
	return nil
}

func (o *setRow) loadObjectValue(r binlogReader) (interface{}, error) {
	it := r.getIterator()
	defer r.putIterator(it)
	set := make([][]byte, 0, o.Size)
	for pfx := it.SeekTo(o.DataKeyPrefix()); it.Valid(); it.Next() {
		key := it.Key()
		if !bytes.HasPrefix(key, pfx) {
			break
		}
		sfx := key[len(pfx):]
		if err := o.ParseDataKeySuffix(sfx); err != nil {
			return nil, err
		}
		if err := o.ParseDataValue(it.Value()); err != nil {
			return nil, err
		}
		set = append(set, o.Member)
	}
	if err := it.Error(); err != nil {
		return nil, err
	}
	if o.Size == 0 || int64(len(set)) != o.Size {
		return nil, errors.Errorf("len(set) = %d, set.size = %d", len(set), o.Size)
	}
	return rdb.Set(set), nil
}

func (o *setRow) getMembers(r binlogReader, count int64) ([][]byte, error) {
	it := r.getIterator()
	defer r.putIterator(it)
	var members [][]byte
	for pfx := it.SeekTo(o.DataKeyPrefix()); count > 0 && it.Valid(); it.Next() {
		key := it.Key()
		if !bytes.HasPrefix(key, pfx) {
			break
		}
		sfx := key[len(pfx):]
		if err := o.ParseDataKeySuffix(sfx); err != nil {
			return nil, err
		}
		if err := o.ParseDataValue(it.Value()); err != nil {
			return nil, err
		}
		if len(o.Member) == 0 {
			return nil, errors.Errorf("len(member) = %d", len(o.Member))
		}
		members = append(members, o.Member)
		count--
	}
	if err := it.Error(); err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, errors.Errorf("len(members) = %d, set.size = %d", len(members), o.Size)
	}
	return members, nil
}

func (b *Binlog) loadSetRow(db uint32, key []byte, deleteIfExpired bool) (*setRow, error) {
	o, err := b.loadBinlogRow(db, key, deleteIfExpired)
	if err != nil {
		return nil, err
	} else if o != nil {
		x, ok := o.(*setRow)
		if ok {
			return x, nil
		}
		return nil, errors.Trace(ErrNotSet)
	}
	return nil, nil
}

// SADD key member [member ...]
func (b *Binlog) SAdd(db uint32, args ...interface{}) (int64, error) {
	if len(args) < 2 {
		return 0, errArguments("len(args) = %d, expect >= 2", len(args))
	}

	var key []byte
	var members = make([][]byte, len(args)-1)
	if err := parseArgument(args[0], &key); err != nil {
		return 0, errArguments("parse args[%d] failed, %s", 0, err)
	}
	for i := 0; i < len(members); i++ {
		if err := parseArgument(args[i+1], &members[i]); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i+1, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	o, err := b.loadSetRow(db, key, true)
	if err != nil {
		return 0, err
	}

	if o == nil {
		o = newSetRow(db, key)
	}

	ms := &markSet{}
	bt := store.NewBatch()
	for _, o.Member = range members {
		exists, err := o.TestDataValue(b)
		if err != nil {
			return 0, err
		}
		if !exists {
			ms.Set(o.Member)
		}
		bt.Set(o.DataKey(), o.DataValue())
	}

	n := ms.Len()
	if n != 0 {
		o.Size += n
		bt.Set(o.MetaKey(), o.MetaValue())
	}
	fw := &Forward{DB: db, Op: "SAdd", Args: args}
	return n, b.commit(bt, fw)
}

// SCARD key
func (b *Binlog) SCard(db uint32, args ...interface{}) (int64, error) {
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

	o, err := b.loadSetRow(db, key, true)
	if err != nil || o == nil {
		return 0, err
	}
	return o.Size, nil
}

// SISMEMBER key member
func (b *Binlog) SIsMember(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 2 {
		return 0, errArguments("len(args) = %d, expect = 2", len(args))
	}

	var key, member []byte
	for i, ref := range []interface{}{&key, &member} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	o, err := b.loadSetRow(db, key, true)
	if err != nil || o == nil {
		return 0, err
	}

	o.Member = member
	exists, err := o.TestDataValue(b)
	if err != nil || !exists {
		return 0, err
	} else {
		return 1, nil
	}
}

// SMEMBERS key
func (b *Binlog) SMembers(db uint32, args ...interface{}) ([][]byte, error) {
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

	o, err := b.loadSetRow(db, key, true)
	if err != nil || o == nil {
		return nil, err
	}

	return o.getMembers(b, o.Size)
}

// SPOP key
func (b *Binlog) SPop(db uint32, args ...interface{}) ([]byte, error) {
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

	o, err := b.loadSetRow(db, key, true)
	if err != nil || o == nil {
		return nil, err
	}

	members, err := o.getMembers(b, 1)
	if err != nil || len(members) == 0 {
		return nil, err
	}
	o.Member = members[0]

	bt := store.NewBatch()
	bt.Del(o.DataKey())
	if o.Size--; o.Size > 0 {
		bt.Set(o.MetaKey(), o.MetaValue())
	} else {
		bt.Del(o.MetaKey())
	}
	fw := &Forward{DB: db, Op: "SRem", Args: []interface{}{key, members[0]}}
	return o.Member, b.commit(bt, fw)
}

// SRANDMEMBER key [count]
func (b *Binlog) SRandMember(db uint32, args ...interface{}) ([][]byte, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, errArguments("len(args) = %d, expect = 1 or 2", len(args))
	}

	var key []byte
	var count int64 = 1
	if err := parseArgument(args[0], &key); err != nil {
		return nil, errArguments("parse args[%d] failed, %s", 0, err)
	}
	if len(args) == 2 {
		if err := parseArgument(args[1], &count); err != nil {
			return nil, errArguments("parse args[%d] failed, %s", 1, err)
		}
	}

	if err := b.acquire(); err != nil {
		return nil, err
	}
	defer b.release()

	o, err := b.loadSetRow(db, key, true)
	if err != nil || o == nil {
		return nil, err
	}

	if count < 0 {
		count += o.Size
	}
	if count > 0 {
		return o.getMembers(b, count)
	} else {
		return nil, nil
	}
}

// SREM key member [member ...]
func (b *Binlog) SRem(db uint32, args ...interface{}) (int64, error) {
	if len(args) < 2 {
		return 0, errArguments("len(args) = %d, expect >= 2", len(args))
	}

	var key []byte
	var members = make([][]byte, len(args)-1)
	if err := parseArgument(args[0], &key); err != nil {
		return 0, errArguments("parse args[%d] failed, %s", 0, err)
	}
	for i := 0; i < len(members); i++ {
		if err := parseArgument(args[i+1], &members[i]); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i+1, err)
		}
	}

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	o, err := b.loadSetRow(db, key, true)
	if err != nil || o == nil {
		return 0, err
	}

	ms := &markSet{}
	bt := store.NewBatch()
	for _, o.Member = range members {
		if !ms.Has(o.Member) {
			exists, err := o.TestDataValue(b)
			if err != nil {
				return 0, err
			}
			if exists {
				bt.Del(o.DataKey())
				ms.Set(o.Member)
			}
		}
	}

	n := ms.Len()
	if n != 0 {
		if o.Size -= n; o.Size > 0 {
			bt.Set(o.MetaKey(), o.MetaValue())
		} else {
			bt.Del(o.MetaKey())
		}
	}
	fw := &Forward{DB: db, Op: "SRem", Args: args}
	return n, b.commit(bt, fw)
}
