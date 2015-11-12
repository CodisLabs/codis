// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"github.com/docopt/docopt-go"

	"github.com/wandoulabs/codis/pkg/utils/log"
)

func main() {
	const usage = `
Usage:
	codis-admin [-v]  --proxy-admin=ADDR [--product-name=NAME [--product-auth=AUTH]]              [overview|config|model|stats|slots]
	codis-admin [-v]  --proxy-admin=ADDR  --product-name=NAME [--product-auth=AUTH]                shutdown
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]]) [overview|config|model|stats|slots]
	codis-admin [-v] (--config=CONF|--dashboard=ADDR  --product-name=NAME [--product-auth=AUTH] )  shutdown
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]]) (proxy|group) [--list|--stats-map]
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  proxy  --create  --addr=ADDR
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  proxy  --remove (--addr=ADDR|--token=TOKEN|--proxy-id=ID) [--force]
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  proxy  --reinit (--addr=ADDR|--token=TOKEN|--proxy-id=ID)
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  proxy  --xpingall
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  group  --create  --group-id=ID
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  group  --remove  --group-id=ID
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  group            --group-id=ID --add            --addr=ADDR
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  group            --group-id=ID --del           (--addr=ADDR|--index=INDEX)
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  group            --group-id=ID --promote       (--addr=ADDR|--index=INDEX)
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  group            --group-id=ID --promote-commit
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  group                          --master-status
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  group            --group-id=ID --master-repair (--addr=ADDR|--index=INDEX)
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  action --create        --group-id=ID --slot-id=ID
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  action --remove                      --slot-id=ID
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  action --create-range  --group-id=ID --slot-beg=ID --slot-end=ID
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  action --set --interval=VALUE
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME [--product-auth=AUTH]])  action --set --disabled=VALUE
	codis-admin [-v]  --remove-lock     --product-name=NAME (--zookeeper=ADDR|--etcd=ADDR)
	codis-admin [-v]  --config-dump     --product-name=NAME (--zookeeper=ADDR|--etcd=ADDR) [-1|-2]
	codis-admin [-v]  --config-convert  --input=FILE
	codis-admin [-v]  --config-restore  --input=FILE --product-name=NAME (--zookeeper=ADDR|--etcd=ADDR)

Options:
	-c CONF, --config=CONF
	-n NAME, --product-name=NAME
	-a AUTH, --product-auth=AUTH
	-x ADDR, --addr=ADDR
	-t TOKEN, --token=TOKEN
	-i INDEX, --index=INDEX
	-p ID, --proxy-id=ID
	-g ID, --group-id=ID
	-s ID, --slot-id=ID
`

	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}
	log.SetLevel(log.LEVEL_INFO)

	if d["-v"].(bool) {
		log.SetLevel(log.LEVEL_DEBUG)
	} else {
		log.SetLevel(log.LEVEL_INFO)
	}

	switch {
	case d["--proxy-admin"] != nil:
		new(cmdProxy).Main(d)
	case d["--dashboard"] != nil || d["--config"] != nil:
		new(cmdDashboard).Main(d)
	default:
		new(cmdAdmin).Main(d)
	}
}
