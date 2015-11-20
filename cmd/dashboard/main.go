// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/models/etcd"
	"github.com/wandoulabs/codis/pkg/models/zk"
	"github.com/wandoulabs/codis/pkg/topom"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func main() {
	const usage = `
Usage:
	codis-dashboard [--ncpu=N] [--config=CONF] [--log=FILE] [--log-level=LEVEL] (--zookeeper=ADDR|--etcd=ADDR) [--host-admin=ADDR]
	codis-dashboard  --version

Options:
	--ncpu=N                    set runtime.GOMAXPROCS to N, default is runtime.NumCPU().
	-c CONF, --config=CONF      run with the specific configuration.
	-l FILE, --log=FILE         set path/name of daliy rotated log file.
	--log-level=LEVEL           set the log-level, should be INFO,WARN,DEBUG or ERROR, default is INFO.
`

	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}

	if d["--version"].(bool) {
		fmt.Println("version:", utils.Version)
		fmt.Println("compile:", utils.Compile)
		return
	}

	if s, ok := utils.Argument(d, "--log"); ok {
		w, err := log.NewRollingFile(s, log.DailyRolling)
		if err != nil {
			log.PanicErrorf(err, "open log file %s failed", s)
		} else {
			log.StdLog = log.New(w, "")
		}
	}
	log.SetLevel(log.LEVEL_INFO)

	if s, ok := utils.Argument(d, "--log-level"); ok {
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
			log.Panicf("invalid option --log-level = '%s'", level)
		}
	}

	if n, ok := utils.ArgumentInteger(d, "--ncpu"); ok {
		runtime.GOMAXPROCS(n)
	} else {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	log.Infof("set ncpu = %d", runtime.GOMAXPROCS(0))

	config := topom.NewDefaultConfig()
	if s, ok := utils.Argument(d, "--config"); ok {
		if err := config.LoadFromFile(s); err != nil {
			log.PanicErrorf(err, "load config %s failed", s)
		}
	}
	if s, ok := utils.Argument(d, "--host-admin"); ok {
		config.HostAdmin = s
		log.Infof("option --host-admin = %s", s)
	}

	var client models.Client

	switch {
	case d["--zookeeper"] != nil:
		addr := utils.ArgumentMust(d, "--zookeeper")
		client, err = zkclient.New(addr, time.Minute)
		if err != nil {
			log.PanicErrorf(err, "create zk client to %s failed", addr)
		}
		defer client.Close()

	case d["--etcd"] != nil:
		addr := utils.ArgumentMust(d, "--etcd")
		client, err = etcdclient.New(addr, time.Minute)
		if err != nil {
			log.PanicErrorf(err, "create etcd client to %s failed", addr)
		}
		defer client.Close()

	default:
		log.Panicf("nil client for topom")
	}

	s, err := topom.New(client, config)
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
