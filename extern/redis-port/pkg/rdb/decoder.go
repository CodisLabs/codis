// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rdb

import (
	"bytes"

	"github.com/cupcake/rdb"
	"github.com/cupcake/rdb/nopdecoder"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

func DecodeDump(p []byte) (interface{}, error) {
	d := &decoder{}
	if err := rdb.DecodeDump(p, 0, nil, 0, d); err != nil {
		return nil, errors.Trace(err)
	}
	return d.obj, d.err
}

type decoder struct {
	nopdecoder.NopDecoder
	obj interface{}
	err error
}

func (d *decoder) initObject(obj interface{}) {
	if d.err != nil {
		return
	}
	if d.obj != nil {
		d.err = errors.New("invalid object, init again")
	} else {
		d.obj = obj
	}
}

func (d *decoder) Set(key, value []byte, expiry int64) {
	d.initObject(String(value))
}

func (d *decoder) StartHash(key []byte, length, expiry int64) {
	d.initObject(Hash(nil))
}

func (d *decoder) Hset(key, field, value []byte) {
	if d.err != nil {
		return
	}
	switch h := d.obj.(type) {
	default:
		d.err = errors.New("invalid object, not a hashmap")
	case Hash:
		v := &HashElement{Field: field, Value: value}
		d.obj = append(h, v)
	}
}

func (d *decoder) StartSet(key []byte, cardinality, expiry int64) {
	d.initObject(Set(nil))
}

func (d *decoder) Sadd(key, member []byte) {
	if d.err != nil {
		return
	}
	switch s := d.obj.(type) {
	default:
		d.err = errors.New("invalid object, not a set")
	case Set:
		d.obj = append(s, member)
	}
}

func (d *decoder) StartList(key []byte, length, expiry int64) {
	d.initObject(List(nil))
}

func (d *decoder) Rpush(key, value []byte) {
	if d.err != nil {
		return
	}
	switch l := d.obj.(type) {
	default:
		d.err = errors.New("invalid object, not a list")
	case List:
		d.obj = append(l, value)
	}
}

func (d *decoder) StartZSet(key []byte, cardinality, expiry int64) {
	d.initObject(ZSet(nil))
}

func (d *decoder) Zadd(key []byte, score float64, member []byte) {
	if d.err != nil {
		return
	}
	switch z := d.obj.(type) {
	default:
		d.err = errors.New("invalid object, not a zset")
	case ZSet:
		v := &ZSetElement{Member: member, Score: score}
		d.obj = append(z, v)
	}
}

type String []byte
type List [][]byte
type Hash []*HashElement
type ZSet []*ZSetElement
type Set [][]byte

type HashElement struct {
	Field, Value []byte
}

type ZSetElement struct {
	Member []byte
	Score  float64
}

func (hash Hash) Len() int {
	return len(hash)
}

func (hash Hash) Swap(i, j int) {
	hash[i], hash[j] = hash[j], hash[i]
}

type HSortByField struct{ Hash }

func (by HSortByField) Less(i, j int) bool {
	return bytes.Compare(by.Hash[i].Field, by.Hash[j].Field) < 0
}

func (zset ZSet) Len() int {
	return len(zset)
}

func (zset ZSet) Swap(i, j int) {
	zset[i], zset[j] = zset[j], zset[i]
}

type ZSortByMember struct{ ZSet }

func (by ZSortByMember) Less(i, j int) bool {
	return bytes.Compare(by.ZSet[i].Member, by.ZSet[j].Member) < 0
}

type ZSortByScore struct{ ZSet }

func (by ZSortByScore) Less(i, j int) bool {
	return by.ZSet[i].Score < by.ZSet[j].Score
}
