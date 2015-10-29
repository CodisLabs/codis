package main

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/topom"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type cmdDashboard struct {
	address string
	product struct {
		name string
		auth string
	}
}

func (t *cmdDashboard) Main(d map[string]interface{}) {
	if d["--dashboard"] != nil {
		t.address = d["--dashboard"].(string)
		if s, ok := d["--product-name"].(string); ok {
			t.product.name = s
		}
		if s, ok := d["--product-auth"].(string); ok {
			t.product.auth = s
		}
	} else {
		config := topom.NewDefaultConfig()
		if err := config.LoadFromFile(d["--config"].(string)); err != nil {
			log.PanicErrorf(err, "load config file failed")
		}
		addr, err := net.ResolveTCPAddr("tcp", config.AdminAddr)
		if err != nil {
			log.PanicErrorf(err, "resolve tcp addr = %s failed", config.AdminAddr)
		}
		if ipv4 := addr.IP.To4(); ipv4 != nil {
			if net.IPv4zero.Equal(ipv4) {
				log.Panicf("invalid tcp address = %s", config.AdminAddr)
			}
		} else if ipv6 := addr.IP.To16(); ipv6 != nil {
			if net.IPv6zero.Equal(ipv6) {
				log.Panicf("invalid tcp address = %s", config.AdminAddr)
			}
		}
		t.address = addr.String()
		t.product.name = config.ProductName
		t.product.auth = config.ProductAuth
	}

	var cmd string
	for _, s := range []string{"overview", "config", "model", "slots", "stats", "shutdown", "proxy", "group"} {
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
		t.handleOverview(cmd, d)
		/*
			case "proxy":
				t.handleProxyCommand(d)
			case "group":
				t.handleGroupCommand(d)
			case "action":
				t.handleActionCommand(d)
		*/
	case "shutdown":
		t.handleShutdown(d)
	}
}

func (t *cmdDashboard) newTopomClient(xauth bool) (*topom.ApiClient, *models.Topom) {
	client := topom.NewApiClient(t.address)

	log.Debugf("call rpc model")
	p, err := client.Model()
	if err != nil {
		log.PanicErrorf(err, "call rpc model failed")
	}
	log.Debugf("call rpc model OK, model =\n%s", p.Encode())

	if !xauth {
		if t.product.name != p.ProductName && t.product.name != "" {
			log.Panicf("wrong product name, should be = %s", p.ProductName)
		}
		return client, p
	} else {
		if t.product.name != p.ProductName {
			log.Panicf("wrong product name, should be = %s", p.ProductName)
		}
	}

	client.SetXAuth(p.ProductName, t.product.auth)

	log.Debugf("call rpc xping")
	if err := client.XPing(); err != nil {
		log.PanicErrorf(err, "call rpc xping failed")
	}
	log.Debugf("call rpc xping OK")

	return client, p
}

func (t *cmdDashboard) handleOverview(cmd string, d map[string]interface{}) {
	client, _ := t.newTopomClient(false)

	log.Debugf("call rpc overview")
	o, err := client.Overview()
	if err != nil {
		log.PanicErrorf(err, "call rpc overview failed")
	}
	log.Debugf("call rpc overview OK")

	var obj interface{}
	switch cmd {
	default:
		o.Stats = nil
		obj = o
	case "overview":
		obj = o
	case "config":
		obj = o.Config
	case "model":
		obj = o.Model
	case "stats":
		o.Stats.Slots = nil
		obj = o.Stats
	case "slots":
		obj = o.Stats.Slots
	case "proxy":
		switch {
		case d["--stats-map"].(bool):
			obj = o.Stats.Proxy.Stats
		case d["--list"].(bool):
			obj = o.Stats.Proxy.Models
		}
	case "group":
		switch {
		case d["--stats-map"].(bool):
			obj = o.Stats.Group.Stats
		case d["--list"].(bool):
			obj = o.Stats.Group.Models
		}
	}

	b, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}

func (t *cmdDashboard) handleShutdown(d map[string]interface{}) {
	client, _ := t.newTopomClient(true)

	log.Debugf("call rpc shutdown")
	if err := client.Shutdown(); err != nil {
		log.PanicErrorf(err, "call rpc shutdown failed")
	}
	log.Debugf("call rpc shutdown OK")
}

/*

func (t *cmdDashboard) getProxyToken(addr string) string {
	client := proxy.NewApiClient(addr)

	p, err := client.Model()
	if err != nil {
		log.PanicErrorf(err, "call rpc model failed, proxy = %s", addr)
	}
	log.Debugf("get proxy model =\n%s", p.Encode())

	if t.product.name != "" && t.product.name != p.ProductName {
		log.Panicf("wrong product name, should be = %s", p.ProductName)
	}

	client.SetXAuth(p.ProductName, t.product.auth, p.Token)
	if err := client.XPing(); err != nil {
		log.Panicf("call rpc xping failed, invalid password")
	}
	return p.Token
}

func (t *cmdDashboard) fetchProxyModel(client *topom.ApiClient, d map[string]interface{}) *models.Proxy {
	o, err := client.Overview()
	if err != nil {
		log.PanicErrorf(err, "call rpc overview failed")
	}

	var match = func(p *models.Proxy) bool {
		return false
	}

	switch {
	case d["--addr"] != nil:
		token := t.getProxyToken(d["--addr"].(string))
		match = func(p *models.Proxy) bool {
			return p.Token == token
		}
	case d["--token"] != nil:
		token := d["--token"].(string)
		match = func(p *models.Proxy) bool {
			return p.Token == token
		}
	case d["--proxy-id"] != nil:
		if id, err := strconv.Atoi(d["--proxy-id"].(string)); err != nil {
			log.PanicErrorf(err, "parse --proxy-id failed")
		} else {
			match = func(p *models.Proxy) bool {
				return p.Id == id
			}
		}
	}
	for _, p := range o.Stats.Proxy.Models {
		if match(p) {
			return p
		}
	}
	return nil
}

func (t *cmdDashboard) handleProxyCommand(d map[string]interface{}) {
	client, _ := t.newTopomClient(true)

	switch {
	case d["--create"].(bool):
		addr := d["--addr"].(string)
		if addr == "" {
			log.Panicf("create-proxy, proxy.addr is empty")
		}
		if err := client.CreateProxy(addr); err != nil {
			log.PanicErrorf(err, "call rpc create-proxy failed")
		} else {
			log.Infof("create-proxy with proxy.addr = %s", addr)
		}
	case d["--remove"].(bool):
		p := t.fetchProxyModel(client, d)
		if p == nil {
			log.Panicf("remove-proxy proxy doesn't exist")
		}
		if err := client.RemoveProxy(p.Token, d["--force"].(bool)); err != nil {
			log.PanicErrorf(err, "call rpc remove-proxy failed")
		} else {
			log.Infof("remove-proxy successfully")
		}
	case d["--reinit"].(bool):
		p := t.fetchProxyModel(client, d)
		if p == nil {
			log.Panicf("remove-proxy proxy doesn't exist")
		}
		if err := client.ReinitProxy(p.Token); err != nil {
			log.PanicErrorf(err, "call rpc reinit-proxy failed")
		} else {
			log.Infof("reinit-proxy successfully")
		}
	}
}

func (c *cmdDashboard) handleGroupCommand(d map[string]interface{}) {
}
*/
