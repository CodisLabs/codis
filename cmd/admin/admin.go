// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

type cmdAdmin struct {
	product string
}

func (t *cmdAdmin) Main(d map[string]interface{}) {
	t.product, _ = d["--product"].(string)

	switch {
	case d["--remove-lock"].(bool):
		t.handleRemoveLock(d)
	case d["--config-dump"].(bool):
		t.handleConfigDump(d)
	case d["--config-convert"] != nil:
		t.handleConfigConvert(d)
	case d["--config-restore"] != nil:
		t.handleConfigRestore(d)
	case d["--dashboard-list"].(bool):
		t.handleDashboardList(d)
	}
}

func (t *cmdAdmin) newTopomClient(d map[string]interface{}) models.Client {
	var coordinator struct {
		name string
		addr string
		auth string
	}

	switch {
	case d["--zookeeper"] != nil:
		coordinator.name = "zookeeper"
		coordinator.addr = utils.ArgumentMust(d, "--zookeeper")
		if d["--zookeeper-auth"] != nil {
			coordinator.auth = utils.ArgumentMust(d, "--zookeeper-auth")
		}

	case d["--etcd"] != nil:
		coordinator.name = "etcd"
		coordinator.addr = utils.ArgumentMust(d, "--etcd")
		if d["--etcd-auth"] != nil {
			coordinator.auth = utils.ArgumentMust(d, "--etcd-auth")
		}

	case d["--filesystem"] != nil:
		coordinator.name = "filesystem"
		coordinator.addr = utils.ArgumentMust(d, "--filesystem")

	default:
		log.Panicf("invalid coordinator")
	}

	c, err := models.NewClient(coordinator.name, coordinator.addr, coordinator.auth, time.Minute)
	if err != nil {
		log.PanicErrorf(err, "create '%s' client to '%s' failed", coordinator.name, coordinator.addr)
	}
	return c
}

func (t *cmdAdmin) newTopomStore(d map[string]interface{}) *models.Store {
	if err := models.ValidateProduct(t.product); err != nil {
		log.PanicErrorf(err, "invalid product name")
	}
	client := t.newTopomClient(d)
	return models.NewStore(client, t.product)
}

func (t *cmdAdmin) handleRemoveLock(d map[string]interface{}) {
	store := t.newTopomStore(d)
	defer store.Close()

	log.Debugf("force remove-lock")
	if err := store.Release(); err != nil {
		log.PanicErrorf(err, "force remove-lock failed")
	}
	log.Debugf("force remove-lock OK")
}

func (t *cmdAdmin) handleConfigDump(d map[string]interface{}) {
	switch {
	case d["-1"].(bool):
		t.dumpConfigV1(d)
	default:
		t.dumpConfigV3(d)
	}
}

type ConfigV3 struct {
	Slots []*models.SlotMapping `json:"slots,omitempty"`
	Group []*models.Group       `json:"group,omitempty"`
	Proxy []*models.Proxy       `json:"proxy,omitempty"`
}

func (t *cmdAdmin) dumpConfigV1(d map[string]interface{}) {
	client := t.newTopomClient(d)
	defer client.Close()

	prefix := filepath.Join("/zk/codis", fmt.Sprintf("db_%s", t.product))
	config := t.dumpConfigV1Recursively(client, prefix)
	if m, ok := config.(map[string]interface{}); !ok || m == nil {
		log.Panicf("cann't find product = %s [v1]", t.product)
	}
	b, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}

func (t *cmdAdmin) dumpConfigV1Recursively(client models.Client, path string) interface{} {
	files, err := client.List(path, false)
	if err != nil {
		log.PanicErrorf(err, "list path = %s failed", path)
	}
	if len(files) != 0 {
		var m = make(map[string]interface{})
		for _, path := range files {
			m[filepath.Base(path)] = t.dumpConfigV1Recursively(client, path)
		}
		return m
	}
	b, err := client.Read(path, false)
	if err != nil {
		log.PanicErrorf(err, "read file = %s failed", path)
	}
	if len(b) != 0 {
		var v interface{}
		if err := json.Unmarshal(b, &v); err != nil {
			log.PanicErrorf(err, "json unmarshal failed")
		}
		return v
	}
	return nil
}

