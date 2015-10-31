package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/topom"
	"github.com/wandoulabs/codis/pkg/utils"
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
	for _, s := range []string{"overview", "config", "model", "slots", "stats", "shutdown", "proxy", "group", "action"} {
		if d[s].(bool) {
			cmd = s
		}
	}

	log.Debugf("args.command = %s", cmd)
	log.Debugf("args.address = %s", t.address)
	log.Debugf("args.product.name = %s", t.product.name)
	log.Debugf("args.product.auth = %s", t.product.auth)

	if t.product.name != "" {
		if !utils.IsValidName(t.product.name) {
			log.Panicf("invalid product name = %s", t.product.name)
		}
	}

	switch cmd {
	default:
		t.handleOverview(cmd, d)
	case "proxy":
		t.handleProxyCommand(d)
	case "group":
		t.handleGroupCommand(d)
	case "action":
		t.handleActionCommand(d)
	case "shutdown":
		t.handleShutdown(d)
	}
}

func (t *cmdDashboard) newTopomClient(xauth bool) (*topom.ApiClient, *models.Topom) {
	client := topom.NewApiClient(t.address)

	log.Debugf("call rpc model to dashboard %s", t.address)
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

	log.Debugf("call rpc xping to dashboard %s", t.address)
	if err := client.XPing(); err != nil {
		log.PanicErrorf(err, "call rpc xping to dashboard %s failed", t.address)
	}
	log.Debugf("call rpc xping OK")

	return client, p
}

func (t *cmdDashboard) newProxyClient(addr string, xauth bool) (*proxy.ApiClient, *models.Proxy) {
	client := proxy.NewApiClient(addr)

	log.Debugf("call rpc model to proxy %s", addr)
	p, err := client.Model()
	if err != nil {
		log.PanicErrorf(err, "call rpc model to proxy %s failed", addr)
	}
	log.Debugf("call rpc model OK")
	log.Debugf("proxy model =\n%s", p.Encode())

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

	client.SetXAuth(p.ProductName, t.product.auth, p.Token)

	log.Debugf("call rpc xping to proxy %s", addr)
	if err := client.XPing(); err != nil {
		log.PanicErrorf(err, "call rpc xping to proxy %s failed", addr)
	}
	log.Debugf("call rpc xping OK")

	return client, p
}

func (t *cmdDashboard) handleOverview(cmd string, d map[string]interface{}) {
	client, _ := t.newTopomClient(false)

	log.Debugf("call rpc overview to dashboard %s", t.address)
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

	log.Debugf("call rpc shutdown to dashboard %s", t.address)
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
		_, p := t.newProxyClient(addr, true)
		return p.Token

	case d["--proxy-id"] != nil:
		log.Debugf("call rpc stats to dashboard %s", t.address)
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
		fallthrough

	default:
		log.Panicf("can't find specified proxy")
		return ""
	}
}

