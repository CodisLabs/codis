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
	codis-admin [--debug] --proxy=ADDR     [simple|config|model|stats|slots|overview] [--output=FILE]
	codis-admin [--debug] --proxy=ADDR      shutdown --product_name=NAME [--product_auth=AUTH]
	codis-admin [--debug] --dashboard=ADDR [simple|config|model|stats|slots|group|proxy|overview] [--output=FILE]
	codis-admin [--debug] --dashboard=ADDR  shutdown --product_name=NAME [--product_auth=AUTH]

Options:
	-o FILE, --output=FILE
`

	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}
	log.SetLevel(log.LEVEL_INFO)

	if d["--debug"].(bool) {
		log.SetLevel(log.LEVEL_DEBUG)
	} else {
		log.SetLevel(log.LEVEL_INFO)
	}

	switch {
	case d["--proxy"] != nil:
		new(cmdProxy).Main(d)
	case d["--dashboard"] != nil:
		new(cmdDashboard).Main(d)
	}
}
