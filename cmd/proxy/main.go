// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"

	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

const banner = `
  _____  ____    ____/ /  (_)  _____
 / ___/ / __ \  / __  /  / /  / ___/
/ /__  / /_/ / / /_/ /  / /  (__  )
\___/  \____/  \__,_/  /_/  /____/

`

func main() {
	const usage = `
Usage:
	codis-proxy [--ncpu=N] [--config=CONF] [--log=FILE] [--log-level=LEVEL] [--ulimit=NLIMIT]
	codis-proxy version

Options:
	--ncpu=N                    set runtime.GOMAXPROCS to N, default is runtime.NumCPU().
	-c CONF, --config=CONF      specify the config file.
	-l FILE, --log=FILE         specify the daliy rotated log file.
	--log-level=LEVEL           specify the log-level, can be INFO,WARN,DEBUG,ERROR, default is INFO.
	--ulimit=NLIMIT             run 'ulimit -n' to check the maximum number of open file descriptors.
`

	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}

	if v, ok := d["version"].(bool); ok && v {
		fmt.Println("version:", utils.Version)
		fmt.Println("compile:", utils.Compile)
		return
	}

	if s, ok := d["--ulimit"].(string); ok && s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			log.PanicErrorf(err, "parse argument of ulimit failed")
		}
		b, err := exec.Command("/bin/sh", "-c", "ulimit -n").Output()
		if err != nil {
			log.PanicErrorf(err, "run ulimit -n failed")
		}
		if v, err := strconv.Atoi(strings.TrimSpace(string(b))); err != nil || v < n {
			log.PanicErrorf(err, "ulimit too small: %d, should be at least %d", v, n)
		}
	}

	if s, ok := d["--log"].(string); ok && s != "" {
		w, err := log.NewRollingFile(s, log.DailyRolling)
		if err != nil {
			log.PanicErrorf(err, "open log file %s failed", s)
		} else {
			log.StdLog = log.New(w, "")
		}
	}
	log.SetLevel(log.LEVEL_INFO)

	fmt.Println(banner)

	ncpu := runtime.NumCPU()
	if s, ok := d["--ncpu"].(string); ok && s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			log.PanicErrorf(err, "parse --ncpu failed, invalid ncpu = '%s'", s)
		}
		ncpu = n
	}
	runtime.GOMAXPROCS(ncpu)
	log.Infof("set ncpu = %d", ncpu)

	if s, ok := d["--log-level"].(string); ok && s != "" {
		var level = strings.ToUpper(s)
		switch s {
		case "ERROR":
			log.SetLevel(log.LEVEL_ERROR)
		case "DEBUG":
			log.SetLevel(log.LEVEL_DEBUG)
		case "WARN", "WARNING":
			log.SetLevel(log.LEVEL_WARN)
		case "INFO":
			log.SetLevel(log.LEVEL_INFO)
		default:
			log.Panicf("parse --log-level failed, invalid level = '%s'", level)
		}
	}

	config := proxy.NewDefaultConfig()
	if s, ok := d["--config"].(string); ok && s != "" {
		if err := config.LoadFromFile(s); err != nil {
			log.PanicErrorf(err, "load config failed, file = '%s'", s)
		}
	}

	s, err := proxy.New(config)
	if err != nil {
		log.PanicErrorf(err, "create proxy with config file failed\n%s\n", config)
	}
	defer s.Close()

	log.Infof("create proxy with config\n%s\n", config)

	for !s.IsClosed() && !s.IsOnline() {
		log.Infof("[%p] proxy waiting online ...", s)
		time.Sleep(time.Second)
	}

	for !s.IsClosed() {
		time.Sleep(time.Second)
	}

	log.Infof("[%p] proxy exiting ...", s)
}
