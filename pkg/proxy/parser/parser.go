// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package parser

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	"github.com/juju/errors"
)

/*
 * redis protocal : Resp protocol
 * http://redis.io/topics/protocol
 */
var (
	NEW_LINE   = []byte("\r\n")
	EMPTY_LINE []byte
)

const (
	ErrorResp = iota
	SimpleString
	IntegerResp
	BulkResp
	MultiResp
	NoKey
)

type Resp struct {
	Type    int
	Error   string
	Status  string
	Integer int64  // Support Redis 64bit integer
	Bulk    []byte // Support Redis Null Bulk Resp
	Multi   []*Resp
}

var (
	noKeyOps = map[string]string{
		"PING":       "fakeKey",
		"SLOTSNUM":   "fakeKey",
		"SLOTSCHECK": "fakeKey",
	}

	keyFun    = make(map[string]funGetKeys)
	intBuffer [][]byte
)

func init() {
	for _, v := range thridAsKeyTbl {
		keyFun[v] = thridAsKey
	}

	cnt := 10000
	intBuffer = make([][]byte, cnt)
	for i := 0; i < cnt; i++ {
		intBuffer[i] = []byte(strconv.Itoa(i))
	}
}

func Itoa(i int) []byte {
	if i < 0 {
		return []byte(strconv.Itoa(i))
	}

	if i < len(intBuffer) {
		return intBuffer[i]
	}

	return []byte(strconv.Itoa(i))
}

//todo: overflow
func Btoi(b []byte) (int, error) {
	n := 0
	sign := 1
	for i := uint8(0); i < uint8(len(b)); i++ {
		if i == 0 && b[i] == '-' {
			if len(b) == 1 {
				return 0, errors.Errorf("Invalid number %s", string(b))
			}
			sign = -1
			continue
		}

		if b[i] >= 0 && b[i] <= '9' {
			if i > 0 {
				n *= 10
			}
			n += int(b[i]) - '0'
			continue
		}

		return 0, errors.Errorf("Invalid number %s", string(b))
	}

	return sign * n, nil
}

func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(line) < 2 || line[len(line)-2] != '\r' { // \r\n
		return nil, errors.Errorf("invalid redis packet %v, err:%v", line, err)
	}
	if len(line) == 2 {
		return EMPTY_LINE, nil
	}

	line = line[:len(line)-2] //strip \r\n
	return line, nil
}

func (r *Resp) Op() ([]byte, error) {
	if len(r.Multi) > 0 {
		return r.Multi[0].Bulk, nil
	}

	return nil, errors.Errorf("invalid resp %+v", r)
}

type funGetKeys func(r *Resp) ([][]byte, error)

func defaultGetKeys(r *Resp) ([][]byte, error) {
	count := len(r.Multi[1:])
	if count == 0 {
		return nil, nil
	}

	keys := make([][]byte, 0, count)
	for _, v := range r.Multi[1:] {
		keys = append(keys, v.Bulk)
	}

	return keys, nil
}

func Parse(r *bufio.Reader) (*Resp, error) {
	line, err := readLine(r)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if len(line) == 0 {
		return nil, errors.New("empty NEW_LINE")
	}

	switch line[0] {
	case '-':
		return &Resp{
			Type:  ErrorResp,
			Error: string(line[1:]),
		}, nil
	case '+':
		return &Resp{
			Type:   SimpleString,
			Status: string(line[1:]),
		}, nil
	case ':':
		i, err := Btoi(line[1:])
		if err != nil {
			return nil, errors.Trace(err)
		}
		return &Resp{
			Type:    IntegerResp,
			Integer: int64(i),
		}, nil
	case '$':
		size, err := Btoi(line[1:])
		if err != nil {
			return nil, errors.Trace(err)
		}
		bulk, err := ReadBulk(r, size)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return &Resp{
			Type: BulkResp,
			Bulk: bulk,
		}, nil
	case '*':
		i, err := Btoi(line[1:])
		if err != nil {
			return nil, errors.Trace(err)
		}
		rp := &Resp{Type: MultiResp}
		if i >= 0 {
			multi := make([]*Resp, i)
			for j := 0; j < i; j++ {
				rp, err := Parse(r)
				if err != nil {
					return nil, errors.Trace(err)
				}
				multi[j] = rp
			}
			rp.Multi = multi
		}
		return rp, nil
	default:
		if !IsLetter(line[0]) {
			return nil, errors.New("redis protocol error, " + string(line))
		}
		rp := &Resp{Type: MultiResp}
		for _, s := range strings.Split(string(line), " ") {
			if str := strings.TrimSpace(s); len(str) > 0 {
				rp.Multi = append(rp.Multi, &Resp{Type: BulkResp, Bulk: []byte(s)})
			}
		}
		return rp, nil
	}

	return nil, errors.New("redis protocol error, " + string(line))
}

