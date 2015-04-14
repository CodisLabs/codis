// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/c4pt0r/cfg"
	"github.com/wandoulabs/codis/pkg/env"
	"github.com/wandoulabs/codis/pkg/utils"

	"net/http"
	_ "net/http/pprof"

	docopt "github.com/docopt/docopt-go"
	"github.com/juju/errors"
	log "github.com/ngaut/logging"
)

// global objects
var (
	globalEnv  env.Env
	livingNode string
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

func Fatal(msg interface{}) {
	// cleanup
	releaseDashboardNode()
	if globalMigrateManager != nil {
		globalMigrateManager.removeNode()
	}

	switch msg.(type) {
	case string:
		log.Fatal(msg)
	case error:
		log.Fatal(errors.ErrorStack(msg.(error)))
	}
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
	log.SetLevelByString("info")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		Fatal("ctrl-c or SIGTERM found, exit")
	}()

	args, err := docopt.Parse(usage, nil, true, "codis config v0.1", true)
	if err != nil {
		log.Error(err)
	}

	// set config file
	var configFile string
	var config *cfg.Cfg
	if args["-c"] != nil {
		configFile = args["-c"].(string)
		config, err = utils.InitConfigFromFile(configFile)
		if err != nil {
			Fatal(err)
		}
	} else {
		config, err = utils.InitConfig()
		if err != nil {
			Fatal(err)
		}
	}

	// load global vars
	globalEnv = env.LoadCodisEnv(config)

	// set output log file
	if args["-L"] != nil {
		log.SetOutputByName(args["-L"].(string))
	}

	// set log level
	if args["--log-level"] != nil {
		log.SetLevelByString(args["--log-level"].(string))
	}

	cmd := args["<command>"].(string)
	cmdArgs := args["<args>"].([]string)

	go http.ListenAndServe(globalEnv.DashboardAddr(), nil)
	err = runCommand(cmd, cmdArgs)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
}
