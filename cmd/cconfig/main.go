// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/ngaut/go-zookeeper/zk"
	"github.com/ngaut/zkhelper"
	"github.com/wandoulabs/codis/pkg/utils"

	"net/http"
	_ "net/http/pprof"

	"github.com/c4pt0r/cfg"
	docopt "github.com/docopt/docopt-go"
	"github.com/juju/errors"
	log "github.com/ngaut/logging"
)

// global objects
var (
	zkConn      zkhelper.Conn
	zkAddr      string
	productName string
	configFile  string
	config      *cfg.Cfg
	zkLock      zkhelper.ZLocker
	livingNode  string
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
	unRegisterConfigNode()
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

func removeOrphanLocks() error {
	nodeDir := fmt.Sprintf("/zk/codis/db_%s/living-codis-config", productName)
	lockDir := fmt.Sprintf("/zk/codis/db_%s/LOCK", productName)

	livingCfgNodes, _, err := zkConn.Children(nodeDir)
	if err != nil {
		return errors.Trace(err)
	}

	// get living nodes
	nodeMap := make(map[string]struct{})
	for _, nodeName := range livingCfgNodes {
		nodeMap[nodeName] = struct{}{}
	}

	// get living lock
	livingLocks, _, err := zkConn.Children(lockDir)
	for _, lockName := range livingLocks {
		data, _, err := zkConn.Get(path.Join(lockDir, lockName))
		if err != nil {
			return errors.Trace(err)
		}

		// get lock info
		var d map[string]interface{}
		err = json.Unmarshal(data, &d)
		if err != nil {
			return errors.Trace(err)
		}

		nodeName := fmt.Sprintf("%v-%v", d["hostname"], d["pid"])
		// remove the locks that no one owns
		if _, ok := nodeMap[nodeName]; !ok {
			log.Info("remove orphan lock", lockName)
			zkConn.Delete(path.Join(lockDir, lockName), 0)
		}
	}
	return nil
}

func registerConfigNode() error {
	zkPath := fmt.Sprintf("/zk/codis/db_%s/living-codis-config", productName)

	hostname, err := os.Hostname()
	if err != nil {
		return errors.Trace(err)
	}
	pid := os.Getpid()

	content := fmt.Sprintf(`{"hostname": "%v", "pid": %v}`, hostname, pid)
	nodeName := fmt.Sprintf("%v-%v", hostname, pid)

	zkhelper.CreateRecursive(zkConn, zkPath, "", 0, zkhelper.DefaultDirACLs())

	pathCreated, err := zkConn.Create(path.Join(zkPath, nodeName), []byte(content),
		zk.FlagEphemeral, zkhelper.DefaultDirACLs())

	log.Info("living node created:", pathCreated)

	if err != nil {
		return errors.Trace(err)
	}

	livingNode = pathCreated

	return nil
}

func unRegisterConfigNode() {
	if len(livingNode) > 0 {
		if exists, _, _ := zkConn.Exists(livingNode); exists {
			zkConn.Delete(livingNode, 0)
		}
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
	return fmt.Errorf("%s is not a valid command. See 'codis-config -h'", cmd)
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
	zkConn, _ = zkhelper.ConnectToZk(zkAddr)
	zkLock = utils.GetZkLock(zkConn, productName)

	log.Debugf("product: %s", productName)
	log.Debugf("zk: %s", zkAddr)

	if err := registerConfigNode(); err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
	defer unRegisterConfigNode()

	if err := removeOrphanLocks(); err != nil {
		log.Fatal(errors.ErrorStack(err))
	}

	cmd := args["<command>"].(string)
	cmdArgs := args["<args>"].([]string)

	go http.ListenAndServe(":10086", nil)
	err = runCommand(cmd, cmdArgs)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
}
