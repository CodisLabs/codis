// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rdb

import (
	"bytes"
	"encoding/binary"
	"hash"
	"io"
	"strconv"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb/digest"
)

type Loader struct {
	*rdbReader
	crc hash.Hash64
	db  uint32
}

func NewLoader(r io.Reader) *Loader {
	l := &Loader{}
	l.crc = digest.New()
	l.rdbReader = newRdbReader(io.TeeReader(r, l.crc))
	return l
}

func (l *Loader) Header() error {
	header := make([]byte, 9)
	if err := l.readFull(header); err != nil {
		return err
	}
	if !bytes.Equal(header[:5], []byte("REDIS")) {
		return errors.New("verify magic string, invalid file format")
	}
	if version, err := strconv.ParseInt(string(header[5:]), 10, 64); err != nil {
		return errors.Trace(err)
	} else if version <= 0 || version > Version {
		return errors.Errorf("verify version, invalid RDB version number %d", version)
	}
	return nil
}

func (l *Loader) Footer() error {
	crc1 := l.crc.Sum64()
	if crc2, err := l.readUint64(); err != nil {
		return err
	} else if crc1 != crc2 {
		return errors.New("checksum validation failed")
	}
	return nil
}

type BinEntry struct {
	DB       uint32
	Key      []byte
	Value    []byte
	ExpireAt uint64
}

func (e *BinEntry) ObjEntry() (*ObjEntry, error) {
	x, err := DecodeDump(e.Value)
	if err != nil {
		return nil, err
	}
	return &ObjEntry{
		DB:       e.DB,
		Key:      e.Key,
		Value:    x,
		ExpireAt: e.ExpireAt,
	}, nil
}

type ObjEntry struct {
	DB       uint32
	Key      []byte
	Value    interface{}
	ExpireAt uint64
}

func (e *ObjEntry) BinEntry() (*BinEntry, error) {
	p, err := EncodeDump(e.Value)
	if err != nil {
		return nil, err
	}
	return &BinEntry{
		DB:       e.DB,
		Key:      e.Key,
		Value:    p,
		ExpireAt: e.ExpireAt,
	}, nil
}

func (l *Loader) NextBinEntry() (*BinEntry, error) {
	var entry = &BinEntry{}
	for {
		t, err := l.readByte()
		if err != nil {
			return nil, err
		}
		switch t {
		case rdbFlagExpiryMS:
			ttlms, err := l.readUint64()
			if err != nil {
				return nil, err
			}
			entry.ExpireAt = ttlms
		case rdbFlagExpiry:
			ttls, err := l.readUint32()
			if err != nil {
				return nil, err
			}
			entry.ExpireAt = uint64(ttls) * 1000
		case rdbFlagSelectDB:
			dbnum, err := l.readLength()
			if err != nil {
				return nil, err
			}
			l.db = dbnum
		case rdbFlagEOF:
			return nil, nil
		default:
			key, err := l.readString()
			if err != nil {
				return nil, err
			}
			val, err := l.readObjectValue(t)
			if err != nil {
				return nil, err
			}
			entry.DB = l.db
			entry.Key = key
			entry.Value = createValueDump(t, val)
			return entry, nil
		}
	}
}

func createValueDump(t byte, val []byte) []byte {
	var b bytes.Buffer
	c := digest.New()
	w := io.MultiWriter(&b, c)
	w.Write([]byte{t})
	w.Write(val)
	binary.Write(w, binary.LittleEndian, uint16(Version))
	binary.Write(w, binary.LittleEndian, c.Sum64())
	return b.Bytes()
}
