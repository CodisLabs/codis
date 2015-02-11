// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rocksdb

import (
	"github.com/wandoulabs/codis/extern/redis-binlog/extern/gorocks"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

type Snapshot struct {
	db *RocksDB

	snap *gorocks.Snapshot
	ropt *gorocks.ReadOptions
}

func newSnapshot(db *RocksDB, fillcache bool) *Snapshot {
	snap := db.rkdb.NewSnapshot()
	ropt := gorocks.NewReadOptions()
	ropt.SetFillCache(fillcache)
	ropt.SetSnapshot(snap)
	return &Snapshot{
		db:   db,
		snap: snap,
		ropt: ropt,
	}
}

func (sp *Snapshot) Close() {
	sp.ropt.Close()
	sp.db.rkdb.ReleaseSnapshot(sp.snap)
}

func (sp *Snapshot) NewIterator() store.Iterator {
	return newIterator(sp.db, sp.ropt)
}

func (sp *Snapshot) Get(key []byte) ([]byte, error) {
	value, err := sp.db.rkdb.Get(sp.ropt, key)
	return value, errors.Trace(err)
}
