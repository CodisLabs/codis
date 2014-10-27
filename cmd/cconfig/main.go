package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"codis/pkg/utils"
	"codis/pkg/zkhelper"

	"net/http"
	_ "net/http/pprof"

	"github.com/c4pt0r/cfg"
	docopt "github.com/docopt/docopt-go"
	"github.com/juju/errors"
	log "github.com/ngaut/logging"
)

// global objects
var zkConn zkhelper.Conn
var zkAddr string
var productName string
var configFile string
var config *cfg.Cfg
var zkLock zkhelper.ZLocker

type Command struct {
	Run   func(cmd *Command, args []string)
	Usage string
	Short string
	Long  string
	Flag  flag.FlagSet
	Ctx   interface{}
}

var usage = `usage: cconfig  [-c <config_file>] [-L <log_file>] [--log-level=<loglevel>]
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
	// try unlock force
	if zkLock != nil {
		zkLock.Unlock()
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
		return cmdAction(argv)
	case "dashboard":
		return cmdDashboard(argv)
	case "server":
		return cmdServer(argv)
	case "proxy":
		return cmdProxy(argv)
	case "slot":
		return cmdSlot(argv)
	}
	return fmt.Errorf("%s is not a valid command. See 'cconfig -h'", cmd)
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

	//	productName, _ = config.ReadString("product", "test")
	args, err := docopt.Parse(usage, nil, true, "codis config v0.1", true)
	if err != nil {
		log.Error(err)
	}

	// set config file
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

	// set output log file
	if args["-L"] != nil {
		log.SetOutputByName(args["-L"].(string))
	}

	// set log level
	if args["--log-level"] != nil {
		log.SetLevelByString(args["--log-level"].(string))
	}

	productName, _ = config.ReadString("product", "test")
	zkAddr, _ = config.ReadString("zk", "localhost:2181")
	zkConn, _ = zkhelper.InitZk(zkAddr)
	zkLock = utils.GetZkLock(zkConn, productName)

	log.Debugf("product: %s", productName)
	log.Debugf("zk: %s", zkAddr)

	cmd := args["<command>"].(string)
	cmdArgs := args["<args>"].([]string)

	go http.ListenAndServe(":10086", nil)
	err = runCommand(cmd, cmdArgs)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
