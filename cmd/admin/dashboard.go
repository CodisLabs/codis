// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/topom"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type cmdDashboard struct {
	addr string
}

func (t *cmdDashboard) Main(d map[string]interface{}) {
	t.addr = utils.ArgumentMust(d, "--dashboard")

	switch {
	default:
		t.handleOverview(d)
	case d["--shutdown"].(bool):
		t.handleShutdown(d)
	case d["--log-level"] != nil:
		t.handleLogLevel(d)

	case d["--slots-remap"] != nil:
		fallthrough
	case d["--slots-status"].(bool):
		t.handleSlotsCommand(d)

	case d["--create-proxy"].(bool):
		fallthrough
	case d["--remove-proxy"].(bool):
		fallthrough
	case d["--reinit-proxy"].(bool):
		t.handleProxyCommand(d)

	case d["--create-group"].(bool):
		fallthrough
	case d["--remove-group"].(bool):
		fallthrough
	case d["--group-add"].(bool):
		fallthrough
	case d["--group-del"].(bool):
		fallthrough
	case d["--group-status"].(bool):
		fallthrough
	case d["--promote-server"].(bool):
		fallthrough
	case d["--promote-commit"].(bool):
		t.handleGroupCommand(d)

	case d["--sync-action"].(bool):
		t.handleSyncActionCommand(d)

	case d["--slot-action"].(bool):
		t.handleSlotActionCommand(d)

	}
}

func (t *cmdDashboard) newTopomClient() *topom.ApiClient {
	c := topom.NewApiClient(t.addr)

	log.Debugf("call rpc model to dashboard %s", t.addr)
	p, err := c.Model()
	if err != nil {
		log.PanicErrorf(err, "call rpc model to dashboard %s failed", t.addr)
	}
	log.Debugf("call rpc model OK")

	c.SetXAuth(p.ProductName)

	log.Debugf("call rpc xping to dashboard %s", t.addr)
	if err := c.XPing(); err != nil {
		log.PanicErrorf(err, "call rpc xping to dashboard %s failed", t.addr)
	}
	log.Debugf("call rpc xping OK")

	return c
}

func (t *cmdDashboard) handleOverview(d map[string]interface{}) {
	c := t.newTopomClient()

	log.Debugf("call rpc overview to dashboard %s", t.addr)
	o, err := c.Overview()
	if err != nil {
		log.PanicErrorf(err, "call rpc overview to dashboard %s failed", t.addr)
	}
	log.Debugf("call rpc overview OK")

	var cmd string
	for _, s := range []string{"config", "model", "slots", "stats", "group", "proxy", "--list-group", "--list-proxy"} {
		if d[s].(bool) {
			cmd = s
		}
	}

	var obj interface{}
	switch cmd {
	default:
		obj = o
	case "config":
		obj = o.Config
	case "model":
		obj = o.Model
	case "stats":
		obj = o.Stats
	case "slots":
		if o.Stats != nil {
			obj = o.Stats.Slots
		}
	case "group":
		if o.Stats != nil {
			obj = o.Stats.Group
		}
	case "--list-group":
		if o.Stats != nil {
			obj = o.Stats.Group.Models
		}
	case "proxy":
		if o.Stats != nil {
			obj = o.Stats.Proxy
		}
	case "--list-proxy":
		if o.Stats != nil {
			obj = o.Stats.Proxy.Models
		}
	}

	b, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}

func (t *cmdDashboard) handleLogLevel(d map[string]interface{}) {
	c := t.newTopomClient()

	s := utils.ArgumentMust(d, "--log-level")

	var v log.LogLevel
	if !v.ParseFromString(s) {
		log.Panicf("option --log-level = %s", s)
	}

	log.Debugf("call rpc loglevel to dashboard %s", t.addr)
	if err := c.LogLevel(v); err != nil {
		log.PanicErrorf(err, "call rpc loglevel to dashboard %s failed", t.addr)
	}
	log.Debugf("call rpc loglevel OK")
}

