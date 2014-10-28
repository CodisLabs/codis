package rdb

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

import (
	"github.com/cupcake/rdb"
	"github.com/cupcake/rdb/nopdecoder"
)

func Decode(p []byte) (interface{}, error) {
	d := &decoder{}
	if err := rdb.DecodeDump(p, 0, nil, 0, d); err != nil {
		return nil, err
	}
	return d.obj, d.err
}

func HexToString(p []byte) string {
	var b bytes.Buffer
	b.WriteByte('{')
	for _, c := range p {
		switch {
		case c >= 'a' && c <= 'z':
			fallthrough
		case c >= 'A' && c <= 'Z':
			fallthrough
		case c >= '0' && c <= '9':
			b.WriteByte(c)
		default:
			b.WriteByte('.')
		}
	}
	b.WriteByte('|')
	b.WriteString(hex.EncodeToString(p))
	b.WriteByte('}')
	return b.String()
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
		d.err = fmt.Errorf("invalid object, init again")
	} else {
		d.obj = obj
	}
}

func (d *decoder) Set(key, value []byte, expiry int64) {
	d.initObject(String(value))
}

func (d *decoder) StartHash(key []byte, length, expiry int64) {
	d.initObject(HashMap(nil))
}

func (d *decoder) Hset(key, field, value []byte) {
	if d.err != nil {
		return
	}
	switch h := d.obj.(type) {
	default:
		d.err = fmt.Errorf("invalid object, not a hashmap")
	case HashMap:
		v := struct {
			Field, Value []byte
		}{
			field,
			value,
		}
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
		d.err = fmt.Errorf("invalid object, not a set")
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
		d.err = fmt.Errorf("invalid object, not a list")
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
		d.err = fmt.Errorf("invalid object, not a zset")
	case ZSet:
		v := struct {
			Value []byte
			Score float64
		}{
			member,
			score,
		}
		d.obj = append(z, v)
	}
}

type String []byte
type List [][]byte
type HashMap []struct {
	Field, Value []byte
}
type Set [][]byte
type ZSet []struct {
	Value []byte
	Score float64
}
