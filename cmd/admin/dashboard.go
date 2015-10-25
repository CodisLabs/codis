package main

import (
	"bufio"
	"encoding/json"
	"os"

	"github.com/wandoulabs/codis/pkg/topom"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type cmdDashboard struct {
	address string
	product struct {
		name string
		auth string
	}
	subcmd string
	output string
}

func (c *cmdDashboard) Main(d map[string]interface{}) {
	c.address = d["--dashboard"].(string)

	if file, ok := d["--output"].(string); ok {
		c.output = file
	} else {
		c.output = "/dev/stdout"
	}

	if s, ok := d["--product_name"].(string); ok {
		c.product.name = s
	}
	if s, ok := d["--product_auth"].(string); ok {
		c.product.auth = s
	}

	for _, t := range []string{"simple", "config", "model", "stats", "overview", "shutdown"} {
		if d[t].(bool) {
			c.subcmd = t
		}
	}
	if c.subcmd == "" {
		c.subcmd = "simple"
	}

	log.Debugf("args.address = %s", c.address)
	log.Debugf("args.subcmd = %s", c.subcmd)
	switch c.subcmd {
	default:
		log.Debugf("args.output = %s", c.output)
		c.handleOverview()
	case "shutdown":
		log.Debugf("args.product.name = %s", c.product.name)
		log.Debugf("args.product.auth = %s", c.product.auth)
		c.handleShutdown()
	}
}

func (c *cmdDashboard) handleOverview() {
	f, err := os.OpenFile(c.output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.PanicErrorf(err, "can't open file %s", c.output)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer func() {
		if err := w.Flush(); err != nil {
			log.PanicErrorf(err, "flush outout failed")
		}
	}()

	client := topom.NewApiClient(c.address)

	o, err := client.Overview()
	if err != nil {
		log.PanicErrorf(err, "call rpc overview failed")
	}

	var result interface{}
	switch c.subcmd {
	case "overview":
		result = o
	case "config", "model", "stats":
		result = o[c.subcmd]
	case "simple":
		result = map[string]interface{}{
			"version": o["version"],
			"compile": o["compile"],
			"config":  o["config"], "model": o["model"],
		}
	}

	b, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	log.Debugf("total bytes = %d", len(b)+1)

	if _, err := w.Write(b); err != nil {
		log.PanicErrorf(err, "write output failed")
	}
	if _, err := w.WriteString("\n"); err != nil {
		log.PanicErrorf(err, "write output failed")
	}
}

func (c *cmdDashboard) handleShutdown() {
	client := topom.NewApiClient(c.address)

	p, err := client.Model()
	if err != nil {
		log.PanicErrorf(err, "call rpc model failed")
	}
	log.Debugf("get topom model =\n%s", p.Encode())

	if p.ProductName != c.product.name {
		log.Panicf("wrong product name, should be = %s", p.ProductName)
	}

	client.SetXAuth(c.product.name, c.product.auth)
	if err := client.XPing(); err != nil {
		log.Panicf("call rpc xping failed, invalid password")
	}

	if err := client.Shutdown(); err != nil {
		log.Panicf("call rpc shutdown failed")
	}
}
