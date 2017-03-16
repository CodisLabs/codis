// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/topom"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/math2"
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
	case d["--reload"].(bool):
		t.handleReload(d)
	case d["--log-level"] != nil:
		t.handleLogLevel(d)

	case d["--slots-assign"].(bool):
		fallthrough
	case d["--slots-status"].(bool):
		t.handleSlotsCommand(d)

	case d["--create-proxy"].(bool):
		fallthrough
	case d["--online-proxy"].(bool):
		fallthrough
	case d["--remove-proxy"].(bool):
		fallthrough
	case d["--reinit-proxy"].(bool):
		fallthrough
	case d["--proxy-status"].(bool):
		t.handleProxyCommand(d)

	case d["--create-group"].(bool):
		fallthrough
	case d["--remove-group"].(bool):
		fallthrough
	case d["--resync-group"].(bool):
		fallthrough
	case d["--group-add"].(bool):
		fallthrough
	case d["--group-del"].(bool):
		fallthrough
	case d["--group-status"].(bool):
		fallthrough
	case d["--replica-groups"].(bool):
		fallthrough
	case d["--promote-server"].(bool):
		t.handleGroupCommand(d)

	case d["--sentinel-add"].(bool):
		fallthrough
	case d["--sentinel-del"].(bool):
		fallthrough
	case d["--sentinel-resync"].(bool):
		t.handleSentinelCommand(d)

	case d["--sync-action"].(bool):
		t.handleSyncActionCommand(d)

	case d["--slot-action"].(bool):
		t.handleSlotActionCommand(d)

	case d["--rebalance"].(bool):
		t.handleSlotRebalance(d)

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

