// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rdb

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/testing/assert"
)

func DecodeHexRdb(t *testing.T, s string, n int) map[string]*BinEntry {
	p, err := hex.DecodeString(strings.NewReplacer("\t", "", "\r", "", "\n", "", " ", "").Replace(s))
	assert.ErrorIsNil(t, err)
	r := bytes.NewReader(p)
	l := NewLoader(r)
	assert.ErrorIsNil(t, l.Header())
	entries := make(map[string]*BinEntry)
	var i int = 0
	for {
		e, err := l.NextBinEntry()
		assert.ErrorIsNil(t, err)
		if e == nil {
			break
		}
		assert.Must(t, e.DB == 0)
		entries[string(e.Key)] = e
		i++
	}
	assert.ErrorIsNil(t, l.Footer())
	assert.Must(t, r.Len() == 0)
	assert.Must(t, len(entries) == i && i == n)
	return entries
}

func getobj(t *testing.T, entries map[string]*BinEntry, key string) (*BinEntry, interface{}) {
	e := entries[key]
	assert.Must(t, e != nil)
	o, err := DecodeDump(e.Value)
	assert.ErrorIsNil(t, err)
	return e, o
}

/*
#!/bin/bash
./redis-cli flushall
for i in 1 255 256 65535 65536 2147483647 2147483648 4294967295 4294967296 -2147483648; do
	./redis-cli set string_${i} ${i}
done
./redis-cli save && xxd -p -c 32 dump.rdb
*/
func TestLoadIntString(t *testing.T) {
	s := `
		524544495330303036fe00000a737472696e675f323535c1ff00000873747269
		6e675f31c0010011737472696e675f343239343936373239360a343239343936
		373239360011737472696e675f343239343936373239350a3432393439363732
		39350012737472696e675f2d32313437343833363438c200000080000c737472
		696e675f3635353335c2ffff00000011737472696e675f323134373438333634
		380a32313437343833363438000c737472696e675f3635353336c20000010000
		0a737472696e675f323536c100010011737472696e675f323134373438333634
		37c2ffffff7fffe49d9f131fb5c3b5
	`
	values := []int{1, 255, 256, 65535, 65536, 2147483647, 2147483648, 4294967295, 4294967296, -2147483648}
	entries := DecodeHexRdb(t, s, len(values))
	for _, value := range values {
		key := fmt.Sprintf("string_%d", value)
		_, obj := getobj(t, entries, key)
		val := obj.(String)
		assert.Must(t, bytes.Equal([]byte(val), []byte(strconv.Itoa(value))))
	}
}

/*
#!/bin/bash
./redis-cli flushall
./redis-cli set string_ttls string_ttls
./redis-cli expireat string_ttls 1500000000
./redis-cli set string_ttlms string_ttlms
./redis-cli pexpireat string_ttlms 1500000000000
./redis-cli save && xxd -p -c 32 dump.rdb
*/
func TestLoadStringTTL(t *testing.T) {
	s := `
		524544495330303036fe00fc0098f73e5d010000000c737472696e675f74746c
		6d730c737472696e675f74746c6d73fc0098f73e5d010000000b737472696e67
		5f74746c730b737472696e675f74746c73ffd15acd935a3fe949
	`
	expireat := uint64(1500000000000)
	entries := DecodeHexRdb(t, s, 2)
	keys := []string{"string_ttls", "string_ttlms"}
	for _, key := range keys {
		e, obj := getobj(t, entries, key)
		val := obj.(String)
		assert.Must(t, bytes.Equal([]byte(val), []byte(key)))
		assert.Must(t, e.ExpireAt == expireat)
	}
}

/*
#!/bin/bash
s="01"
for ((i=0;i<15;i++)); do
    s=$s$s
done
./redis-cli flushall
./redis-cli set string_long $s
./redis-cli save && xxd -p -c 32 dump.rdb
*/
func TestLoadLongString(t *testing.T) {
	s := `
		524544495330303036fe00000b737472696e675f6c6f6e67c342f28000010000
		02303130e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0
		ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01
		e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff
		01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0
		ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01
		e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff
		01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0
		ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01
		e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff
		01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0
		ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01
		e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff
		01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0
		ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01
		e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff
		01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0
		ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01
		e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff
		01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0
		ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01
		e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff
		01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0
		ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01
		e0ff01e0ff01e0ff01e0ff01e03201013031ffdfdb02bd6d5da5e6
	`
	entries := DecodeHexRdb(t, s, 1)
	_, obj := getobj(t, entries, "string_long")
	val := []byte(obj.(String))
	for i := 0; i < (1 << 15); i++ {
		var c uint8 = '0'
		if i%2 != 0 {
			c = '1'
		}
		assert.Must(t, val[i] == c)
	}
}

