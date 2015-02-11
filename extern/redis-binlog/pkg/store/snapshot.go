// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package store

type Snapshot interface {
	Close()
	NewIterator() Iterator
	Get(key []byte) ([]byte, error)
}