func (t *cmdDashboard) handleShutdown(d map[string]interface{}) {
	c := t.newTopomClient()

	log.Debugf("call rpc shutdown to dashboard %s", t.addr)
	if err := c.Shutdown(); err != nil {
		log.PanicErrorf(err, "call rpc shutdown to dashboard %s failed", t.addr)
	}
	log.Debugf("call rpc shutdown OK")
}

func (t *cmdDashboard) handleSlotsCommand(d map[string]interface{}) {
	c := t.newTopomClient()

	switch {

	case d["--slots-status"].(bool):

		log.Debugf("call rpc slots to dashboard %s", t.addr)
		o, err := c.Slots()
		if err != nil {
			log.PanicErrorf(err, "call rpc slots to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc slots OK")

		b, err := json.MarshalIndent(o, "", "    ")
		if err != nil {
			log.PanicErrorf(err, "json marshal failed")
		}
		fmt.Println(string(b))

	case d["--slots-remap"] != nil:

		file := utils.ArgumentMust(d, "--slots-remap")
		b, err := ioutil.ReadFile(file)
		if err != nil {
			log.PanicErrorf(err, "read file '%s' failed", file)
		}

		slots := []*models.SlotMapping{}
		if err := json.Unmarshal(b, &slots); err != nil {
			log.PanicErrorf(err, "json unmarshal failed")
		}

		if !d["--confirm"].(bool) {
			b, err := json.MarshalIndent(slots, "", "    ")
			if err != nil {
				log.PanicErrorf(err, "json marshal failed")
			}
			fmt.Println(string(b))
			return
		}

		log.Debugf("call rpc slots-remap to dashboard %s", t.addr)
		if err := c.SlotsRemapGroup(slots); err != nil {
			log.PanicErrorf(err, "call rpc slots-remap to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc slots-remap OK")

	}
}

func (t *cmdDashboard) parseProxyToken(d map[string]interface{}) string {
	switch {

	default:

		log.Panicf("cann't find specific proxy")

		return ""

	case d["--token"] != nil:

		return utils.ArgumentMust(d, "--token")

	case d["--pid"] != nil:

		pid := utils.ArgumentIntegerMust(d, "--pid")

		c := t.newTopomClient()

		log.Debugf("call rpc stats to dashboard %s", t.addr)
		s, err := c.Stats()
		if err != nil {
			log.Debugf("call rpc stats to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc stats OK")

		for _, p := range s.Proxy.Models {
			if p.Id == pid {
				return p.Token
			}
		}

		log.Panicf("cann't find specific proxy with id = %d", pid)

		return ""

	case d["--addr"] != nil:

		addr := utils.ArgumentMust(d, "--addr")

		c := proxy.NewApiClient(addr)

		log.Debugf("call rpc model to proxy %s", t.addr)
		p, err := c.Model()
		if err != nil {
			log.PanicErrorf(err, "call rpc model to proxy %s failed", t.addr)
		}
		log.Debugf("call rpc model OK")

		return p.Token

	}
}

func (t *cmdDashboard) handleProxyCommand(d map[string]interface{}) {
	c := t.newTopomClient()

	switch {

	case d["--create-proxy"].(bool):

		addr := utils.ArgumentMust(d, "--addr")

		log.Debugf("call rpc create-proxy to dashboard %s", t.addr)
		if err := c.CreateProxy(addr); err != nil {
			log.PanicErrorf(err, "call rpc create-proxy to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc create-proxy OK")

	case d["--remove-proxy"].(bool):

		token := t.parseProxyToken(d)
		force := d["--force"].(bool)

		log.Debugf("call rpc remove-proxy to dashboard %s", t.addr)
		if err := c.RemoveProxy(token, force); err != nil {
			log.PanicErrorf(err, "call rpc remove-proxy to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc remove-proxy OK")

	case d["--reinit-proxy"].(bool):

		switch {

		default:

			token := t.parseProxyToken(d)

			log.Debugf("call rpc reinit-proxy to dashboard %s", t.addr)
			if err := c.ReinitProxy(token); err != nil {
				log.PanicErrorf(err, "call rpc reinit-proxy to dashboard %s failed", t.addr)
			}
			log.Debugf("call rpc reinit-proxy OK")

		case d["--all"].(bool):

			log.Debugf("call rpc stats to dashboard %s", t.addr)
			s, err := c.Stats()
			if err != nil {
				log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.addr)
			}
			log.Debugf("call rpc stats OK")

			for _, p := range s.Proxy.Models {
				fmt.Printf("%s\n", p.Encode())
				log.Debugf("call rpc reinit-proxy to dashboard %s", t.addr)
				if err := c.ReinitProxy(p.Token); err != nil {
					log.PanicErrorf(err, "call rpc reinit-proxy to dashboard %s failed", t.addr)
				}
				log.Debugf("call rpc reinit-proxy OK")
			}

		}

	}
}

func (t *cmdDashboard) handleGroupCommand(d map[string]interface{}) {
	c := t.newTopomClient()

	switch {

	case d["--create-group"].(bool):

		gid := utils.ArgumentIntegerMust(d, "--gid")

		log.Debugf("call rpc create-group to dashboard %s", t.addr)
		if err := c.CreateGroup(gid); err != nil {
			log.PanicErrorf(err, "call rpc create-group to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc create-group OK")

	case d["--remove-group"].(bool):

		gid := utils.ArgumentIntegerMust(d, "--gid")

		log.Debugf("call rpc remove-group to dashboard %s", t.addr)
		if err := c.RemoveGroup(gid); err != nil {
			log.PanicErrorf(err, "call rpc remove-group to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc remove-group OK")

	case d["--group-add"].(bool):

		gid, addr := utils.ArgumentIntegerMust(d, "--gid"), utils.ArgumentMust(d, "--addr")

		log.Debugf("call rpc group-add-server to dashboard %s", t.addr)
		if err := c.GroupAddServer(gid, addr); err != nil {
			log.PanicErrorf(err, "call rpc group-add-server to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc group-add-server OK")

	case d["--group-del"].(bool):

		gid, addr := utils.ArgumentIntegerMust(d, "--gid"), utils.ArgumentMust(d, "--addr")

		log.Debugf("call rpc group-del-server to dashboard %s", t.addr)
		if err := c.GroupDelServer(gid, addr); err != nil {
			log.PanicErrorf(err, "call rpc group-del-server to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc group-del-server OK")

	case d["--promote-server"].(bool):

		gid, addr := utils.ArgumentIntegerMust(d, "--gid"), utils.ArgumentMust(d, "--addr")

		log.Debugf("call rpc group-promote-server to dashboard %s", t.addr)
		if err := c.GroupPromoteServer(gid, addr); err != nil {
			log.PanicErrorf(err, "call rpc group-promote-server to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc group-promote-server OK")

		fallthrough

	case d["--promote-commit"].(bool):

		gid := utils.ArgumentIntegerMust(d, "--gid")

		log.Debugf("call rpc group-promote-commit to dashboard %s", t.addr)
		if err := c.GroupPromoteCommit(gid); err != nil {
			log.PanicErrorf(err, "call rpc group-promote-commit to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc group-promote-commit OK")

	case d["--group-status"].(bool):

		log.Debugf("call rpc stats to dashboard %s", t.addr)
		s, err := c.Stats()
		if err != nil {
			log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc stats OK")

		var format string
		var wgid int
		for _, g := range s.Group.Models {
			wgid = utils.MaxInt(wgid, len(strconv.Itoa(g.Id)))
		}
		format += fmt.Sprintf("group-%%0%dd", wgid)

		var widx int
		for _, g := range s.Group.Models {
			for i, _ := range g.Servers {
				widx = utils.MaxInt(widx, len(strconv.Itoa(i)))
			}
		}
		format += fmt.Sprintf(" index-%%0%dd", widx)

		var waddr int
		for _, g := range s.Group.Models {
			for _, x := range g.Servers {
				waddr = utils.MaxInt(waddr, len(x.Addr))
			}
		}
		format += fmt.Sprintf(" %%-%ds", waddr)

		for _, g := range s.Group.Models {
			for i, x := range g.Servers {
				var addr = x.Addr
				switch stats := s.Group.Stats[addr]; {
				case stats == nil:
					fmt.Printf("[?] "+format, g.Id, i, addr)
				case stats.Error != nil:
					fmt.Printf("[E] "+format, g.Id, i, addr)
				case stats.Timeout || stats.Stats == nil:
					fmt.Printf("[T] "+format, g.Id, i, addr)
				default:
					s1 := stats.Stats["master_addr"]
					s2 := stats.Stats["master_link_status"]
					master := s1 + ":" + s2
					if master == ":" {
						master = "NO:ONE"
					}
					expect := "NO:ONE"
					if i != 0 {
						expect = g.Servers[0].Addr + ":up"
					}
					if master == expect {
						fmt.Printf("[ ] "+format, g.Id, i, addr)
					} else {
						fmt.Printf("[X] "+format, g.Id, i, addr)
					}
					fmt.Printf("      ==> %s", master)
				}
				fmt.Println()
			}
		}
	}
}

func (t *cmdDashboard) handleSyncActionCommand(d map[string]interface{}) {
	c := t.newTopomClient()

	switch {

	case d["--create"].(bool):

		addr := utils.ArgumentMust(d, "--addr")

		log.Debugf("call rpc create-sync-action to dashboard %s", t.addr)
		if err := c.SyncCreateAction(addr); err != nil {
			log.PanicErrorf(err, "call rpc create-sync-action to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc create-sync-action OK")

	case d["--remove"].(bool):

		addr := utils.ArgumentMust(d, "--addr")

		log.Debugf("call rpc remove-sync-action to dashboard %s", t.addr)
		if err := c.SyncRemoveAction(addr); err != nil {
			log.PanicErrorf(err, "call rpc remove-sync-action to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc remove-sync-action OK")

	}

}

func (t *cmdDashboard) handleSlotActionCommand(d map[string]interface{}) {
	c := t.newTopomClient()

	switch {

	case d["--create"].(bool):

		sid := utils.ArgumentIntegerMust(d, "--sid")
		gid := utils.ArgumentIntegerMust(d, "--gid")

		log.Debugf("call rpc create-slot-action to dashboard %s", t.addr)
		if err := c.SlotCreateAction(sid, gid); err != nil {
			log.PanicErrorf(err, "call rpc create-slot-action to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc create-slot-action OK")

	case d["--remove"].(bool):

		sid := utils.ArgumentIntegerMust(d, "--sid")

		log.Debugf("call rpc remove-slot-action to dashboard %s", t.addr)
		if err := c.SlotRemoveAction(sid); err != nil {
			log.PanicErrorf(err, "call rpc remove-slot-action to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc remove-slot-action OK")

	case d["--create-range"].(bool):

		beg := utils.ArgumentIntegerMust(d, "--beg")
		end := utils.ArgumentIntegerMust(d, "--end")
		gid := utils.ArgumentIntegerMust(d, "--gid")

		log.Debugf("call rpc create-slot-action-range to dashboard %s", t.addr)
		if err := c.SlotCreateActionRange(beg, end, gid); err != nil {
			log.PanicErrorf(err, "call rpc create-slot-action-range to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc create-slot-action-range OK")

	case d["--interval"] != nil:

		value := utils.ArgumentIntegerMust(d, "--interval")

		log.Debugf("call rpc slot-action-interval to dashboard %s", t.addr)
		if err := c.SetSlotActionInterval(value); err != nil {
			log.PanicErrorf(err, "call rpc slot-action-interval to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc slot-action-interval OK")

	case d["--disabled"] != nil:

		value := utils.ArgumentIntegerMust(d, "--disabled")

		log.Debugf("call rpc slot-action-disabled to dashboard %s", t.addr)
		if err := c.SetSlotActionDisabled(value != 0); err != nil {
			log.PanicErrorf(err, "call rpc slot-action-disabled to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc slot-action-disabled OK")

	}
}