/*
#!/bin/bash
./redis-cli flushall
for ((i=0;i<256;i++)); do
    ./redis-cli rpush list_lzf 0
    ./redis-cli rpush list_lzf 1
done
./redis-cli save && xxd -p -c 32 dump.rdb
*/
func TestLoadListZipmap(t *testing.T) {
	s := `
		524544495330303036fe000a086c6973745f6c7a66c31f440b040b0400000820
		0306000200f102f202e0ff03e1ff07e1ff07e1d90701f2ffff6a1c2d51c02301
		16
	`
	entries := DecodeHexRdb(t, s, 1)
	_, obj := getobj(t, entries, "list_lzf")
	val := obj.(List)
	assert.Must(t, len(val) == 512)
	for i := 0; i < 256; i++ {
		var s string = "0"
		if i%2 != 0 {
			s = "1"
		}
		assert.Must(t, string(val[i]) == s)
	}
}

/*
#!/bin/bash
./redis-cli flushall
for ((i=0;i<32;i++)); do
    ./redis-cli rpush list ${i}
done
./redis-cli save && xxd -p -c 32 dump.rdb
*/
func TestLoadList(t *testing.T) {
	s := `
		524544495330303036fe0001046c69737420c000c001c002c003c004c005c006
		c007c008c009c00ac00bc00cc00dc00ec00fc010c011c012c013c014c015c016
		c017c018c019c01ac01bc01cc01dc01ec01fff756ea1fa90adefe3
	`
	entries := DecodeHexRdb(t, s, 1)
	_, obj := getobj(t, entries, "list")
	val := obj.(List)
	assert.Must(t, len(val) == 32)
	for i := 0; i < 32; i++ {
		assert.Must(t, string(val[i]) == strconv.Itoa(i))
	}
}

/*
#!/bin/bash
./redis-cli flushall
for ((i=0;i<16;i++)); do
	./redis-cli sadd set1 ${i}
done
for ((i=0;i<32;i++)); do
	./redis-cli sadd set2 ${i}
done
./redis-cli save && xxd -p -c 32 dump.rdb
*/
func TestLoadSetAndSetIntset(t *testing.T) {
	s := `
		524544495330303036fe0002047365743220c016c00dc01bc012c01ac004c014
		c002c017c01dc01cc013c019c01ec008c006c000c001c007c00fc009c01fc00e
		c003c00ac015c010c00bc018c011c00cc0050b04736574312802000000100000
		0000000100020003000400050006000700080009000a000b000c000d000e000f
		00ff3a0a9697324d19c3
	`
	entries := DecodeHexRdb(t, s, 2)

	_, obj1 := getobj(t, entries, "set1")
	val1 := obj1.(Set)
	set1 := make(map[string]bool)
	for _, mem := range val1 {
		set1[string(mem)] = true
	}
	assert.Must(t, len(set1) == 16)
	assert.Must(t, len(set1) == len(val1))
	for i := 0; i < 16; i++ {
		_, ok := set1[strconv.Itoa(i)]
		assert.Must(t, ok)
	}

	_, obj2 := getobj(t, entries, "set2")
	val2 := obj2.(Set)
	set2 := make(map[string]bool)
	for _, mem := range val2 {
		set2[string(mem)] = true
	}
	assert.Must(t, len(set2) == 32)
	assert.Must(t, len(set2) == len(val2))
	for i := 0; i < 32; i++ {
		_, ok := set2[strconv.Itoa(i)]
		assert.Must(t, ok)
	}
}