func (t *cmdDashboard) parseGroupServer(client *topom.ApiClient, d map[string]interface{}) (int, string) {
	groupId := t.parseInteger(d, "--group-id")

	switch {
	case d["--addr"] != nil:
		return groupId, t.parseString(d, "--addr")

	case d["--index"] != nil:
		index := t.parseInteger(d, "--index")

		log.Debugf("call rpc stats to dashboard %s", t.address)
		stats, err := client.Stats()
		if err != nil {
			log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc stats OK")

		for _, g := range stats.Group.Models {
			if g.Id == groupId {
				if index < 0 {
					index += len(g.Servers)
				}
				if index < 0 || index >= len(g.Servers) {
					log.Panicf("invalid index, out of range")
				}
				return groupId, g.Servers[index]
			}
		}
		fallthrough

	default:
		log.Panicf("can't find specifed group")
		return 0, ""
	}
}

func (t *cmdDashboard) handleProxyCommand(d map[string]interface{}) {
	switch {
	case d["--create"].(bool):
		client, _ := t.newTopomClient(true)

		addr := t.parseString(d, "--addr")
		_, p := t.newProxyClient(addr, true)
		log.Debugf("create proxy with token = %s, addr = %s", p.Token, addr)

		log.Debugf("call rpc create-proxy to dashboard %s", t.address)
		if err := client.CreateProxy(addr); err != nil {
			log.PanicErrorf(err, "call rpc create-proxy to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc create-proxy OK")

	case d["--remove"].(bool):
		client, _ := t.newTopomClient(true)

		token := t.parseProxyToken(client, d)
		force := d["--force"].(bool)
		log.Debugf("remove proxy with token = %s, force = %t", token, force)

		log.Debugf("call rpc remove-proxy to dashboard %s", t.address)
		if err := client.RemoveProxy(token, force); err != nil {
			log.PanicErrorf(err, "call rpc remove-proxy to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc remove-proxy OK")

	case d["--reinit"].(bool):
		client, _ := t.newTopomClient(true)

		token := t.parseProxyToken(client, d)
		log.Debugf("reinit proxy with token = %s", token)

		log.Debugf("call rpc reinit-proxy to dashboard %s", t.address)
		if err := client.ReinitProxy(token); err != nil {
			log.PanicErrorf(err, "call rpc reinit-proxy to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc reinit-proxy OK")

	case d["--xpingall"].(bool):
		client, _ := t.newTopomClient(true)

		log.Debugf("call rpc stats to dashboard %s", t.address)
		stats, err := client.Stats()
		if err != nil {
			log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc stats OK")

		fmt.Printf("      Total=%d\n", len(stats.Proxy.Models))
		for _, p := range stats.Proxy.Models {
			c := proxy.NewApiClient(p.AdminAddr)
			c.SetXAuth(p.ProductName, t.product.auth, p.Token)
			if err := c.XPing(); err != nil {
				fmt.Printf("[EE]")
			} else {
				fmt.Printf("[OK]")
			}
			fmt.Printf("  proxy-%-4d    %s      %s\n", p.Id, p.Token, p.AdminAddr)
		}

	default:
		d["--list"] = true
		fallthrough

	case d["--list"].(bool) || d["--stats-map"].(bool):
		t.handleOverview("proxy", d)
	}
}

func (t *cmdDashboard) handleGroupCommand(d map[string]interface{}) {
	switch {
	case d["--create"].(bool):
		client, _ := t.newTopomClient(true)

		groupId := t.parseInteger(d, "--group-id")
		log.Debugf("create group-[%d]", groupId)

		log.Debugf("call rpc create-group to dashboard %s", t.address)
		if err := client.CreateGroup(groupId); err != nil {
			log.PanicErrorf(err, "call rpc create-group to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc create-group OK")

	case d["--remove"].(bool):
		client, _ := t.newTopomClient(true)

		groupId := t.parseInteger(d, "--group-id")
		log.Debugf("remove group-[%d]", groupId)

		log.Debugf("call rpc remove-group to dashboard %s", t.address)
		if err := client.RemoveGroup(groupId); err != nil {
			log.PanicErrorf(err, "call rpc remove-group to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc remove-group OK")

	case d["--add"].(bool):
		client, _ := t.newTopomClient(true)

		groupId, addr := t.parseInteger(d, "--group-id"), t.parseString(d, "--addr")
		log.Debugf("group-[%d] add server = %s", groupId, addr)

		log.Debugf("call rpc check-server to dashboard %s", t.address)
		if err := client.GroupCheckServer(addr); err != nil {
			log.PanicErrorf(err, "call rpc check-server to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc check-server OK")

		log.Debugf("call rpc group add-server to dashboard %s", t.address)
		if err := client.GroupAddServer(groupId, addr); err != nil {
			log.PanicErrorf(err, "call rpc group add-server to dashbard %s failed", t.address)
		}
		log.Debugf("call rpc group add-server OK")

	case d["--del"].(bool):
		client, _ := t.newTopomClient(true)

		groupId, addr := t.parseGroupServer(client, d)
		log.Debugf("group-[%d] del server = %s", groupId, addr)

		log.Debugf("call rpc group del-server to dashboard %s", t.address)
		if err := client.GroupDelServer(groupId, addr); err != nil {
			log.PanicErrorf(err, "call rpc group del-server to dashbard %s failed", t.address)
		}
		log.Debugf("call rpc group del-server OK")

	case d["--promote"].(bool):
		client, _ := t.newTopomClient(true)

		groupId, addr := t.parseGroupServer(client, d)
		log.Debugf("group-[%d] promote server = %s", groupId, addr)

		log.Debugf("call rpc group promote to dashboard %s", t.address)
		if err := client.GroupPromoteServer(groupId, addr); err != nil {
			log.PanicErrorf(err, "call rpc group promote to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc group promote to dashboard OK")

		log.Debugf("call rpc group promote-commit to dashboard %s", t.address)
		if err := client.GroupPromoteCommit(groupId); err != nil {
			log.PanicErrorf(err, "call rpc group promote-commit to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc group promote-commit to dashboard OK")

	case d["--promote-commit"].(bool):
		client, _ := t.newTopomClient(true)

		groupId := t.parseInteger(d, "--group-id")
		log.Debugf("group-[%d] promote-commit", groupId)

		log.Debugf("call rpc group promote-commit to dashboard %s", t.address)
		if err := client.GroupPromoteCommit(groupId); err != nil {
			log.PanicErrorf(err, "call rpc group promote-commit to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc group promote-commit to dashboard OK")

	case d["--master-status"].(bool):
		client, _ := t.newTopomClient(true)

		log.Debugf("call rpc stats to dashboard %s", t.address)
		stats, err := client.Stats()
		if err != nil {
			log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc stats OK")

		for _, g := range stats.Group.Models {
			fmt.Printf("group-%-6d -----+   ", g.Id)
			for i, addr := range g.Servers {
				var infom map[string]string
				var master string
				if x := stats.Group.Stats[addr]; x != nil {
					infom, master = x.Infom, x.Infom["master_addr"]
				}
				if i == 0 {
					switch {
					case infom != nil && master == "":
						fmt.Printf("[M] %s", addr)
					case infom != nil && master != "":
						fmt.Printf("[E] %s ==> %s", addr, master)
					default:
						fmt.Printf("[?] %s", addr)
					}
				} else {
					fmt.Println()
					fmt.Printf("             ")
					fmt.Printf("     +   ")
					switch {
					case infom != nil && master == g.Servers[0]:
						fmt.Printf("[S] %s", addr)
					case infom != nil && master != "":
						fmt.Printf("[E] %s ==> %s", addr, master)
					case infom != nil && master == "":
						fmt.Printf("[E] %s", addr)
					default:
						fmt.Printf("[?] %s", addr)
					}
				}
			}
			fmt.Println()
		}

	case d["--master-repair"].(bool):
		client, _ := t.newTopomClient(true)

		groupId, addr := t.parseGroupServer(client, d)
		log.Debugf("group-[%d] repair-master server = %s", groupId, addr)

		log.Debugf("call rpc group repair-master to dashboard %s", t.address)
		if err := client.GroupRepairMaster(groupId, addr); err != nil {
			log.PanicErrorf(err, "call rpc group repair-master to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc group repair-master to dashboard OK")

	default:
		d["--list"] = true
		fallthrough

	case d["--list"].(bool) || d["--stats-map"].(bool):
		t.handleOverview("group", d)
	}
}

func (t *cmdDashboard) handleActionCommand(d map[string]interface{}) {
	switch {
	case d["--create"].(bool):
		client, _ := t.newTopomClient(true)

		slotId, groupId := t.parseInteger(d, "--slot-id"), t.parseInteger(d, "--group-id")
		log.Debugf("create action slot-[%d] to group-[%d]", slotId, groupId)

		log.Debugf("call rpc create-action to dashboard %s", t.address)
		if err := client.SlotCreateAction(slotId, groupId); err != nil {
			log.PanicErrorf(err, "call rpc create-action to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc create-action OK")

	case d["--remove"].(bool):
		client, _ := t.newTopomClient(true)

		slotId := t.parseInteger(d, "--slot-id")
		log.Debugf("remove action slot-[%d]", slotId)

		log.Debugf("call rpc remove-action to dashboard %s", t.address)
		if err := client.SlotRemoveAction(slotId); err != nil {
			log.PanicErrorf(err, "call rpc remove-action to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc remove-action OK")

	case d["--create-range"].(bool):
		client, _ := t.newTopomClient(true)

		groupId := t.parseInteger(d, "--group-id")
		slotBeg := t.parseInteger(d, "--slot-beg")
		slotEnd := t.parseInteger(d, "--slot-end")
		log.Debugf("create action range slot-[%d - %d] to group-[%d]", slotBeg, slotEnd, groupId)

		log.Debugf("call rpc create-action-range to dashboard %s", t.address)
		if err := client.SlotCreateActionRange(slotBeg, slotEnd, groupId); err != nil {
			log.PanicErrorf(err, "call rpc create-action-range to dashboard %s failed", t.address)
		}
		log.Debugf("call rpc create-action-range OK")

	case d["--set"].(bool):
		client, _ := t.newTopomClient(true)

		switch {
		case d["--interval"] != nil:
			interval := t.parseInteger(d, "--interval")
			log.Debugf("call rpc set action-interval to dashboard %s", t.address)
			if err := client.SetActionInterval(interval); err != nil {
				log.PanicErrorf(err, "call rpc set action-interval to dashboard %s failed", t.address)
			}
			log.Debugf("call rpc set action-interval OK")

		case d["--disabled"] != nil:
			disabled := t.parseInteger(d, "--disabled") != 0
			log.Debugf("call rpc set action-disabled to dashboard %s", t.address)
			if err := client.SetActionDisabled(disabled); err != nil {
				log.PanicErrorf(err, "call rpc set action-disabled to dashboard %s failed", t.address)
			}
			log.Debugf("call rpc set action-disabled OK")

		}
	}
}
