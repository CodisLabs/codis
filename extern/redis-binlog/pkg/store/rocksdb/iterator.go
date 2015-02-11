// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rocksdb

import (
	"github.com/wandoulabs/codis/extern/redis-binlog/extern/gorocks"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

type Iterator struct {
	db  *RocksDB
	err error

	iter *gorocks.Iterator
}

func newIterator(db *RocksDB, ropt *gorocks.ReadOptions) *Iterator {
	return &Iterator{
		db:   db,
		iter: db.rkdb.NewIterator(ropt),
	}
}

func (it *Iterator) Close() {
	it.iter.Close()
}

func (it *Iterator) SeekTo(key []byte) []byte {
	it.iter.Seek(key)
	return key
}

func (it *Iterator) SeekToFirst() {
	it.iter.SeekToFirst()
}

func (it *Iterator) Valid() bool {
	return it.err == nil && it.iter.Valid()
}

func (it *Iterator) Next() {
	it.iter.Next()
}

func (it *Iterator) Key() []byte {
	return it.iter.Key()
}

func (it *Iterator) Value() []byte {
	return it.iter.Value()
}

func (it *Iterator) Error() error {
	if it.err == nil {
		if err := it.iter.GetError(); err != nil {
			it.err = errors.Trace(err)
		}
	}
	return it.err
}
