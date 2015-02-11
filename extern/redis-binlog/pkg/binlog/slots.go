// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"time"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

const (
	MaxSlotNum = 1024
)

func HashTag(key []byte) []byte {
	part := key
	if i := bytes.IndexByte(part, '{'); i != -1 {
		part = part[i+1:]
	} else {
		return key
	}
	if i := bytes.IndexByte(part, '}'); i != -1 {
		return part[:i]
	} else {
		return key
	}
}

func HashTagToSlot(tag []byte) uint32 {
	return crc32.ChecksumIEEE(tag) % MaxSlotNum
}

func HashKeyToSlot(key []byte) ([]byte, uint32) {
	tag := HashTag(key)
	return tag, HashTagToSlot(tag)
}

// SLOTSINFO [start] [count]
func (b *Binlog) SlotsInfo(db uint32, args ...interface{}) (map[uint32]int64, error) {
	if len(args) > 2 {
		return nil, errArguments("len(args) = %d, expect <= 2", len(args))
	}

	var start, count uint32 = 0, MaxSlotNum
	switch len(args) {
	case 2:
		if err := parseArgument(args[1], &count); err != nil {
			return nil, errArguments("parse args[%d] failed, %s", 1, err)
		}
		fallthrough
	case 1:
		if err := parseArgument(args[0], &start); err != nil {
			return nil, errArguments("parse args[%d] failed, %s", 0, err)
		}
	}
	limit := start + count

	if err := b.acquire(); err != nil {
		return nil, err
	}
	defer b.release()

	m := make(map[uint32]int64)
	for slot := start; slot < limit && slot < MaxSlotNum; slot++ {
		if key, err := firstKeyUnderSlot(b, db, slot); err != nil {
			return nil, err
		} else if key != nil {
			m[slot] = 1
		} else {
			m[slot] = 0
		}
	}
	return m, nil
}

// SLOTSRESTORE key ttlms value [key ttlms value ...]
func (b *Binlog) SlotsRestore(db uint32, args ...interface{}) error {
	if len(args) == 0 || len(args)%3 != 0 {
		return errArguments("len(args) = %d, expect != 0 && mod 3 = 0", len(args))
	}

	objs := make([]*rdb.ObjEntry, len(args)/3)
	for i := 0; i < len(objs); i++ {
		var key, value []byte
		var ttlms int64
		for j, ref := range []interface{}{&key, &ttlms, &value} {
			if err := parseArgument(args[i*3+j], ref); err != nil {
				return errArguments("parse args[%d] failed, %s", i*3+j, err)
			}
		}
		expireat := uint64(0)
		if ttlms != 0 {
			if v, ok := TTLmsToExpireAt(ttlms); ok && v > 0 {
				expireat = v
			} else {
				return errArguments("parse args[%d] ttlms = %d", i*3+1, ttlms)
			}
		}
		obj, err := rdb.DecodeDump(value)
		if err != nil {
			return errArguments("decode args[%d] failed, %s", i*3+2, err)
		}
		objs[i] = &rdb.ObjEntry{
			DB:       db,
			Key:      key,
			ExpireAt: expireat,
			Value:    obj,
		}
	}

	if err := b.acquire(); err != nil {
		return err
	}
	defer b.release()

	ms := &markSet{}
	bt := store.NewBatch()
	for i := len(objs) - 1; i >= 0; i-- {
		e := objs[i]
		if ms.Has(e.Key) {
			log.Debugf("[%d] restore batch, db = %d, key = %v, ignore", i, e.DB, e.Key)
			continue
		} else {
			log.Debugf("[%d] restore batch, db = %d, key = %v", i, e.DB, e.Key)
		}
		if err := b.restore(bt, e.DB, e.Key, e.ExpireAt, e.Value); err != nil {
			log.DebugErrorf(err, "restore object failed, db = %d, key = %v", e.DB, e.Key)
			return err
		}
		ms.Set(e.Key)
	}
	fw := &Forward{DB: db, Op: "SlotsRestore", Args: args}
	return b.commit(bt, fw)
}

// SLOTSMGRTSLOT host port timeout slot
func (b *Binlog) SlotsMgrtSlot(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 4 {
		return 0, errArguments("len(args) = %d, expect = 4", len(args))
	}

	var host string
	var port int64
	var ttlms uint64
	var slot uint32
	for i, ref := range []interface{}{&host, &port, &ttlms, &slot} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}
	var timeout = time.Duration(ttlms) * time.Millisecond
	if slot >= MaxSlotNum {
		return 0, errArguments("slot = %d", slot)
	}
	if timeout == 0 {
		timeout = time.Second
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	log.Debugf("migrate slot, addr = %s, timeout = %d, db = %d, slot = %d", addr, timeout, db, slot)

	key, err := firstKeyUnderSlot(b, db, slot)
	if err != nil || key == nil {
		return 0, err
	}
	return b.migrateOne(addr, timeout, db, key)
}

