// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/models/store/etcd"
	"github.com/wandoulabs/codis/pkg/models/store/zk"
	"github.com/wandoulabs/codis/pkg/topom"
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
	codis-dashboard [--ncpu=N] [--config=CONF] [--log=FILE] [--log-level=LEVEL] (--zookeeper=ADDR|--etcd=ADDR)
	codis-dashboard version

Options:
	--ncpu=N                    set runtime.GOMAXPROCS to N, default is runtime.NumCPU().
	-c CONF, --config=CONF      specify the config file.
	-l FILE, --log=FILE         specify the daliy rotated log file.
	--log-level=LEVEL           specify the log-level, can be INFO,WARN,DEBUG,ERROR, default is INFO.
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

	config := topom.NewDefaultConfig()
	if s, ok := d["--config"].(string); ok && s != "" {
		if err := config.LoadFromFile(s); err != nil {
			log.PanicErrorf(err, "load config failed, file = '%s'", s)
		}
	}

	if !utils.IsValidName(config.ProductName) {
		log.Panicf("invalid product name")
	}

	var store models.Store
	switch {
	case d["--zookeeper"] != nil:
		store, err = zkstore.NewStore(d["--zookeeper"].(string), config.ProductName)
		if err != nil {
			log.PanicErrorf(err, "create zkstore failed")
		}
	case d["--etcd"] != nil:
		store, err = etcdstore.NewStore(d["--etcd"].(string), config.ProductName)
		if err != nil {
			log.PanicErrorf(err, "create etcdstore failed")
		}
	}

	if store == nil {
		log.Panicf("nil store for topom")
	}
	defer store.Close()

	s, err := topom.New(store, config)
	if err != nil {
		log.PanicErrorf(err, "create topom with config file failed\n%s\n", config)
	}
	defer s.Close()

	s.StartDaemonRoutines()

	log.Infof("create topom with config\n%s\n", config)

	go func() {
		defer s.Close()
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)

		sig := <-c
		log.Infof("[%p] dashboard receive signal = '%v'", s, sig)
	}()

	for !s.IsClosed() {
		time.Sleep(time.Second)
	}

	log.Infof("[%p] topom exiting ...", s)
}