func (t *cmdAdmin) dumpConfigV3(d map[string]interface{}) {
	store := t.newTopomStore(d)
	defer store.Close()

	group, err := store.ListGroup()
	if err != nil {
		log.PanicErrorf(err, "list group failed")
	}
	proxy, err := store.ListProxy()
	if err != nil {
		log.PanicErrorf(err, "list proxy failed")
	}

	if len(group) == 0 && len(proxy) == 0 {
		log.Panicf("cann't find product = %s [v3]", t.product)
	}

	slots, err := store.SlotMappings()
	if err != nil {
		log.PanicErrorf(err, "list slots failed")
	}

	config := &ConfigV3{
		Slots: slots,
		Group: models.SortGroup(group),
		Proxy: models.SortProxy(proxy),
	}

	b, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}

func (t *cmdAdmin) loadJsonConfigV1(file string) map[string]interface{} {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		log.PanicErrorf(err, "read file '%s' failed", file)
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		log.PanicErrorf(err, "json unmarshal failed")
	}
	return v.(map[string]interface{})
}

func (t *cmdAdmin) convertSlotsV1(slots map[int]*models.SlotMapping, v interface{}) {
	m := v.(map[string]interface{})

	var sid = int(m["id"].(float64))
	var gid = int(m["group_id"].(float64))
	var status = m["state"].(map[string]interface{})["status"].(string)

	log.Debugf("found slot-%04d status = %s", sid, status)

	switch status {
	case "online":
	case "offline":
		return
	default:
		log.Panicf("invalid slot status")
	}

	if slots[sid] != nil {
		log.Panicf("slot-%04d already exists", sid)
	}
	slots[sid] = &models.SlotMapping{
		Id: sid, GroupId: gid,
	}
}

func (t *cmdAdmin) convertGroupV1(group map[int]*models.Group, v interface{}) {
	m := v.(map[string]interface{})

	var addr = m["addr"].(string)
	var gid = int(m["group_id"].(float64))
	var master = m["type"].(string) == "master"

	log.Debugf("found group-%04d %s is master = %t", gid, addr, master)

	if gid <= 0 || gid > models.MaxGroupId {
		log.Panicf("invalid group = %d", gid)
	}

	if group[gid] == nil {
		group[gid] = &models.Group{Id: gid}
	}
	g := group[gid]

	servers := []*models.GroupServer{}
	if master {
		servers = append(servers, &models.GroupServer{Addr: addr})
		servers = append(servers, g.Servers...)
	} else {
		servers = append(servers, g.Servers...)
		servers = append(servers, &models.GroupServer{Addr: addr})
	}
	g.Servers = servers
}

