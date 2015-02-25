// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"os"
	"strconv"
	"testing"

	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/binlog"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/rdb"
)

func TestBgsaveTo(t *testing.T) {
	c := client(t)
	k := random(t)
	checkok(t, c, "flushall")
	const max = 100
	for i := 0; i < max; i++ {
		checkok(t, c, "set", k+strconv.Itoa(i), i)
	}
	path := "/tmp/testdb-dump.rdb"
	checkok(t, c, "bgsaveto", path)
	f, err := os.Open(path)
	checkerror(t, err, true)
	defer f.Close()
	l := rdb.NewLoader(f)
	checkerror(t, l.Header(), true)
	m := make(map[string][]byte)
	for {
		e, err := l.NextBinEntry()
		checkerror(t, err, true)
		if e == nil {
			break
		}
		checkerror(t, nil, e.DB == 0)
		checkerror(t, nil, e.ExpireAt == 0)
		m[string(e.Key)] = e.Value
	}
	checkerror(t, l.Footer(), true)
	for i := 0; i < max; i++ {
		b := m[k+strconv.Itoa(i)]
		o, err := rdb.DecodeDump(b)
		checkerror(t, err, true)
		x, ok := o.(rdb.String)
		checkerror(t, nil, ok)
		checkerror(t, nil, string(x) == string(binlog.FormatInt(int64(i))))
	}
}
