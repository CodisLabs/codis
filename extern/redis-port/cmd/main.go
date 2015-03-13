// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/bytesize"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
)

var args struct {
	input    string
	output   string
	parallel int

	from   string
	target string
	extra  bool

	sockfile string
	filesize int64

	auth string

	shift time.Duration
}

const (
	ReaderBufferSize = bytesize.MB * 32
	WriterBufferSize = bytesize.MB * 8
)

func parseInt(s string, min, max int) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n >= min && n <= max {
		return n, nil
	}
	return 0, errors.Errorf("out of range [%d,%d], got %d", min, max, n)
}

const (
	MinDB = 0
	MaxDB = 1023
)

var acceptDB = func(db uint32) bool {
	return db >= MinDB && db <= MaxDB
}

func main() {
	usage := `
Usage:
	redis-port decode   [--ncpu=N]  [--parallel=M]  [--input=INPUT]  [--output=OUTPUT]
	redis-port restore  [--ncpu=N]  [--parallel=M]  [--input=INPUT]   --target=TARGET  [--extra]     [--faketime=FAKETIME] [--filterdb=DB]
	redis-port dump     [--ncpu=N]  [--parallel=M]   --from=MASTER   [--output=OUTPUT] [--extra] [--password=PASSWORD]
	redis-port sync     [--ncpu=N]  [--parallel=M]   --from=MASTER    --target=TARGET  [--sockfile=FILE [--filesize=SIZE]] [--filterdb=DB] [--password=PASSWORD]

Options:
	-P PASSWORD, --password           Set master's auth code.
	-n N, --ncpu=N                    Set runtime.GOMAXPROCS to N.
	-p M, --parallel=M                Set the number of parallel routines to M.
	-i INPUT, --input=INPUT           Set input file, default is stdin ('/dev/stdin').
	-o OUTPUT, --output=OUTPUT        Set output file, default is stdout ('/dev/stdout').
	-f MASTER, --from=MASTER          Set host:port of master redis.
	-t TARGET, --target=TARGET        Set host:port of slave redis.
	--faketime=FAKETIME               Set current system time to adjust key's expire time.
	--sockfile=FILE                   Use FILE to as socket buffer, default is disabled.
	--filesize=SIZE                   Set FILE size, default value is 1gb.
	-e, --extra                       Set ture to send/receive following redis commands, default is false.
	--filterdb=DB                     Filter db = DB, default is *.
`
	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}

	if s, ok := d["--ncpu"].(string); ok && s != "" {
		n, err := parseInt(s, 1, 1024)
		if err != nil {
			log.PanicErrorf(err, "parse --ncpu failed")
		}
		runtime.GOMAXPROCS(n)
	}
	ncpu := runtime.GOMAXPROCS(0)

	if s, ok := d["--parallel"].(string); ok && s != "" {
		n, err := parseInt(s, 1, 1024)
		if err != nil {
			log.PanicErrorf(err, "parse --parallel failed")
		}
		args.parallel = n
	}
	if ncpu > args.parallel {
		args.parallel = ncpu
	}
	if args.parallel == 0 {
		args.parallel = 4
	}

	args.input, _ = d["--input"].(string)
	args.output, _ = d["--output"].(string)

	args.target, _ = d["--target"].(string)
	args.from, _ = d["--from"].(string)

	args.extra, _ = d["--extra"].(bool)
	args.sockfile, _ = d["--sockfile"].(string)

	args.auth, _ = d["--password"].(string)

	if s, ok := d["--faketime"].(string); ok && s != "" {
		switch s[0] {
		case '-', '+':
			d, err := time.ParseDuration(strings.ToLower(s))
			if err != nil {
				log.PanicError(err, "parse --faketime failed")
			}
			args.shift = d
		case '@':
			n, err := strconv.ParseInt(s[1:], 10, 64)
			if err != nil {
				log.PanicError(err, "parse --faketime failed")
			}
			args.shift = time.Duration(n*int64(time.Millisecond) - time.Now().UnixNano())
		default:
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				log.PanicError(err, "parse --faketime failed")
			}
			args.shift = time.Duration(t.UnixNano() - time.Now().UnixNano())
		}
	}

	if s, ok := d["--filterdb"].(string); ok && s != "" && s != "*" {
		n, err := parseInt(s, MinDB, MaxDB)
		if err != nil {
			log.PanicError(err, "parse --filterdb failed")
		}
		u := uint32(n)
		acceptDB = func(db uint32) bool {
			return db == u
		}
	}

	if s, ok := d["--filesize"].(string); ok && s != "" {
		if len(args.sockfile) == 0 {
			log.Panic("please specify --sockfile first")
		}
		n, err := bytesize.Parse(s)
		if err != nil {
			log.PanicError(err, "parse --filesize failed")
		}
		if n <= 0 {
			log.Panicf("parse --filesize = %d, invalid number", n)
		}
		args.filesize = n
	} else {
		args.filesize = bytesize.GB
	}

	log.Infof("set ncpu = %d, parallel = %d\n", ncpu, args.parallel)

	switch {
	case d["decode"].(bool):
		new(cmdDecode).Main()
	case d["restore"].(bool):
		new(cmdRestore).Main()
	case d["dump"].(bool):
		new(cmdDump).Main()
	case d["sync"].(bool):
		new(cmdSync).Main()
	}
}
