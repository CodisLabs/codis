// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package store

type Database interface {
	Close()
	Clear() error
	NewIterator() Iterator
	NewSnapshot() Snapshot
	Commit(bt *Batch) error
	Compact(start, limit []byte) error
	Get(key []byte) ([]byte, error)
	Stats() string
}
