// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package leveldb

import (
	"github.com/wandoulabs/codis/extern/redis-binlog/extern/levigo"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

type Snapshot struct {
	db *LevelDB

	snap *levigo.Snapshot
	ropt *levigo.ReadOptions
}

func newSnapshot(db *LevelDB) *Snapshot {
	snap := db.lvdb.NewSnapshot()
	ropt := levigo.NewReadOptions()
	ropt.SetFillCache(false)
	ropt.SetSnapshot(snap)
	return &Snapshot{
		db:   db,
		snap: snap,
		ropt: ropt,
	}
}

func (sp *Snapshot) Close() {
	sp.ropt.Close()
	sp.db.lvdb.ReleaseSnapshot(sp.snap)
}

func (sp *Snapshot) NewIterator() store.Iterator {
	return newIterator(sp.db, sp.ropt)
}

func (sp *Snapshot) Get(key []byte) ([]byte, error) {
	value, err := sp.db.lvdb.Get(sp.ropt, key)
	return value, errors.Trace(err)
}
