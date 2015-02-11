// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package leveldb

import (
	"bytes"
	"fmt"
	"os"

	"github.com/wandoulabs/codis/extern/redis-binlog/extern/levigo"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

type LevelDB struct {
	path  string
	lvdb  *levigo.DB
	opts  *levigo.Options
	ropt  *levigo.ReadOptions
	wopt  *levigo.WriteOptions
	cache *levigo.Cache
	bloom *levigo.FilterPolicy
}

func Open(path string, conf *Config, create, repair bool) (*LevelDB, error) {
	db := &LevelDB{}
	if err := db.init(path, conf, create, repair); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (db *LevelDB) init(path string, conf *Config, create, repair bool) error {
	if conf == nil {
		conf = NewDefaultConfig()
	}
	opts := levigo.NewOptions()
	if create {
		opts.SetCreateIfMissing(true)
		opts.SetErrorIfExists(true)
	} else {
		opts.SetCreateIfMissing(false)
		opts.SetErrorIfExists(false)
	}

	opts.SetCompression(levigo.SnappyCompression)
	opts.SetBlockSize(conf.BlockSize)
	opts.SetWriteBufferSize(conf.WriteBufferSize)
	opts.SetMaxOpenFiles(conf.MaxOpenFiles)

	cache := levigo.NewLRUCache(conf.CacheSize)
	opts.SetCache(cache)

	bloom := levigo.NewBloomFilter(conf.BloomFilterSize)
	opts.SetFilterPolicy(bloom)

	db.path = path
	db.opts = opts
	db.ropt = levigo.NewReadOptions()
	db.wopt = levigo.NewWriteOptions()
	db.cache = cache
	db.bloom = bloom

	if create {
		if err := os.MkdirAll(db.path, 0700); err != nil {
			return errors.Trace(err)
		}
	} else if repair {
		if err := levigo.RepairDatabase(db.path, db.opts); err != nil {
			return errors.Trace(err)
		}
	}

	var err error
	if db.lvdb, err = levigo.Open(db.path, db.opts); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (db *LevelDB) Clear() error {
	if db.lvdb != nil {
		db.lvdb.Close()
		db.lvdb = nil
		db.opts.SetCreateIfMissing(true)
		db.opts.SetErrorIfExists(true)
		if err := levigo.DestroyDatabase(db.path, db.opts); err != nil {
			return errors.Trace(err)
		} else if db.lvdb, err = levigo.Open(db.path, db.opts); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (db *LevelDB) Close() {
	if db.lvdb != nil {
		db.lvdb.Close()
	}
	db.opts.Close()
	db.ropt.Close()
	db.wopt.Close()
	db.cache.Close()
	db.bloom.Close()
}

func (db *LevelDB) NewIterator() store.Iterator {
	return newIterator(db, db.ropt)
}

func (db *LevelDB) NewSnapshot() store.Snapshot {
	return newSnapshot(db)
}

func (db *LevelDB) Get(key []byte) ([]byte, error) {
	value, err := db.lvdb.Get(db.ropt, key)
	return value, errors.Trace(err)
}

func (db *LevelDB) Commit(bt *store.Batch) error {
	if bt.OpList.Len() == 0 {
		return nil
	}
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	for e := bt.OpList.Front(); e != nil; e = e.Next() {
		switch op := e.Value.(type) {
		case *store.BatchOpSet:
			wb.Put(op.Key, op.Value)
		case *store.BatchOpDel:
			wb.Delete(op.Key)
		default:
			panic(fmt.Sprintf("unsupported batch operation: %+v", op))
		}
	}
	return errors.Trace(db.lvdb.Write(db.wopt, wb))
}

func (db *LevelDB) Compact(start, limit []byte) error {
	db.lvdb.CompactRange(levigo.Range{start, limit})
	return nil
}

func (db *LevelDB) Stats() string {
	var b bytes.Buffer
	for _, s := range []string{"leveldb.stats", "leveldb.sstables"} {
		v := db.lvdb.PropertyValue(s)
		fmt.Fprintf(&b, "[%s]\n%s\n", s, v)
	}
	return b.String()
}
