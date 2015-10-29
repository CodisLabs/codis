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
	codis-admin [-v]  --proxy=ADDR                                           [overview|config|model|stats|slots]
	codis-admin [-v]  --proxy=ADDR --product-name=NAME [--product-auth=AUTH]  shutdown
	codis-admin [-v] (--config=CONF|--dashboard=ADDR)                                             [overview|config|model|stats|slots]
	codis-admin [-v] (--config=CONF|--dashboard=ADDR  --product-name=NAME  [--product-auth=AUTH])  shutdown
	codis-admin [-v] (--config=CONF|--dashboard=ADDR [--product-name=NAME] [--product-auth=AUTH]) (proxy|group) [--list|--stats-map]



	codis-admin [-v] --dashboard=ADDR  proxy --create  --addr=ADDR                                  [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  proxy --remove (--addr=ADDR|--token=TOKEN|--proxy-id=ID) [--force] [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  proxy --reinit (--addr=ADDR|--token=TOKEN|--proxy-id=ID)           [--product-name=NAME] [--product-auth=AUTH]

	codis-admin [-v] --dashboard=ADDR  group --create=GID   [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  group --remove=GID   [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  server --status=GID  [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  server --repair=GID  [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  server --group=GID --add=ADDR [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  server --group=GID --del=ADDR [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  server --group=GID (--promote=ADDR|--commit) [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  action --create --slot=SID --group=GID [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  action --remove --slot=SID             [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  action --set-interval=VALUE            [--product-name=NAME] [--product-auth=AUTH]
	codis-admin [-v] --dashboard=ADDR  action --set-disabled=FLAG             [--product-name=NAME] [--product-auth=AUTH]

Options:
	-c CONF, --config=CONF
	-n NAME, --product-name=NAME
	-x ADDR, --addr=ADDR
	-l, --list
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
	case d["--proxy"] != nil:
		new(cmdProxy).Main(d)
	case d["--dashboard"] != nil || d["--config"] != nil:
		new(cmdDashboard).Main(d)
	}
}
