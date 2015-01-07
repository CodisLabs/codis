// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/utils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/utils/bytesize"
)

var args struct {
	ncpu   int
	input  string
	output string

	from   string
	target string
	extra  bool

	sockfile string
	filesize int64

	shift time.Duration
}

const (
	ReaderBufferSize = bytesize.MB * 32
	WriterBufferSize = bytesize.MB * 8
)

var acceptDB func(db int64) bool

func main() {
	usage := `
Usage:
	redis-port decode   [--ncpu=N]  [--input=INPUT]  [--output=OUTPUT]
	redis-port restore  [--ncpu=N]  [--input=INPUT]   --target=TARGET  [--extra] [--faketime=FAKETIME] [--filterdb=DB]
	redis-port dump     [--ncpu=N]   --from=MASTER   [--output=OUTPUT] [--extra]
	redis-port sync     [--ncpu=N]   --from=MASTER    --target=TARGET  [--sockfile=FILE [--filesize=SIZE]] [--filterdb=DB]

Options:
	-n N, --ncpu=N                    Set runtime.GOMAXPROCS to N.
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
		utils.ErrorPanic(err, "parse arguments failed")
	}
	args.ncpu = runtime.GOMAXPROCS(0)

	if s, ok := d["--ncpu"].(string); ok && len(s) != 0 {
		max := runtime.NumCPU()
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			utils.ErrorPanic(err, "parse --ncpu failed")
		}
		if n <= 0 || n > int64(max) {
			utils.Panic("parse --ncpu = %d, only accept [1,%d]", n, max)
		}
		args.ncpu = int(n)
		runtime.GOMAXPROCS(args.ncpu)
	}
	if args.ncpu == 0 {
		args.ncpu = runtime.GOMAXPROCS(0)
	}

	args.input, _ = d["--input"].(string)
	args.output, _ = d["--output"].(string)

	args.target, _ = d["--target"].(string)
	args.from, _ = d["--from"].(string)

	args.extra, _ = d["--extra"].(bool)
	args.sockfile, _ = d["--sockfile"].(string)

	if s, ok := d["--faketime"].(string); ok && s != "" {
		switch s[0] {
		case '-', '+':
			d, err := time.ParseDuration(strings.ToLower(s))
			if err != nil {
				utils.ErrorPanic(err, "parse --faketime failed")
			}
			args.shift = d
		case '@':
			n, err := strconv.ParseInt(s[1:], 10, 64)
			if err != nil {
				utils.ErrorPanic(err, "parse --faketime failed")
			}
			args.shift = time.Duration(n*int64(time.Millisecond) - time.Now().UnixNano())
		default:
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				utils.ErrorPanic(err, "parse --faketime failed")
			}
			args.shift = time.Duration(t.UnixNano() - time.Now().UnixNano())
		}
	}

	acceptDB = func(db int64) bool {
		return true
	}
	if s, ok := d["--filterdb"].(string); ok && s != "" {
		if s != "*" {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				utils.ErrorPanic(err, "parse --filterdb failed")
			}
			const max int64 = 1024
			if n < 0 || n > max {
				utils.Panic("parse --filterdb = %d, only accpet [0,%d]", n, max)
			}
			acceptDB = func(db int64) bool {
				return db == n
			}
		}
	}

	if s, ok := d["--filesize"].(string); ok && s != "" {
		if len(args.sockfile) == 0 {
			utils.Panic("please specify --sockfile first")
		}
		n, err := bytesize.Parse(s)
		if err != nil {
			utils.ErrorPanic(err, "parse --filesize failed")
		}
		if n <= 0 {
			utils.Panic("parse --filesize = %d, invalid number", n)
		}
		args.filesize = n
	} else {
		args.filesize = bytesize.GB
	}

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