// SLOTSMGRTTAGSLOT host port timeout slot
func (b *Binlog) SlotsMgrtTagSlot(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 4 {
		return 0, errArguments("len(args) = %d, expect = 4", len(args))
	}

	var host string
	var port int64
	var ttlms uint64
	var slot uint32
	for i, ref := range []interface{}{&host, &port, &ttlms, &slot} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}
	var timeout = time.Duration(ttlms) * time.Millisecond
	if slot >= MaxSlotNum {
		return 0, errArguments("slot = %d", slot)
	}
	if timeout == 0 {
		timeout = time.Second
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	log.Debugf("migrate slot with tag, addr = %s, timeout = %d, db = %d, slot = %d", addr, timeout, db, slot)

	key, err := firstKeyUnderSlot(b, db, slot)
	if err != nil || key == nil {
		return 0, err
	}

	if tag := HashTag(key); len(tag) == len(key) {
		return b.migrateOne(addr, timeout, db, key)
	} else {
		return b.migrateTag(addr, timeout, db, tag)
	}
}

// SLOTSMGRTONE host port timeout key
func (b *Binlog) SlotsMgrtOne(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 4 {
		return 0, errArguments("len(args) = %d, expect = 4", len(args))
	}

	var host string
	var port int64
	var ttlms uint64
	var key []byte
	for i, ref := range []interface{}{&host, &port, &ttlms, &key} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}
	var timeout = time.Duration(ttlms) * time.Millisecond
	if timeout == 0 {
		timeout = time.Second
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	log.Debugf("migrate one, addr = %s, timeout = %d, db = %d, key = %v", addr, timeout, db, key)

	return b.migrateOne(addr, timeout, db, key)
}

// SLOTSMGRTTAGONE host port timeout key
func (b *Binlog) SlotsMgrtTagOne(db uint32, args ...interface{}) (int64, error) {
	if len(args) != 4 {
		return 0, errArguments("len(args) = %d, expect = 4", len(args))
	}

	var host string
	var port int64
	var ttlms uint64
	var key []byte
	for i, ref := range []interface{}{&host, &port, &ttlms, &key} {
		if err := parseArgument(args[i], ref); err != nil {
			return 0, errArguments("parse args[%d] failed, %s", i, err)
		}
	}
	var timeout = time.Duration(ttlms) * time.Millisecond
	if timeout == 0 {
		timeout = time.Second
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	if err := b.acquire(); err != nil {
		return 0, err
	}
	defer b.release()

	log.Debugf("migrate one with tag, addr = %s, timeout = %d, db = %d, key = %v", addr, timeout, db, key)

	if tag := HashTag(key); len(tag) == len(key) {
		return b.migrateOne(addr, timeout, db, key)
	} else {
		return b.migrateTag(addr, timeout, db, tag)
	}
}

func (b *Binlog) migrateOne(addr string, timeout time.Duration, db uint32, key []byte) (int64, error) {
	n, err := b.migrate(addr, timeout, db, key)
	if err != nil {
		log.ErrorErrorf(err, "migrate one failed")
		return 0, err
	}
	return n, nil
}

func (b *Binlog) migrateTag(addr string, timeout time.Duration, db uint32, tag []byte) (int64, error) {
	keys, err := allKeysWithTag(b, db, tag)
	if err != nil || len(keys) == 0 {
		return 0, err
	}
	n, err := b.migrate(addr, timeout, db, keys...)
	if err != nil {
		log.ErrorErrorf(err, "migrate tag failed")
		return 0, err
	}
	return n, nil
}

func (b *Binlog) migrate(addr string, timeout time.Duration, db uint32, keys ...[]byte) (int64, error) {
	var rows []binlogRow
	var bins []*rdb.BinEntry

	for i, key := range keys {
		o, bin, err := loadBinEntry(b, db, key)
		if err != nil {
			return 0, err
		}
		if o == nil {
			log.Debugf("[%d] missing, db = %d, key = %v", i, db, key)
			continue
		}

		rows = append(rows, o)
		if bin != nil {
			log.Debugf("[%d] migrate, db = %d, key = %v, expireat = %d", i, db, key, o.GetExpireAt())
			bins = append(bins, bin)
		} else {
			log.Debugf("[%d] expired, db = %d, key = %v, expireat = %d", i, db, key, o.GetExpireAt())
		}
	}

	if len(bins) != 0 {
		if err := doMigrate(addr, timeout, db, bins); err != nil {
			return 0, err
		}
	}

	if len(rows) == 0 {
		return 0, nil
	}

	bt := store.NewBatch()
	for _, o := range rows {
		if err := o.deleteObject(b, bt); err != nil {
			return 0, err
		}
	}
	fw := &Forward{DB: db, Op: "Del"}
	for _, key := range keys {
		fw.Args = append(fw.Args, key)
	}
	return int64(len(rows)), b.commit(bt, fw)
}
