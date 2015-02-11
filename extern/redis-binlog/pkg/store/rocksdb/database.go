// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rocksdb

import (
	"bytes"
	"fmt"
	"os"

	"github.com/wandoulabs/codis/extern/redis-binlog/extern/gorocks"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

type RocksDB struct {
	path string
	rkdb *gorocks.DB
	opts *gorocks.Options
	ropt *gorocks.ReadOptions
	wopt *gorocks.WriteOptions

	env   *gorocks.Env
	topts *gorocks.TableOptions
	cache *gorocks.Cache

	snapshotFillCache bool
}

func Open(path string, conf *Config, create, repair bool) (*RocksDB, error) {
	db := &RocksDB{}
	if err := db.init(path, conf, create, repair); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (db *RocksDB) init(path string, conf *Config, create, repair bool) error {
	if conf == nil {
		conf = NewDefaultConfig()
	}
	opts := gorocks.NewOptions()
	if create {
		opts.SetCreateIfMissing(true)
		opts.SetErrorIfExists(true)
	} else {
		opts.SetCreateIfMissing(false)
		opts.SetErrorIfExists(false)
	}

	opts.SetCompression(gorocks.Lz4Compression)
	opts.SetBlockSize(conf.BlockSize)
	opts.SetWriteBufferSize(conf.WriteBufferSize)
	opts.SetMaxOpenFiles(conf.MaxOpenFiles)
	opts.SetNumLevels(conf.NumLevels)

	opts.SetMaxWriteBufferNumber(conf.MaxWriteBufferNumber)
	opts.SetMinWriteBufferNumberToMerge(conf.MinWriteBufferNumberToMerge)
	opts.SetLevel0FileNumCompactionTrigger(conf.Level0FileNumCompactionTrigger)
	opts.SetLevel0SlowdownWritesTrigger(conf.Level0SlowdownWritesTrigger)
	opts.SetLevel0StopWritesTrigger(conf.Level0StopWritesTrigger)
	opts.SetTargetFileSizeBase(conf.TargetFileSizeBase)
	opts.SetTargetFileSizeMultiplier(conf.TargetFileSizeMultiplier)
	opts.SetMaxBytesForLevelBase(conf.MaxBytesForLevelBase)
	opts.SetMaxBytesForLevelMultiplier(conf.MaxBytesForLevelMultiplier)

	opts.SetDisableAutoCompactions(conf.DisableAutoCompactions)
	opts.SetDisableDataSync(conf.DisableDataSync)
	opts.SetUseFsync(conf.UseFsync)
	opts.SetMaxBackgroundCompactions(conf.MaxBackgroundCompactions)
	opts.SetMaxBackgroundFlushes(conf.MaxBackgroundFlushes)
	opts.SetAllowOSBuffer(conf.AllowOSBuffer)

	topts := gorocks.NewTableOptions()
	topts.SetBlockSize(conf.BlockSize)

	cache := gorocks.NewLRUCache(conf.CacheSize)
	topts.SetCache(cache)

	topts.SetFilterPolicy(gorocks.NewBloomFilter(conf.BloomFilterSize))
	opts.SetBlockBasedTableFactory(topts)

	env := gorocks.NewDefaultEnv()
	env.SetBackgroundThreads(conf.BackgroundThreads)
	env.SetHighPriorityBackgroundThreads(conf.HighPriorityBackgroundThreads)
	opts.SetEnv(env)

	db.path = path
	db.opts = opts
	db.ropt = gorocks.NewReadOptions()
	db.wopt = gorocks.NewWriteOptions()
	db.env = env
	db.topts = topts
	db.cache = cache
	db.snapshotFillCache = conf.SnapshotFillCache

	if create {
		if err := os.MkdirAll(db.path, 0700); err != nil {
			return errors.Trace(err)
		}
	} else if repair {
		if err := gorocks.RepairDatabase(db.path, db.opts); err != nil {
			return errors.Trace(err)
		}
	}

	var err error
	if db.rkdb, err = gorocks.Open(db.path, db.opts); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (db *RocksDB) Clear() error {
	if db.rkdb != nil {
		db.rkdb.Close()
		db.rkdb = nil
		db.opts.SetCreateIfMissing(true)
		db.opts.SetErrorIfExists(true)
		if err := gorocks.DestroyDatabase(db.path, db.opts); err != nil {
			return errors.Trace(err)
		} else if db.rkdb, err = gorocks.Open(db.path, db.opts); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (db *RocksDB) Close() {
	if db.rkdb != nil {
		db.rkdb.Close()
	}
	db.opts.Close()
	db.ropt.Close()
	db.wopt.Close()
	db.env.Close()
	db.topts.Close()
	db.cache.Close()
}

func (db *RocksDB) NewIterator() store.Iterator {
	return newIterator(db, db.ropt)
}

func (db *RocksDB) NewSnapshot() store.Snapshot {
	return newSnapshot(db, db.snapshotFillCache)
}

func (db *RocksDB) Get(key []byte) ([]byte, error) {
	value, err := db.rkdb.Get(db.ropt, key)
	return value, errors.Trace(err)
}

func (db *RocksDB) Commit(bt *store.Batch) error {
	if bt.OpList.Len() == 0 {
		return nil
	}
	wb := gorocks.NewWriteBatch()
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
	return errors.Trace(db.rkdb.Write(db.wopt, wb))
}

func (db *RocksDB) Compact(start, limit []byte) error {
	db.rkdb.CompactRange(gorocks.Range{start, limit})
	return nil
}

func (db *RocksDB) Stats() string {
	var b bytes.Buffer
	for _, s := range []string{"rocksdb.stats", "rocksdb.sstables"} {
		v := db.rkdb.PropertyValue(s)
		fmt.Fprintf(&b, "[%s]\n%s\n", s, v)
	}
	return b.String()
}
