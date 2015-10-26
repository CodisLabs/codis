package main

import (
	"encoding/json"
	"fmt"

	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type cmdProxy struct {
	address string
	product struct {
		name string
		auth string
	}
}

func (t *cmdProxy) Main(d map[string]interface{}) {
	t.address = d["--proxy"].(string)

	if s, ok := d["--product_name"].(string); ok {
		t.product.name = s
	}
	if s, ok := d["--product_auth"].(string); ok {
		t.product.auth = s
	}

	var cmd string
	for _, s := range []string{"overview", "config", "model", "slots", "stats", "shutdown"} {
		if d[s].(bool) {
			cmd = s
		}
	}

	log.Debugf("args.command = %s", cmd)
	log.Debugf("args.address = %s", t.address)
	log.Debugf("args.product.name = %s", t.product.name)
	log.Debugf("args.product.auth = %s", t.product.auth)

	switch cmd {
	default:
		t.handleOverview(cmd)
	case "shutdown":
		t.handleShutdown()
	}
}

func (t *cmdProxy) handleOverview(cmd string) {
	client := proxy.NewApiClient(t.address)

	o, err := client.Overview()
	if err != nil {
		log.PanicErrorf(err, "call rpc overview failed")
	}

	var obj interface{}
	switch cmd {
	default:
		o.Slots = nil
		o.Stats = nil
		fallthrough
	case "overview":
		obj = o
	case "config":
		obj = o.Config
	case "model":
		obj = o.Model
	case "slots":
		obj = o.Slots
	case "stats":
		obj = o.Stats
	}

	b, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	log.Debugf("total bytes = %d", len(b))

	fmt.Println(string(b))
}

func (t *cmdProxy) handleShutdown() {
	client := proxy.NewApiClient(t.address)

	p, err := client.Model()
	if err != nil {
		log.PanicErrorf(err, "call rpc model failed")
	}
	log.Debugf("get proxy model =\n%s", p.Encode())

	if p.ProductName != t.product.name {
		log.Panicf("wrong product name, should be = %s", p.ProductName)
	}

	client.SetXAuth(p.ProductName, t.product.auth, p.Token)
	if err := client.XPing(); err != nil {
		log.Panicf("call rpc xping failed, invalid password")
	}

	if err := client.Shutdown(); err != nil {
		log.Panicf("call rpc shutdown failed")
	} else {
		log.Infof("shutdown-proxy successfully")
	}
}
