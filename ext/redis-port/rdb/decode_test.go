// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rdb

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
)

func hexStringToObject(t *testing.T, s string) (obj interface{}) {
	p, err := hex.DecodeString(strings.NewReplacer("\t", "", "\r", "", "\n", "", " ", "").Replace(s))
	if err != nil {
		t.Fatalf("decode hex string error = '%s'", err)
	}
	obj, err = DecodeDump(p)
	if err != nil {
		t.Fatalf("decode dump error = '%s'", err)
	}
	if obj == nil {
		t.Fatalf("decode object is nil")
	}
	return obj
}

/*
#!/bin/bash
for i in 1 255 256 65535 65536 2147483647 2147483648 4294967295 4294967296; do
	./redis-cli set string ${i}
	./redis-cli dump string
done
./redis-cli set string "hello world!!"
./redis-cli dump string
s=""
for ((i=0;i<1024;i++)); do
	s="01"$s
done
./redis-cli set string $s
./redis-cli dump string
*/
func TestDecodeString(t *testing.T) {
	docheck := func(hexs string, expect string) {
		val := hexStringToObject(t, hexs).(String)
		if bytes.Compare([]byte(val), []byte(expect)) != 0 {
			t.Fatalf("string = '%v', expect = '%s'", hexs, val, expect)
		}
	}
	docheck("00c0010600b0958f3624542d6f", "1")
	docheck("00c1ff0006004a42131348a52fa4", "255")
	docheck("00c1000106009cb3bb1c58e36c78", "256")
	docheck("00c2ffff0000060047a5299686680606", "65535")
	docheck("00c200000100060056e7032772340449", "65536")
	docheck("00c2ffffff7f0600ba998d1e157b9132", "2147483647")
	docheck("000a323134373438333634380600715c4123b9484a7d", "2147483648")
	docheck("000a3432393439363732393506009a94b642c60c15f2", "4294967295")
	docheck("000a343239343936373239360600334ee148efd97ac5", "4294967296")

	docheck("000d68656c6c6f20776f726c64212106004aa70c88a8ad3601", "hello world!!")
	var b bytes.Buffer
	for i := 0; i < 1024; i++ {
		b.Write([]byte("01"))
	}
	docheck("00c31f480002303130e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ff01e0ba010130310600bcd6e486102c99c7", b.String())
}

/*
#!/bin/bash
for ((i=0;i<32;i++)); do
	./redis-cli rpush list $i
done
./redis-cli dump list
*/
func TestDecodeListZipmap(t *testing.T) {
	s := `
		0a405e5e0000005a000000200000f102f202f302f402f502f602f702f802f902
		fa02fb02fc02fd02fe0d03fe0e03fe0f03fe1003fe1103fe1203fe1303fe1403
		fe1503fe1603fe1703fe1803fe1903fe1a03fe1b03fe1c03fe1d03fe1e03fe1f
		ff060052f7f617938b332a
	`
	val := hexStringToObject(t, s).(List)
	if len(val) != 32 {
		t.Fatalf("len(list) = %d, expect = 32", len(val))
	}
	for i := 0; i < len(val); i++ {
		if string(val[i]) != strconv.Itoa(i) {
			t.Fatalf("list[%d] = '%v', expect = %d", i, val[i], i)
		}
	}
}

/*
#!/bin/bash
for ((i=0;i<32;i++)); do
	./redis-cli rpush list $i
done
./redis-cli dump list
*/
func TestDecodeList(t *testing.T) {
	s := `
		0120c000c001c002c003c004c005c006c007c008c009c00ac00bc00cc00dc00e
		c00fc010c011c012c013c014c015c016c017c018c019c01ac01bc01cc01dc01e
		c01f0600e87781cbebc997f5
	`
	val := hexStringToObject(t, s).(List)
	if len(val) != 32 {
		t.Fatalf("len(list) = %d, expect = 32)", len(val))
	}
	for i := 0; i < len(val); i++ {
		if string(val[i]) != strconv.Itoa(i) {
			t.Fatalf("list[%d] = '%v', expect = %d", i, val[i], i)
		}
	}
}

