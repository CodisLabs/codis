// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package args

import (
	"runtime"
	"strconv"
)

import (
	"github.com/docopt/docopt-go"
	"github.com/wandoulabs/codis/ext/redis-port/utils"
)

var (
	ncpu int
	args map[string]interface{}
	code string
)

func invArg(name string) {
	utils.Panic("please specify argument `%s' correctly", name)
}

func strArg(name string, nonil bool) string {
	if v := args[name]; v != nil {
		if s, ok := v.(string); !ok {
			utils.Panic("argument `%s' is not a string", name)
		} else if len(s) != 0 {
			return s
		}
	}
	if nonil {
		invArg(name)
	}
	return ""
}

func init() {
	usage := `
Usage:
	redis-port decode   [--ncpu=N]  [--input=INPUT]  [--output=OUTPUT]
	redis-port restore  [--ncpu=N]  [--input=INPUT]   --target=TARGET
	redis-port dump     [--ncpu=N]   --from=MASTER   [--output=OUTPUT]
	redis-port sync     [--ncpu=N]   --from=MASTER    --target=TARGET

Options:
	-n N, --ncpu=N                    Set runtime.GOMAXPROCS to N.
	-i INPUT, --input=INPUT           Set input file, default is stdin ('/dev/stdin').
	-o OUTPUT, --output=OUTPUT        Set output file, default is stdout ('/dev/stdout').
	-f MASTER, --from=MASTER          Set master redis.
	-t TARGET, --target=TARGET        Set slave redis.
`
	var err error
	args, err = docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		utils.Panic("parse usage error = '%s'", err)
	}
	ncpu = runtime.GOMAXPROCS(0)

	if s := strArg("--ncpu", false); len(s) != 0 {
		if n, err := strconv.ParseInt(s, 10, 64); err != nil {
			utils.Panic("parse --ncpu = %v, error = '%s'", s, err)
		} else if n <= 0 || n > 64 {
			utils.Panic("parse --ncpu = %d, only accept [1,64]", n)
		} else {
			ncpu = int(n)
			runtime.GOMAXPROCS(ncpu)
		}
	}
	if ncpu == 0 {
		ncpu = 1
	}

	for _, s := range []string{"decode", "restore", "dump", "sync"} {
		if args[s].(bool) {
			code = s
			return
		}
	}
	utils.Panic("parse command code error")
}

func NCPU() int {
	return ncpu
}

func Code() string {
	return code
}

func Input() string {
	if s := strArg("--input", false); len(s) != 0 {
		return s
	} else {
		return "/dev/stdin"
	}
}

func Output() string {
	if s := strArg("--output", false); len(s) != 0 {
		return s
	} else {
		return "/dev/stdout"
	}
}

func From() string {
	return strArg("--from", true)
}

func Target() string {
	return strArg("--target", true)
}
