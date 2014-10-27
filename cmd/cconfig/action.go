package main

import (
	"codis/pkg/models"
	"fmt"
	"strconv"

	docopt "github.com/docopt/docopt-go"
	log "github.com/ngaut/logging"
)

func cmdAction(argv []string) (err error) {
	usage := `usage: cconfig action (gc [-n <num> | -s <seconds>] | remove-lock)

options:
	gc:
	gc -n N		keep last N actions;
	gc -s Sec	keep last Sec seconds actions;

	remove-lock	force remove zookeeper lock;
`
	args, err := docopt.Parse(usage, argv, true, "", false)
	if err != nil {
		log.Error(err)
		return err
	}

	if args["remove-lock"].(bool) {
		return runRemoveLock()
	}

	zkLock.Lock(fmt.Sprintf("action, %+v", argv))
	defer func() {
		err := zkLock.Unlock()
		if err != nil {
			log.Info(err)
		}
	}()

	if args["gc"].(bool) {
		if args["-n"].(bool) {
			n, err := strconv.Atoi(args["<num>"].(string))
			if err != nil {
				log.Warning(err)
				return err
			}
			return runGCKeepN(n)
		} else if args["-s"].(bool) {
			sec, err := strconv.Atoi(args["<seconds>"].(string))
			if err != nil {
				log.Warning(err)
				return err
			}
			return runGCKeepNSec(sec)
		}
	}

	return nil
}

func runGCKeepN(keep int) error {
	log.Info("gc...")
	return models.ActionGC(zkConn, productName, models.GC_TYPE_N, keep)
}

func runGCKeepNSec(secs int) error {
	log.Info("gc...")
	return models.ActionGC(zkConn, productName, models.GC_TYPE_SEC, secs)
}

func runRemoveLock() error {
	log.Info("removing lock...")
	zkLock.Unlock()
	return models.ForceRemoveLock(zkConn, productName)
}