func (t *cmdAdmin) handleConfigConvert(d map[string]interface{}) {
	defer func() {
		if x := recover(); x != nil {
			log.Panicf("convert config failed: %+v", x)
		}
	}()

	cfg1 := t.loadJsonConfigV1(utils.ArgumentMust(d, "--config-convert"))
	cfg2 := &ConfigV3{}

	if slots := cfg1["slots"]; slots != nil {
		temp := make(map[int]*models.SlotMapping)
		for _, v := range slots.(map[string]interface{}) {
			t.convertSlotsV1(temp, v)
		}
		for i := 0; i < models.MaxSlotNum; i++ {
			if temp[i] == nil {
				continue
			}
			cfg2.Slots = append(cfg2.Slots, temp[i])
		}
	}

	if servers := cfg1["servers"]; servers != nil {
		group := make(map[int]*models.Group)
		for _, g := range servers.(map[string]interface{}) {
			for _, v := range g.(map[string]interface{}) {
				t.convertGroupV1(group, v)
			}
		}
		cfg2.Group = models.SortGroup(group)
	}

	b, err := json.MarshalIndent(cfg2, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}

func (t *cmdAdmin) loadJsonConfigV3(file string) *ConfigV3 {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		log.PanicErrorf(err, "read file '%s' failed", file)
	}

	config := &ConfigV3{}
	if err := json.Unmarshal(b, config); err != nil {
		log.PanicErrorf(err, "json unmarshal failed")
	}

	var proxy = make(map[string]*models.Proxy)
	for _, p := range config.Proxy {
		if proxy[p.Token] != nil {
			log.Panicf("proxy-%s already exists", p.Token)
		}
		proxy[p.Token] = p
	}

	var group = make(map[int]*models.Group)
	var maddr = make(map[string]bool)
	for _, g := range config.Group {
		if g.Id <= 0 || g.Id > models.MaxGroupId {
			log.Panicf("invalid group id = %d", g.Id)
		}
		if group[g.Id] != nil {
			log.Panicf("group-%04d already exists", g.Id)
		}
		if g.Promoting.State != models.ActionNothing {
			log.Panicf("gorup-%04d is promoting", g.Id)
		}
		for _, x := range g.Servers {
			addr := x.Addr
			if maddr[addr] {
				log.Panicf("server %s already exists", addr)
			}
			maddr[addr] = true
		}
		group[g.Id] = g
	}

	var slots = make(map[int]*models.SlotMapping)
	for _, s := range config.Slots {
		if s.Id < 0 || s.Id >= models.MaxSlotNum {
			log.Panicf("invalid slot id = %d", s.Id)
		}
		if slots[s.Id] != nil {
			log.Panicf("slot-%04d already exists", s.Id)
		}
		if s.Action.State != models.ActionNothing {
			log.Panicf("slot-%04d action is not empty", s.Id)
		}
		if g := group[s.GroupId]; g == nil || len(g.Servers) == 0 {
			log.Panicf("slot-%04d with group-%04d doesn't exist or empty", s.Id, s.GroupId)
		}
		slots[s.Id] = s
	}

	return config
}

func (t *cmdAdmin) handleConfigRestore(d map[string]interface{}) {
	store := t.newTopomStore(d)
	defer store.Close()

	config := t.loadJsonConfigV3(utils.ArgumentMust(d, "--config-restore"))

	if !d["--confirm"].(bool) {
		b, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			log.PanicErrorf(err, "json marshal failed")
		}
		fmt.Println(string(b))
		return
	}

	proxy, err := store.ListProxy()
	if err != nil {
		log.PanicErrorf(err, "list proxy failed")
	}
	group, err := store.ListGroup()
	if err != nil {
		log.PanicErrorf(err, "list group failed")
	}

	if len(group) != 0 || len(proxy) != 0 {
		log.Panicf("product %s is not empty", t.product)
	}

	for _, s := range config.Slots {
		if err := store.UpdateSlotMapping(s); err != nil {
			log.PanicErrorf(err, "restore slot-%04d failed", s.Id)
		}
	}

	for _, g := range config.Group {
		if err := store.UpdateGroup(g); err != nil {
			log.PanicErrorf(err, "restore group-%04d failed", g.Id)
		}
	}

	for _, p := range config.Proxy {
		if err := store.UpdateProxy(p); err != nil {
			log.PanicErrorf(err, "restore proxy-%s failed", p.Token)
		}
	}
}

func (t *cmdAdmin) handleDashboardList(d map[string]interface{}) {
	client := t.newTopomClient(d)
	defer client.Close()

	list, err := client.List(models.CodisDir, false)
	if err != nil {
		log.PanicErrorf(err, "list products failed")
	}

	nodes := []interface{}{}

	for _, path := range list {
		var elem = &struct {
			Name      string `json:"name"`
			Dashboard string `json:"dashboard"`
		}{filepath.Base(path), ""}

		if b, err := client.Read(models.LockPath(elem.Name), false); err != nil {
			log.PanicErrorf(err, "read topom of product %s failed", elem.Name)
		} else if b != nil {
			var t = &models.Topom{}
			if err := json.Unmarshal(b, t); err != nil {
				log.PanicErrorf(err, "decode json failed")
			}
			elem.Dashboard = t.AdminAddr
		}

		nodes = append(nodes, elem)
	}

	if b, err := json.MarshalIndent(nodes, "", "    "); err != nil {
		log.PanicErrorf(err, "json encode failed")
	} else {
		fmt.Println(string(b))
	}
}