/*
#!/bin/bash
./redis-cli flushall
for ((i=0;i<16;i++)); do
	./redis-cli hset hash1 ${i}
done
for ((i=-16;i<16;i++)); do
	./redis-cli hset hash2 ${i}
done
./redis-cli save && xxd -p -c 32 dump.rdb
*/
func TestLoadHashAndHashZiplist(t *testing.T) {
	s := `
		524544495330303036fe000405686173683220c00dc00dc0fcc0fcc0ffc0ffc0
		04c004c002c002c0fbc0fbc0f0c0f0c0f9c0f9c008c008c0fac0fac006c006c0
		00c000c001c001c0fec0fec007c007c0f6c0f6c00fc00fc009c009c0f7c0f7c0
		fdc0fdc0f1c0f1c0f2c0f2c0f3c0f3c00ec00ec003c003c00ac00ac00bc00bc0
		f8c0f8c00cc00cc0f5c0f5c0f4c0f4c005c0050d056861736831405151000000
		4d000000200000f102f102f202f202f302f302f402f402f502f502f602f602f7
		02f702f802f802f902f902fa02fa02fb02fb02fc02fc02fd02fd02fe0d03fe0d
		03fe0e03fe0e03fe0f03fe0fffffa423d3036c15e534
	`
	entries := DecodeHexRdb(t, s, 2)

	_, obj1 := getobj(t, entries, "hash1")
	val1 := obj1.(Hash)
	hash1 := make(map[string]string)
	for _, ent := range val1 {
		hash1[string(ent.Field)] = string(ent.Value)
	}
	assert.Must(t, len(hash1) == 16)
	assert.Must(t, len(hash1) == len(val1))
	for i := 0; i < 16; i++ {
		s := strconv.Itoa(i)
		assert.Must(t, hash1[s] == s)
	}

	_, obj2 := getobj(t, entries, "hash2")
	val2 := obj2.(Hash)
	hash2 := make(map[string]string)
	for _, ent := range val2 {
		hash2[string(ent.Field)] = string(ent.Value)
	}
	assert.Must(t, len(hash2) == 32)
	assert.Must(t, len(hash2) == len(val2))
	for i := -16; i < 16; i++ {
		s := strconv.Itoa(i)
		assert.Must(t, hash2[s] == s)
	}
}

/*
#!/bin/bash
./redis-cli flushall
for ((i=0;i<16;i++)); do
	./redis-cli zadd zset1 ${i} ${i}
done
for ((i=0;i<32;i++)); do
	./redis-cli zadd zset2 -${i} ${i}
done
./redis-cli save && xxd -p -c 32 dump.rdb
*/
func TestLoadZSetAndZSetZiplist(t *testing.T) {
	s := `
		524544495330303036fe0003057a7365743220c016032d3232c00d032d3133c0
		1b032d3237c012032d3138c01a032d3236c004022d34c014032d3230c002022d
		32c017032d3233c01d032d3239c01c032d3238c013032d3139c019032d3235c0
		1e032d3330c008022d38c006022d36c000022d30c001022d31c007022d37c009
		022d39c00f032d3135c01f032d3331c00e032d3134c003022d33c00a032d3130
		c015032d3231c010032d3136c00b032d3131c018032d3234c011032d3137c00c
		032d3132c005022d350c057a736574314051510000004d000000200000f102f1
		02f202f202f302f302f402f402f502f502f602f602f702f702f802f802f902f9
		02fa02fa02fb02fb02fc02fc02fd02fd02fe0d03fe0d03fe0e03fe0e03fe0f03
		fe0fffff2addedbf4f5a8f93
	`
	entries := DecodeHexRdb(t, s, 2)

	_, obj1 := getobj(t, entries, "zset1")
	val1 := obj1.(ZSet)
	zset1 := make(map[string]float64)
	for _, ent := range val1 {
		zset1[string(ent.Member)] = ent.Score
	}
	assert.Must(t, len(zset1) == 16)
	assert.Must(t, len(zset1) == len(val1))
	for i := 0; i < 16; i++ {
		s := strconv.Itoa(i)
		score, ok := zset1[s]
		assert.Must(t, ok)
		assert.Must(t, math.Abs(score-float64(i)) < 1e-10)
	}

	_, obj2 := getobj(t, entries, "zset2")
	val2 := obj2.(ZSet)
	zset2 := make(map[string]float64)
	for _, ent := range val2 {
		zset2[string(ent.Member)] = ent.Score
	}
	assert.Must(t, len(zset2) == 32)
	assert.Must(t, len(zset2) == len(val2))
	for i := 0; i < 32; i++ {
		s := strconv.Itoa(i)
		score, ok := zset2[s]
		assert.Must(t, ok)
		assert.Must(t, math.Abs(score+float64(i)) < 1e-10)
	}
}
