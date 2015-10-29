package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
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
	case "proxy":
		t.handleProxyCommand(d)
		/*
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
		log.PanicErrorf(err, "call rpc model to dashboard %s failed", t.address)
	}
	log.Debugf("call rpc model OK")
	log.Debugf("topom model =\n%s", p.Encode())

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
		log.PanicErrorf(err, "call rpc xping to dashboard %s failed", t.address)
	}
	log.Debugf("call rpc xping OK")

	return client, p
}

func (t *cmdDashboard) handleOverview(cmd string, d map[string]interface{}) {
	client, _ := t.newTopomClient(false)

	log.Debugf("call rpc overview")
	o, err := client.Overview()
	if err != nil {
		log.PanicErrorf(err, "call rpc overview to dashboard %s failed", t.address)
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
		log.PanicErrorf(err, "call rpc shutdown to dashboard %s failed", t.address)
	}
	log.Debugf("call rpc shutdown OK")
}

func (t *cmdDashboard) parseInteger(d map[string]interface{}, name string) int {
	if s, ok := d[name].(string); ok && s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			log.PanicErrorf(err, "parse argument %s failed", name)
		}
		log.Debugf("parse %s = %d", name, v)
		return v
	}
	log.Panicf("parse argument %s failed, not found or blank string", name)
	return 0
}

func (t *cmdDashboard) parseString(d map[string]interface{}, name string) string {
	if s, ok := d[name].(string); ok && s != "" {
		log.Debugf("parse %s = %s", name, s)
		return s
	}
	log.Panicf("parse argument %s failed, not found or blank string", name)
	return ""
}

func (t *cmdDashboard) parseProxyToken(client *topom.ApiClient, d map[string]interface{}) string {
	switch {
	case d["--token"] != nil:
		return t.parseString(d, "--token")

	case d["--addr"] != nil:
		addr := t.parseString(d, "--addr")

		client := proxy.NewApiClient(addr)

		log.Debugf("call rpc model")
		p, err := client.Model()
		if err != nil {
			log.PanicErrorf(err, "call rpc model to proxy %s failed", addr)
		}
		log.Debugf("call rpc model OK")
		log.Debugf("proxy model =\n%s", p.Encode())

		return p.Token

	case d["--proxy-id"] != nil:
		log.Debugf("call rpc stats")
		stats, err := client.Stats()
		if err != nil {
			log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc stats OK")

		id := t.parseInteger(d, "--proxy-id")
		for _, p := range stats.Proxy.Models {
			if p.Id == id {
				return p.Token
			}
		}

	}

	log.Panicf("can't find specified proxy")
	return ""
}

func (t *cmdDashboard) handleProxyCommand(d map[string]interface{}) {
	switch {
	case d["--create"].(bool):
		client, _ := t.newTopomClient(true)

		addr := t.parseString(d, "--addr")

		if _, err := proxy.NewApiClient(addr).Model(); err != nil {
			log.PanicErrorf(err, "call rpc model to proxy %s failed", addr)
		}

		log.Debugf("call rpc create-proxy")
		if err := client.CreateProxy(addr); err != nil {
			log.PanicErrorf(err, "call rpc create-proxy to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc create-proxy OK")

	case d["--remove"].(bool):
		client, _ := t.newTopomClient(true)

		token := t.parseProxyToken(client, d)
		force := d["--force"].(bool)
		log.Debugf("parse --force = %t", force)

		log.Debugf("call rpc remove-proxy")
		if err := client.RemoveProxy(token, force); err != nil {
			log.PanicErrorf(err, "call rpc remove-proxy to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc remove-proxy OK")

	case d["--reinit"].(bool):
		client, _ := t.newTopomClient(true)

		token := t.parseProxyToken(client, d)

		log.Debugf("call rpc reinit-proxy")
		if err := client.ReinitProxy(token); err != nil {
			log.PanicErrorf(err, "call rpc remove-reinit to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc reinit-proxy OK")

	default:
		d["--list"] = true
		fallthrough

	case d["--list"].(bool) || d["--stats-map"].(bool):
		t.handleOverview("proxy", d)
	}
}

/*
func (c *cmdDashboard) handleGroupCommand(d map[string]interface{}) {
}
*/
