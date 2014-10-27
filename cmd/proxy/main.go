package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"

	"github.com/wandoulabs/codis/pkg/proxy/router"

	"github.com/docopt/docopt-go"
	log "github.com/ngaut/logging"
)

var (
	cpus       = 2
	addr       = ":9000"
	httpAddr   = ":9001"
	configFile = "config.ini"
)

var usage = `usage: proxy [-c <config_file>] [-L <log_file>] [--log-level=<loglevel>] [--cpu=<cpu_num>] [--addr=<proxy_listen_addr>] [--http-addr=<debug_http_server_addr>]

options:
   -c	set config file
   -L	set output log file, default is stdout
   --log-level=<loglevel>	set log level: info, warn, error, debug [default: info]
   --cpu=<cpu_num>		blabla
   --addr=<proxy_listen_addr>		blabla
   --http-addr=<debug_http_server_addr>		blabla
`

var banner string = `
  _____  ____    ____/ /  (_)  _____
 / ___/ / __ \  / __  /  / /  / ___/
/ /__  / /_/ / / /_/ /  / /  (__  )
\___/  \____/  \__,_/  /_/  /____/

`

func handleSetLogLevel(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	level := r.Form.Get("level")
	log.SetLevelByString(level)
	log.Info("set log level to", level)
}

func main() {
	fmt.Print(banner)
	log.SetLevelByString("info")

	args, err := docopt.Parse(usage, nil, true, "codis proxy v0.1", true)
	if err != nil {
		log.Error(err)
	}

	// set config file
	if args["-c"] != nil {
		configFile = args["-c"].(string)
	}

	// set output log file
	if args["-L"] != nil {
		log.SetOutputByName(args["-L"].(string))
	}

	// set log level
	if args["--log-level"] != nil {
		log.SetLevelByString(args["--log-level"].(string))
	}

	// set cpu
	if args["--cpu"] != nil {
		cpus, err = strconv.Atoi(args["--cpu"].(string))
		if err != nil {
			log.Fatal(err)
		}
	}

	// set addr
	if args["--addr"] != nil {
		addr = args["--addr"].(string)
	}

	// set http addr
	if args["--http-addr"] != nil {
		httpAddr = args["--http-addr"].(string)
	}

	wd, _ := os.Getwd()
	log.Info("wd:", wd)
	log.CrashLog(wd + ".dump")

	router.CheckUlimit(1024)
	runtime.GOMAXPROCS(cpus)

	http.HandleFunc("/setloglevel", handleSetLogLevel)
	go http.ListenAndServe(httpAddr, nil)
	log.Info("running on ", addr)
	s := router.NewServer(addr, httpAddr, configFile)
	s.Run()
	log.Warning("exit")
}
