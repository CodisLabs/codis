// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/c4pt0r/cfg"
	"github.com/docopt/docopt-go"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils"
)

// global objects
var (
	globalEnv            Env
	livingNode           string
	createdDashboardNode bool
)

type Command struct {
	Run   func(cmd *Command, args []string)
	Usage string
	Short string
	Long  string
	Flag  flag.FlagSet
	Ctx   interface{}
}

var usage = `usage: codis-config  [-c <config_file>] [-L <log_file>] [--log-level=<loglevel>]
		<command> [<args>...]
options:
   -c	set config file
   -L	set output log file, default is stdout
   --log-level=<loglevel>	set log level: info, warn, error, debug [default: info]

commands:
	server
	slot
	dashboard
	action
	proxy
`

func init() {
	log.SetLevel(log.LEVEL_INFO)
}

func setLogLevel(level string) {
	var lv = log.LEVEL_INFO
	switch strings.ToLower(level) {
	case "error":
		lv = log.LEVEL_ERROR
	case "warn", "warning":
		lv = log.LEVEL_WARN
	case "debug":
		lv = log.LEVEL_DEBUG
	case "info":
		fallthrough
	default:
		lv = log.LEVEL_INFO
	}
	log.SetLevel(lv)
	log.Infof("set log level to %s", lv)
}

func runCommand(cmd string, args []string) (err error) {
	argv := make([]string, 1)
	argv[0] = cmd
	argv = append(argv, args...)
	switch cmd {
	case "action":
		return errors.Trace(cmdAction(argv))
	case "dashboard":
		return errors.Trace(cmdDashboard(argv))
	case "server":
		return errors.Trace(cmdServer(argv))
	case "proxy":
		return errors.Trace(cmdProxy(argv))
	case "slot":
		return errors.Trace(cmdSlot(argv))
	}
	return errors.Errorf("%s is not a valid command. See 'codis-config -h'", cmd)
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		if createdDashboardNode {
			releaseDashboardNode()
		}
		log.Panicf("ctrl-c or SIGTERM found, exit")
	}()

	args, err := docopt.Parse(usage, nil, true, utils.Version, true)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// set output log file
	if s, ok := args["-L"].(string); ok && s != "" {
		f, err := os.OpenFile(s, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			log.PanicErrorf(err, "open log file failed: %s", s)
		} else {
			defer f.Close()
			log.StdLog = log.New(f, "")
		}
	}
	log.SetLevel(log.LEVEL_INFO)
	log.SetFlags(log.Flags() | log.Lshortfile)

	// set log level
	if s, ok := args["--log-level"].(string); ok && s != "" {
		setLogLevel(s)
	}

	// set config file
	var configFile string
	if args["-c"] != nil {
		configFile = args["-c"].(string)
	} else {
		configFile = "config.ini"
	}
	config := cfg.NewCfg(configFile)

	if err := config.Load(); err != nil {
		log.PanicErrorf(err, "load config file error")
	}

	// load global vars
	globalEnv = LoadCodisEnv(config)

	cmd := args["<command>"].(string)
	cmdArgs := args["<args>"].([]string)

	go http.ListenAndServe(":10086", nil)
	err = runCommand(cmd, cmdArgs)
	if err != nil {
		log.PanicErrorf(err, "run sub-command failed")
	}
}