/*
#!/bin/bash
for ((i=0;i<32;i++)); do
	./redis-cli sadd set $i
done
./redis-cli dump set
*/
func TestDecodeSet(t *testing.T) {
	s := `
		0220c016c00dc01bc012c01ac004c014c002c017c01dc01cc013c019c01ec008
		c006c000c001c007c00fc009c01fc00ec003c00ac015c010c00bc018c011c00c
		c00506007bd0a89270890016
	`
	val := hexStringToObject(t, s).(Set)
	if len(val) != 32 {
		t.Fatalf("len(set) = %d, expect = 32", len(val))
	}
	set := make(map[string]bool)
	for _, mem := range val {
		set[string(mem)] = true
	}
	if len(set) != 32 || len(val) != len(set) {
		t.Fatalf("len(set) = %d/%d, expect = 32", len(set), len(val))
	}
	for i := 0; i < 32; i++ {
		if _, ok := set[strconv.Itoa(i)]; !ok {
			t.Fatalf("set missing %d", i)
		}
	}
}

/*
#!/bin/bash
for ((i=0;i<32;i++)); do
	let j="$i*$i"
	./redis-cli hset hash $i $j
done
./redis-cli dump hash
*/
func TestDecodeHash(t *testing.T) {
	s := `
		0420c016c1e401c00dc1a900c01bc1d902c012c14401c01ac1a402c004c010c0
		02c004c014c19001c017c11102c01dc14903c01cc11003c013c16901c019c171
		02c01ec18403c008c040c006c024c000c000c001c001c007c031c009c051c00f
		c1e100c01fc1c103c00ec1c400c003c009c00ac064c015c1b901c010c10001c0
		0bc079c018c14002c011c12101c00cc19000c005c019060072320e870e10799d
	`
	val := hexStringToObject(t, s).(HashMap)
	if len(val) != 32 {
		t.Fatalf("len(hash) = %d, expect = 32", len(val))
	}
	hash := make(map[string]string)
	for _, ent := range val {
		hash[string(ent.Field)] = string(ent.Value)
	}
	if len(hash) != 32 || len(val) != len(hash) {
		t.Fatalf("len(hash) = %d/%d, expect = 32", len(hash), len(val))
	}
	for i := 0; i < 32; i++ {
		s := strconv.Itoa(i)
		if hash[s] != strconv.Itoa(i*i) {
			t.Fatalf("hash[%s] is error", s)
		}
	}
}

/*
#!/bin/bash
for ((i=0;i<32;i++)); do
	./redis-cli zadd zset -$i $i
done
./redis-cli dump zset
*/
func TestDecodeZSet(t *testing.T) {
	s := `
		0320c016032d3232c00d032d3133c01b032d3237c012032d3138c01a032d3236
		c004022d34c014032d3230c002022d32c017032d3233c01d032d3239c01c032d
		3238c013032d3139c019032d3235c01e032d3330c008022d38c006022d36c000
		0130c001022d31c007022d37c009022d39c00f032d3135c01f032d3331c00e03
		2d3134c003022d33c00a032d3130c015032d3231c010032d3136c00b032d3131
		c018032d3234c011032d3137c00c032d3132c005022d35060046177397f6688b
		16
	`
	val := hexStringToObject(t, s).(ZSet)
	if len(val) != 32 {
		t.Fatalf("len(zset) = %d, expect = 32", len(val))
	}
	zset := make(map[string]float64)
	for _, ent := range val {
		zset[string(ent.Member)] = ent.Score
	}
	if len(zset) != 32 || len(zset) != len(val) {
		t.Fatalf("len(zset) = %d/%d, expect = 32", len(zset), len(val))
	}
	for i := 0; i < 32; i++ {
		s := strconv.Itoa(i)
		if score, ok := zset[s]; !ok || score != -float64(i) {
			t.Fatalf("zset[%s] is error", s)
		}
	}
}