func IsLetter(c byte) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}

	if c >= 'A' && c <= 'Z' {
		return true
	}

	return false
}

func ReadBulk(r *bufio.Reader, size int) ([]byte, error) {
	if size < 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	line, err := readLine(r)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if len(line) != 0 {
		return nil, errors.New("should be just 0 " + string(line))
	}

	return buf, nil
}

var thridAsKeyTbl = []string{"ZINTERSTORE", "ZUNIONSTORE", "EVAL", "EVALSHA"}

func thridAsKey(r *Resp) ([][]byte, error) {
	if len(r.Multi) < 4 {
		return nil, errors.New("invalid argument, key not found")
	}

	numKeys, err := Btoi(r.Multi[2].Bulk)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var keys [][]byte
	for _, v := range r.Multi[3:] {
		keys = append(keys, v.Bulk)
		if len(keys) == numKeys {
			break
		}
	}

	return keys, nil
}

func (r *Resp) Keys() ([][]byte, error) {
	key, err := r.Op()
	if err != nil {
		return nil, err
	}

	f, ok := keyFun[string(key)]
	if !ok {
		return defaultGetKeys(r)
	}

	return f(r)
}

func (r *Resp) Key() ([]byte, error) {
	keys, err := r.Keys()
	if len(keys) > 0 {
		return keys[0], err
	}

	return []byte("fakeKey"), nil
}

func (r *Resp) getBulkBuf() []byte {
	v := r.Bulk
	buf := make([]byte, 0, 20+len(v))
	buf = append(buf, '$')
	if v == nil {
		buf = append(buf, []byte("-1")...)
	} else if len(v) == 0 {
		buf = append(buf, []byte("0")...)
		buf = append(buf, NEW_LINE...)
	} else {
		buf = append(buf, Itoa(len(v))...)
		buf = append(buf, NEW_LINE...)
		buf = append(buf, v...)
	}

	buf = append(buf, NEW_LINE...)
	return buf
}

func (r *Resp) getSimpleStringBuf() []byte {
	buf := make([]byte, 0, 20+len(r.Status))
	buf = append(buf, '+')
	buf = append(buf, r.Status...)
	buf = append(buf, NEW_LINE...)
	return buf
}

func (r *Resp) getErrorBuf() []byte {
	buf := make([]byte, 0, 20+len(r.Error))
	buf = append(buf, '-')
	buf = append(buf, r.Error...)
	buf = append(buf, NEW_LINE...)
	return buf
}

func (r *Resp) getIntegerBuf() []byte {
	buf := make([]byte, 0, 20+len(NEW_LINE))
	buf = append(buf, ':')
	buf = append(buf, Itoa(int(r.Integer))...)
	buf = append(buf, NEW_LINE...)
	return buf
}

func (r *Resp) Bytes() ([]byte, error) {
	var buf []byte
	switch r.Type {
	case NoKey:
		buf = append(buf, r.Bulk...)
		buf = append(buf, NEW_LINE...)
	case SimpleString:
		buf = r.getSimpleStringBuf()
	case ErrorResp:
		buf = r.getErrorBuf()
	case IntegerResp:
		buf = r.getIntegerBuf()
	case BulkResp:
		buf = r.getBulkBuf()
	case MultiResp:
		length := len(r.Multi)
		if r.Multi == nil {
			length = -1
		}

		buf = make([]byte, 0, 256)
		buf = append(buf, '*')
		buf = append(buf, Itoa(length)...)
		buf = append(buf, NEW_LINE...)

		if len(r.Multi) > 0 {
			for _, resp := range r.Multi {
				slice, err := resp.Bytes()
				if err != nil {
					return nil, errors.Trace(err)
				}
				buf = append(buf, slice...)
			}
		}
	}

	return buf, nil
}
