// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"
	"strconv"

	"github.com/juju/errors"

	docopt "github.com/docopt/docopt-go"
	log "github.com/ngaut/logging"
)

func cmdAction(argv []string) (err error) {
	usage := `usage: codis-config action (gc [-n <num> | -s <seconds>] | remove-lock | remove-fence)

options:
	gc:
	gc -n N		keep last N actions;
	gc -s Sec	keep last Sec seconds actions;

	remove-lock	force remove zookeeper lock;
`
	args, err := docopt.Parse(usage, argv, true, "", false)
	if err != nil {
		log.Error(err)
		return errors.Trace(err)
	}

	if args["remove-lock"].(bool) {
		return errors.Trace(runRemoveLock())
	}

	if args["remove-fence"].(bool) {
		return errors.Trace(runRemoveFence())
	}

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
				return errors.Trace(err)
			}
			return runGCKeepNSec(sec)
		}
	}

	return nil
}

func runRemoveFence() error {
	var v interface{}
	if err := callApi(METHOD_GET, "/api/remove_fence", nil, &v); err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}

func runGCKeepN(keep int) error {
	var v interface{}
	if err := callApi(METHOD_GET, fmt.Sprintf("/api/action/gc?keep=%d", keep), nil, &v); err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}

func runGCKeepNSec(secs int) error {
	var v interface{}
	if err := callApi(METHOD_GET, fmt.Sprintf("/api/action/gc?secs=%d", secs), nil, &v); err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}

func runRemoveLock() error {
	var v interface{}
	if err := callApi(METHOD_GET, "/api/force_remove_locks", nil, &v); err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil

}