func (t *cmdDashboard) handleReload(d map[string]interface{}) {
	c := t.newTopomClient()

	log.Debugf("call rpc reload to dashboard %s", t.addr)
	if err := c.Reload(); err != nil {
		log.PanicErrorf(err, "call rpc reload to dashboard %s failed", t.addr)
	}
	log.Debugf("call rpc reload OK")
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

	case d["--slots-assign"].(bool) && d["--offline"].(bool):

		beg := utils.ArgumentIntegerMust(d, "--beg")
		end := utils.ArgumentIntegerMust(d, "--end")

		slots := []*models.SlotMapping{}
		for i := beg; i <= end; i++ {
			slots = append(slots, &models.SlotMapping{
				Id: i,
			})
		}

		if !d["--confirm"].(bool) {
			b, err := json.MarshalIndent(slots, "", "    ")
			if err != nil {
				log.PanicErrorf(err, "json marshal failed")
			}
			fmt.Println(string(b))
			return
		}

		log.Debugf("call rpc slots-assign to dashboard %s", t.addr)
		if err := c.SlotsAssignOffline(slots); err != nil {
			log.PanicErrorf(err, "call rpc slots-assign to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc slots-assign OK")

	case d["--slots-assign"].(bool) && !d["--offline"].(bool):

		beg := utils.ArgumentIntegerMust(d, "--beg")
		end := utils.ArgumentIntegerMust(d, "--end")
		gid := utils.ArgumentIntegerMust(d, "--gid")

		slots := []*models.SlotMapping{}
		for i := beg; i <= end; i++ {
			slots = append(slots, &models.SlotMapping{
				Id: i, GroupId: gid,
			})
		}

		if !d["--confirm"].(bool) {
			b, err := json.MarshalIndent(slots, "", "    ")
			if err != nil {
				log.PanicErrorf(err, "json marshal failed")
			}
			fmt.Println(string(b))
			return
		}

		log.Debugf("call rpc slots-assign to dashboard %s", t.addr)
		if err := c.SlotsAssignGroup(slots); err != nil {
			log.PanicErrorf(err, "call rpc slots-assign to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc slots-assign OK")

	}
}

func (t *cmdDashboard) parseProxyTokens(d map[string]interface{}) []string {
	switch {

	default:

		log.Panicf("can't find specific proxy")

		return nil

	case d["--token"] != nil:

		return []string{utils.ArgumentMust(d, "--token")}

	case d["--pid"] != nil:

		pid := utils.ArgumentIntegerMust(d, "--pid")

		c := t.newTopomClient()

		log.Debugf("call rpc stats to dashboard %s", t.addr)
		s, err := c.Stats()
		if err != nil {
			log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc stats OK")

		var tokens []string

		for _, p := range s.Proxy.Models {
			if p.Id == pid {
				tokens = append(tokens, p.Token)
			}
		}

		if len(tokens) != 0 {
			return tokens
		}

		if !d["--force"].(bool) {
			log.Panicf("can't find specific proxy with id = %d", pid)
		}
		return nil

	case d["--addr"] != nil:

		addr := utils.ArgumentMust(d, "--addr")

		c := t.newTopomClient()

		log.Debugf("call rpc stats to dashboard %s", t.addr)
		s, err := c.Stats()
		if err != nil {
			log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc stats OK")

		var tokens []string

		for _, p := range s.Proxy.Models {
			if p.AdminAddr == addr {
				tokens = append(tokens, p.Token)
			}
		}

		if len(tokens) != 0 {
			return tokens
		}

		if !d["--force"].(bool) {
			log.Panicf("can't find specific proxy with addr = %s", addr)
		}
		return nil

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

	case d["--online-proxy"].(bool):

		addr := utils.ArgumentMust(d, "--addr")

		log.Debugf("call rpc online-proxy to dashboard %s", t.addr)
		if err := c.OnlineProxy(addr); err != nil {
			log.PanicErrorf(err, "call rpc online-proxy to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc online-proxy OK")

	case d["--remove-proxy"].(bool):

		force := d["--force"].(bool)

		for _, token := range t.parseProxyTokens(d) {
			log.Debugf("call rpc remove-proxy to dashboard %s", t.addr)
			if err := c.RemoveProxy(token, force); err != nil {
				log.PanicErrorf(err, "call rpc remove-proxy to dashboard %s failed", t.addr)
			}
			log.Debugf("call rpc remove-proxy OK")
		}

	case d["--reinit-proxy"].(bool):

		switch {

		default:

			for _, token := range t.parseProxyTokens(d) {
				log.Debugf("call rpc reinit-proxy to dashboard %s", t.addr)
				if err := c.ReinitProxy(token); err != nil {
					log.PanicErrorf(err, "call rpc reinit-proxy to dashboard %s failed", t.addr)
				}
				log.Debugf("call rpc reinit-proxy OK")
			}

		case d["--all"].(bool):

			log.Debugf("call rpc stats to dashboard %s", t.addr)
			s, err := c.Stats()
			if err != nil {
				log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.addr)
			}
			log.Debugf("call rpc stats OK")

			for _, p := range s.Proxy.Models {
				fmt.Printf("reinit proxy: %s\n", p.Encode())
				log.Debugf("call rpc reinit-proxy to dashboard %s", t.addr)
				if err := c.ReinitProxy(p.Token); err != nil {
					log.PanicErrorf(err, "call rpc reinit-proxy to dashboard %s failed", t.addr)
				}
				log.Debugf("call rpc reinit-proxy OK")
			}

		}

	case d["--proxy-status"].(bool):

		log.Debugf("call rpc stats to dashboard %s", t.addr)
		s, err := c.Stats()
		if err != nil {
			log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc stats OK")

		var format string
		var wpid int
		for _, p := range s.Proxy.Models {
			wpid = math2.MaxInt(wpid, len(strconv.Itoa(p.Id)))
		}
		format += fmt.Sprintf("proxy-%%0%dd [T] %%s", wpid)

		var waddr1, waddr2 int
		for _, p := range s.Proxy.Models {
			waddr1 = math2.MaxInt(waddr1, len(p.AdminAddr))
			waddr2 = math2.MaxInt(waddr2, len(p.ProxyAddr))
		}
		format += fmt.Sprintf(" [A] %%-%ds", waddr1)
		format += fmt.Sprintf(" [P] %%-%ds", waddr2)

		for _, p := range s.Proxy.Models {
			var xfmt string
			switch stats := s.Proxy.Stats[p.Token]; {
			case stats == nil:
				xfmt = "[?] " + format
			case stats.Error != nil:
				xfmt = "[E] " + format
			case stats.Timeout || stats.Stats == nil:
				xfmt = "[T] " + format
			default:
				xfmt = "[ ] " + format
			}
			fmt.Printf(xfmt, p.Id, p.Token, p.AdminAddr, p.ProxyAddr)
			fmt.Println()
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

	case d["--resync-group"].(bool):

		switch {

		case d["--all"].(bool):

			stats, err := c.Stats()
			if err != nil {
				log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.addr)
			}
			log.Debugf("call rpc stats OK")

			for _, g := range stats.Group.Models {
				log.Debugf("call rpc resync-group [%d] to dashboard %s", g.Id, t.addr)
				if err := c.ResyncGroup(g.Id); err != nil {
					log.PanicErrorf(err, "call rpc resync-group to dashboard %s failed", t.addr)
				}
			}
			log.Debugf("call rpc resync-group OK")

		default:

			gid := utils.ArgumentIntegerMust(d, "--gid")

			log.Debugf("call rpc resync-group to dashboard %s", t.addr)
			if err := c.ResyncGroup(gid); err != nil {
				log.PanicErrorf(err, "call rpc resync-group to dashboard %s failed", t.addr)
			}
			log.Debugf("call rpc resync-group OK")

		}

	case d["--group-add"].(bool):

		gid, addr := utils.ArgumentIntegerMust(d, "--gid"), utils.ArgumentMust(d, "--addr")
		dc, _ := utils.Argument(d, "--datacenter")

		log.Debugf("call rpc group-add-server to dashboard %s", t.addr)
		if err := c.GroupAddServer(gid, dc, addr); err != nil {
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

	case d["--replica-groups"].(bool):

		gid, addr := utils.ArgumentIntegerMust(d, "--gid"), utils.ArgumentMust(d, "--addr")
		value := d["--enable"].(bool)

		log.Debugf("call rpc replica-groups to dashboard %s", t.addr)
		if err := c.EnableReplicaGroups(gid, addr, value); err != nil {
			log.PanicErrorf(err, "call rpc replica-groups to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc replica-groups to dashboard OK")

	case d["--promote-server"].(bool):

		gid, addr := utils.ArgumentIntegerMust(d, "--gid"), utils.ArgumentMust(d, "--addr")

		log.Debugf("call rpc group-promote-server to dashboard %s", t.addr)
		if err := c.GroupPromoteServer(gid, addr); err != nil {
			log.PanicErrorf(err, "call rpc group-promote-server to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc group-promote-server OK")

		fallthrough

	case d["--group-status"].(bool):

		log.Debugf("call rpc stats to dashboard %s", t.addr)
		s, err := c.Stats()
		if err != nil {
			log.PanicErrorf(err, "call rpc stats to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc stats OK")

		var format string
		var wgid, widx int
		for _, g := range s.Group.Models {
			wgid = math2.MaxInt(wgid, len(strconv.Itoa(g.Id)))
			for i, _ := range g.Servers {
				widx = math2.MaxInt(widx, len(strconv.Itoa(i)))
			}
		}
		format += fmt.Sprintf("group-%%0%dd [%%0%dd]", wgid, widx)

		var waddr int
		for _, g := range s.Group.Models {
			for _, x := range g.Servers {
				waddr = math2.MaxInt(waddr, len(x.Addr))
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
					var master string
					if s, ok := stats.Stats["master_addr"]; ok {
						master = s + ":" + stats.Stats["master_link_status"]
					} else {
						master = "NO:ONE"
					}
					var expect string
					if i == 0 {
						expect = "NO:ONE"
					} else {
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

func (t *cmdDashboard) handleSentinelCommand(d map[string]interface{}) {
	c := t.newTopomClient()

	switch {

	case d["--sentinel-add"].(bool):

		addr := utils.ArgumentMust(d, "--addr")

		log.Debugf("call rpc add-sentinel to dashboard %s", t.addr)
		if err := c.AddSentinel(addr); err != nil {
			log.PanicErrorf(err, "call rpc add-sentinel to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc add-sentinel OK")

	case d["--sentinel-del"].(bool):

		addr := utils.ArgumentMust(d, "--addr")

		force := d["--force"].(bool)

		log.Debugf("call rpc del-sentinel to dashboard %s", t.addr)
		if err := c.DelSentinel(addr, force); err != nil {
			log.PanicErrorf(err, "call rpc del-sentinel to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc del-sentinel OK")

	case d["--sentinel-resync"].(bool):

		log.Debugf("call rpc resync-sentinels to dashboard %s", t.addr)
		if err := c.ResyncSentinels(); err != nil {
			log.PanicErrorf(err, "call rpc resync-sentinels to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc resync-sentinels OK")

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

	case d["--create-some"].(bool):

		src := utils.ArgumentIntegerMust(d, "--gid-from")
		dst := utils.ArgumentIntegerMust(d, "--gid-to")
		num := utils.ArgumentIntegerMust(d, "--num-slots")

		log.Debugf("call rpc create-slot-action-some to dashboard %s", t.addr)
		if err := c.SlotCreateActionSome(src, dst, num); err != nil {
			log.PanicErrorf(err, "call rpc create-slot-action-some to dashboard %s failed", t.addr)
		}
		log.Debugf("call rpc create-slot-action-some OK")

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

func (t *cmdDashboard) handleSlotRebalance(d map[string]interface{}) {
	c := t.newTopomClient()

	confirm := d["--confirm"].(bool)

	log.Debugf("call rpc slot-rebalance to dashboard %s", t.addr)
	plans, err := c.SlotsRebalance(confirm)
	if err != nil {
		log.PanicErrorf(err, "call rpc slot-rebalance to dashboard %s failed", t.addr)
	}
	log.Debugf("call rpc slot-rebalance OK")

	if len(plans) == 0 {
		fmt.Println("nothing changes")
	} else {
		var slotIds = make([]int, 0, len(plans))
		for sid := range plans {
			slotIds = append(slotIds, sid)
		}
		sort.Ints(slotIds)

		var gid, beg, end = -1, 0, -1
		for _, sid := range slotIds {
			if beg <= end {
				if sid == end+1 && plans[sid] == gid {
					end = sid
					continue
				}
				fmt.Printf("[%04d,%04d] => %d\n", beg, end, gid)
			}
			beg, end, gid = sid, sid, plans[sid]
		}
		if beg <= end {
			fmt.Printf("[%04d,%04d] => %d\n", beg, end, gid)
		}
		fmt.Println("done")
	}
}
