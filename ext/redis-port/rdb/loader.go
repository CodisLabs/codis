// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rdb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"strconv"
)

import (
	"github.com/wandoulabs/codis/ext/redis-port/rdb/digest"
)

type Loader struct {
	*rdbReader
	crc   hash.Hash64
	dbnum uint32
}

func NewLoader(r io.Reader) *Loader {
	l := &Loader{}
	l.crc = digest.New()
	l.rdbReader = newRdbReader(io.TeeReader(r, l.crc))
	return l
}

func (l *Loader) LoadHeader() error {
	header := make([]byte, 9)
	if err := l.readFull(header); err != nil {
		return err
	}
	if !bytes.Equal(header[:5], []byte("REDIS")) {
		return fmt.Errorf("verify magic string, invalid file format")
	}
	version, _ := strconv.ParseInt(string(header[5:]), 10, 64)
	if version <= 0 || version > Version {
		return fmt.Errorf("verify version, invalid RDB version number %d", version)
	}
	return nil
}

func (l *Loader) LoadChecksum() error {
	crc1 := l.crc.Sum64()
	if crc2, err := l.readUint64(); err != nil {
		return err
	} else if crc1 != crc2 {
		return fmt.Errorf("checksum validation failed")
	}
	return nil
}

type Entry struct {
	DB       uint32
	Key      []byte
	ValDump  []byte
	ExpireAt uint64
}

func (l *Loader) LoadEntry() (entry *Entry, offset int64, err error) {
	defer func() {
		offset = l.offset()
	}()
	var expireat uint64
	for {
		var otype byte
		if otype, err = l.readByte(); err != nil {
			return
		}
		switch otype {
		case rdbFlagExpiryMS:
			if expireat, err = l.readUint64(); err != nil {
				return
			}
		case rdbFlagExpiry:
			var sec uint32
			if sec, err = l.readUint32(); err != nil {
				return
			}
			expireat = uint64(sec) * 1000
		case rdbFlagSelectDB:
			if l.dbnum, err = l.readLength(); err != nil {
				return
			}
		case rdbFlagEOF:
			return
		default:
			var key, obj []byte
			if key, err = l.readString(); err != nil {
				return
			}
			if obj, err = l.readObject(otype); err != nil {
				return
			}
			entry = &Entry{}
			entry.DB = l.dbnum
			entry.Key = key
			entry.ValDump = createDump(otype, obj)
			entry.ExpireAt = expireat
			return
		}
	}
}

func createDump(otype byte, obj []byte) []byte {
	var b bytes.Buffer
	c := digest.New()
	w := io.MultiWriter(&b, c)
	w.Write([]byte{otype})
	w.Write(obj)
	binary.Write(w, binary.LittleEndian, uint16(Version))
	binary.Write(w, binary.LittleEndian, c.Sum64())
	return b.Bytes()
}
