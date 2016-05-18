// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"github.com/docopt/docopt-go"

	"github.com/CodisLabs/codis/pkg/utils/log"
)

func main() {
	const usage = `
Usage:
	codis-admin [-v] --proxy=ADDR [--auth=AUTH] [config|model|stats|slots]
	codis-admin [-v] --proxy=ADDR [--auth=AUTH]  --start
	codis-admin [-v] --proxy=ADDR [--auth=AUTH]  --shutdown
	codis-admin [-v] --proxy=ADDR [--auth=AUTH]  --log-level=LEVEL
	codis-admin [-v] --proxy=ADDR [--auth=AUTH]  --fillslots=FILE [--locked]
	codis-admin [-v] --dashboard=ADDR           [config|model|stats|slots|group|proxy]
	codis-admin [-v] --dashboard=ADDR            --shutdown
	codis-admin [-v] --dashboard=ADDR            --log-level=LEVEL
	codis-admin [-v] --dashboard=ADDR            --slots-remap=FILE [--confirm]
	codis-admin [-v] --dashboard=ADDR            --slots-status
	codis-admin [-v] --dashboard=ADDR            --list-proxy
	codis-admin [-v] --dashboard=ADDR            --create-proxy   --addr=ADDR
	codis-admin [-v] --dashboard=ADDR            --remove-proxy  (--addr=ADDR|--token=TOKEN|--pid=ID) [--force]
	codis-admin [-v] --dashboard=ADDR            --reinit-proxy  (--addr=ADDR|--token=TOKEN|--pid=ID|--all)
	codis-admin [-v] --dashboard=ADDR            --proxy-status
	codis-admin [-v] --dashboard=ADDR            --list-group
	codis-admin [-v] --dashboard=ADDR            --create-group   --gid=ID
	codis-admin [-v] --dashboard=ADDR            --remove-group   --gid=ID
	codis-admin [-v] --dashboard=ADDR            --group-add      --gid=ID --addr=ADDR
	codis-admin [-v] --dashboard=ADDR            --group-del      --gid=ID --addr=ADDR
	codis-admin [-v] --dashboard=ADDR            --group-status
	codis-admin [-v] --dashboard=ADDR            --promote-server --gid=ID --addr=ADDR
	codis-admin [-v] --dashboard=ADDR            --promote-commit --gid=ID
	codis-admin [-v] --dashboard=ADDR            --sync-action    --create --addr=ADDR
	codis-admin [-v] --dashboard=ADDR            --sync-action    --remove --addr=ADDR
	codis-admin [-v] --dashboard=ADDR            --slot-action    --create --sid=ID --gid=ID
	codis-admin [-v] --dashboard=ADDR            --slot-action    --remove --sid=ID
	codis-admin [-v] --dashboard=ADDR            --slot-action    --create-range --beg=ID --end=ID --gid=ID
	codis-admin [-v] --dashboard=ADDR            --slot-action    --interval=VALUE
	codis-admin [-v] --dashboard=ADDR            --slot-action    --disabled=VALUE
	codis-admin [-v] --dashboard=ADDR            --rebalance     [--confirm]
	codis-admin [-v] --remove-lock               --product=NAME (--zookeeper=ADDR|--etcd=ADDR)
	codis-admin [-v] --config-dump               --product=NAME (--zookeeper=ADDR|--etcd=ADDR) [-1]
	codis-admin [-v] --config-convert=FILE
	codis-admin [-v] --config-restore=FILE       --product=NAME (--zookeeper=ADDR|--etcd=ADDR) [--confirm]
	codis-admin [-v] --dashboard-list                           (--zookeeper=ADDR|--etcd=ADDR)

Options:
	-a AUTH, --auth=AUTH
	-x ADDR, --addr=ADDR
	-t TOKEN, --token=TOKEN
`

	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}
	log.SetLevel(log.LevelInfo)

	if d["-v"].(bool) {
		log.SetLevel(log.LevelDebug)
	}

	switch {
	case d["--proxy"] != nil:
		new(cmdProxy).Main(d)
	case d["--dashboard"] != nil:
		new(cmdDashboard).Main(d)
	default:
		new(cmdAdmin).Main(d)
	}
}
