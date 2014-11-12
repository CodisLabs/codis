// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/wandoulabs/codis/pkg/models"

	"github.com/docopt/docopt-go"
	log "github.com/ngaut/logging"
)

func cmdProxy(argv []string) (err error) {
	usage := `usage:
	cconfig proxy list
	cconfig proxy offline <proxy_name>
	cconfig proxy online <proxy_name>
`
	args, err := docopt.Parse(usage, argv, true, "", false)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debug(args)

	zkLock.Lock(fmt.Sprintf("proxy, %+v", argv))
	defer func() {
		err := zkLock.Unlock()
		if err != nil {
			log.Error(err)
		}
	}()

	if args["list"].(bool) {
		log.Warning(err)
		return runProxyList()
	}

	proxyName := args["<proxy_name>"].(string)
	if args["online"].(bool) {
		return runSetProxyStatus(proxyName, models.PROXY_STATE_ONLINE)
	}
	if args["offline"].(bool) {
		return runSetProxyStatus(proxyName, models.PROXY_STATE_MARK_OFFLINE)
	}
	return nil
}

func runProxyList() error {
	proxies, err := models.ProxyList(zkConn, productName, nil)
	if err != nil {
		log.Warning(err)
		return err
	}
	b, _ := json.MarshalIndent(proxies, " ", "  ")
	fmt.Println(string(b))
	return nil
}

func runSetProxyStatus(proxyName, status string) error {
	if err := models.SetProxyStatus(zkConn, productName, proxyName, status); err != nil {
		log.Warning(err)
		return err
	}
	return nil
}
