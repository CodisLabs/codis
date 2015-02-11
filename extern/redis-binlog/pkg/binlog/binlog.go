// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"container/list"
	"sync"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
)

var (
	ErrClosed = errors.Static("binlog has been closed")
)

type Binlog struct {
	mu sync.Mutex
	db store.Database

	splist list.List
	itlist list.List
	serial uint64
}

func New(db store.Database) *Binlog {
	return &Binlog{db: db}
}

func (b *Binlog) acquire() error {
	b.mu.Lock()
	if b.db != nil {
		return nil
	}
	b.mu.Unlock()
	return errors.Trace(ErrClosed)
}

func (b *Binlog) release() {
	b.mu.Unlock()
}

func (b *Binlog) commit(bt *store.Batch, fw *Forward) error {
	if bt.Len() == 0 {
		return nil
	}
	if err := b.db.Commit(bt); err != nil {
		log.WarnErrorf(err, "binlog commit failed")
		return err
	}
	for i := b.itlist.Len(); i != 0; i-- {
		v := b.itlist.Remove(b.itlist.Front()).(*binlogIterator)
		v.Close()
	}
	b.serial++
	return nil
}

func (b *Binlog) getRowValue(key []byte) ([]byte, error) {
	return b.db.Get(key)
}

func (b *Binlog) getIterator() (it *binlogIterator) {
	if e := b.itlist.Front(); e != nil {
		return b.itlist.Remove(e).(*binlogIterator)
	}
	return &binlogIterator{
		Iterator: b.db.NewIterator(),
		serial:   b.serial,
	}
}

func (b *Binlog) putIterator(it *binlogIterator) {
	if it.serial == b.serial && it.Error() == nil {
		b.itlist.PushFront(it)
	} else {
		it.Close()
	}
}

func (b *Binlog) Close() {
	if err := b.acquire(); err != nil {
		return
	}
	defer b.release()
	log.Infof("binlog is closing ...")
	for i := b.splist.Len(); i != 0; i-- {
		v := b.splist.Remove(b.splist.Front()).(*BinlogSnapshot)
		v.Close()
	}
	for i := b.itlist.Len(); i != 0; i-- {
		v := b.itlist.Remove(b.itlist.Front()).(*binlogIterator)
		v.Close()
	}
	if b.db != nil {
		b.db.Close()
		b.db = nil
	}
	log.Infof("binlog is closed")
}

func (b *Binlog) NewSnapshot() (*BinlogSnapshot, error) {
	if err := b.acquire(); err != nil {
		return nil, err
	}
	defer b.release()
	sp := &BinlogSnapshot{sp: b.db.NewSnapshot()}
	b.splist.PushBack(sp)
	log.Infof("binlog create new snapshot, address = %p", sp)
	return sp, nil
}

func (b *Binlog) ReleaseSnapshot(sp *BinlogSnapshot) {
	if err := b.acquire(); err != nil {
		return
	}
	defer b.release()
	log.Infof("binlog release snapshot, address = %p", sp)
	for i := b.splist.Len(); i != 0; i-- {
		v := b.splist.Remove(b.splist.Front()).(*BinlogSnapshot)
		if v != sp {
			b.splist.PushBack(v)
		}
	}
	sp.Close()
}

func (b *Binlog) Reset() error {
	if err := b.acquire(); err != nil {
		return err
	}
	defer b.release()
	log.Infof("binlog is reseting...")
	for i := b.splist.Len(); i != 0; i-- {
		v := b.splist.Remove(b.splist.Front()).(*BinlogSnapshot)
		v.Close()
	}
	for i := b.itlist.Len(); i != 0; i-- {
		v := b.itlist.Remove(b.itlist.Front()).(*binlogIterator)
		v.Close()
	}
	if err := b.db.Clear(); err != nil {
		b.db.Close()
		b.db = nil
		log.ErrorErrorf(err, "binlog reset failed")
		return err
	} else {
		b.serial++
		log.Infof("binlog is reset")
		return nil
	}
}

func (b *Binlog) compact(start, limit []byte) error {
	if err := b.db.Compact(start, limit); err != nil {
		log.ErrorErrorf(err, "binlog compact failed")
		return err
	} else {
		return nil
	}
}

func errArguments(format string, v ...interface{}) error {
	err := errors.Errorf(format, v...)
	log.DebugErrorf(err, "call binlog function with invalid arguments")
	return err
}
