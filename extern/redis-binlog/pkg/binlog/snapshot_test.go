// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import (
	"fmt"
	"testing"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

func TestSnapshot(t *testing.T) {
	xsetex(t, 0, "string", "value", 10)
	kpexpire(t, 0, "string", 10, 1)

	now := nowms()

	m := make(map[string]string)
	for db := uint32(0); db < 128; db++ {
		key := fmt.Sprintf("key_%d", db)
		val := fmt.Sprintf("val_%d", db)
		m[key] = val
		ss := []string{}
		for k, v := range m {
			ss = append(ss, k, v)
		}
		hmset(t, db, "hash", ss...)
		kpexpireat(t, db, "hash", now+1000*uint64(db+37), 1)
	}

	sleepms(20)

	s, err := testbl.NewSnapshot()
	checkerror(t, err, true)

	objs, _, err := s.LoadObjCron(time.Hour, 4, 4096)
	checkerror(t, err, len(objs) == 128)

	kpttl(t, 0, "string", -2)

	testbl.ReleaseSnapshot(s)

	for db := uint32(0); db < 128; db++ {
		ok := false
		for _, obj := range objs {
			if obj.DB != db {
				continue
			}
			ok = true
			checkerror(t, nil, string(obj.Key) == "hash")
			checkerror(t, nil, obj.ExpireAt == now+uint64(db+37)*1000)
			x := obj.Value.(rdb.Hash)
			checkerror(t, err, len(x) == int(db+1))
			for _, e := range x {
				checkerror(t, nil, m[string(e.Field)] == string(e.Value))
			}
		}
		checkerror(t, err, ok)
		hdelall(t, db, "hash", 1)
	}
	checkcompact(t)
	checkempty(t)

	s, err = testbl.NewSnapshot()
	checkerror(t, err, true)
	objs, _, err = s.LoadObjCron(time.Hour, 4, 4096)
	checkerror(t, err, len(objs) == 0)
	testbl.ReleaseSnapshot(s)
	checkempty(t)
}
