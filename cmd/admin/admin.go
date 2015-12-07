// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/models/etcd"
	"github.com/wandoulabs/codis/pkg/models/zk"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
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
	case d["--config-convert"].(bool):
		t.handleConfigConvert(d)
	case d["--config-restore"].(bool):
		t.handleConfigRestore(d)
	}
}

func (t *cmdAdmin) newTopomClient(d map[string]interface{}) models.Client {
	switch {
	case d["--zookeeper"] != nil:

		addr := utils.ArgumentMust(d, "--zookeeper")

		c, err := zkclient.New(addr, time.Minute)
		if err != nil {
			log.PanicErrorf(err, "create zkclient to %s failed", addr)
		}
		return c

	case d["--etcd"] != nil:

		addr := utils.ArgumentMust(d, "--etcd")

		c, err := etcdclient.New(addr, time.Minute)
		if err != nil {
			log.PanicErrorf(err, "create etcdclient to %s failed", addr)
		}
		return c

	default:
		log.Panicf("nil client for topom")
		return nil
	}
}

func (t *cmdAdmin) newTopomStore(d map[string]interface{}) *models.Store {
	if !utils.IsValidProduct(t.product) {
		log.Panicf("invalid product = %s", t.product)
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

func (t *cmdAdmin) newZooKeeperClient(d map[string]interface{}) models.Client {
	client, err := zkclient.NewWithLogfunc(d["--zookeeper"].(string), time.Second*5, func(format string, v ...interface{}) {
		log.Debugf("zookeeper - %s", fmt.Sprintf(format, v...))
	})
	if err != nil {
		log.PanicErrorf(err, "create zookeeper client to failed")
	}
	return client
}

type ConfigV3 struct {
	Slots []*models.SlotMapping `json:"slots,omitempty"`
	Group []*models.Group       `json:"group,omitempty"`
	Proxy []*models.Proxy       `json:"proxy,omitempty"`
}

func (t *cmdAdmin) loadAndDecode(client models.Client, path string, v interface{}) {
	b, err := client.Read(path)
	if err != nil {
		log.PanicErrorf(err, "load path = %s failed", path)
	}
	if err := json.Unmarshal(b, v); err != nil {
		log.PanicErrorf(err, "decode path = %s failed", path)
	}
	log.Debugf("load & decode path = %s", path)
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
	files, err := client.List(path)
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
	b, err := client.Read(path)
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

func (t *cmdAdmin) loadJsonConfigV1(d map[string]interface{}) map[string]interface{} {
	b, err := ioutil.ReadFile(utils.ArgumentMust(d, "--input"))
	if err != nil {
		log.PanicErrorf(err, "read file failed")
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

	cfg1 := t.loadJsonConfigV1(d)
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

func (t *cmdAdmin) loadJsonConfigV3(d map[string]interface{}) *ConfigV3 {
	b, err := ioutil.ReadFile(utils.ArgumentMust(d, "--input"))
	if err != nil {
		log.PanicErrorf(err, "read file failed")
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

	config := t.loadJsonConfigV3(d)

	if err := store.Acquire(&models.Topom{}); err != nil {
		log.PanicErrorf(err, "acquire store lock failed")
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

	if err := store.Release(); err != nil {
		log.PanicErrorf(err, "release store lock failed")
	}
}
