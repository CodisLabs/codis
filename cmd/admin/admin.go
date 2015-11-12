// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/models/etcd"
	"github.com/wandoulabs/codis/pkg/models/zk"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type cmdAdmin struct {
	product struct {
		name string
	}
}

func (t *cmdAdmin) Main(d map[string]interface{}) {
	t.product.name, _ = utils.Argument(d, "--product-name")

	switch {
	case d["--config-convert"].(bool):
		t.handleConfigConvert(d)

	default:
		if !utils.IsValidProduct(t.product.name) {
			log.Panicf("invalid product name = %s", t.product.name)
		}
		log.Debugf("args.product.name = %s", t.product.name)

		switch {
		case d["--remove-lock"].(bool):
			t.handleRemoveLock(d)
		case d["--config-dump"].(bool):
			t.handleConfigDump(d)
		case d["--config-restore"].(bool):
			t.handleConfigRestore(d)
		}
	}
}

func (t *cmdAdmin) newTopomClient(d map[string]interface{}) models.Client {
	switch {
	case d["--zookeeper"] != nil:
		addr := utils.ArgumentMust(d, "--zookeeper")
		c, err := zkclient.New(addr, time.Minute)
		if err != nil {
			log.PanicErrorf(err, "create zk client to %s failed", addr)
		}
		return c

	case d["--etcd"] != nil:
		addr := utils.ArgumentMust(d, "--etcd")
		c, err := etcdclient.New(addr, time.Minute)
		if err != nil {
			log.PanicErrorf(err, "create etcd client to %s failed", addr)
		}
		return c

	default:
		log.Panicf("nil client for topom")
		return nil
	}
}

func (t *cmdAdmin) newTopomStore(d map[string]interface{}) *models.Store {
	client := t.newTopomClient(d)
	return models.NewStore(client, t.product.name)
}

func (t *cmdAdmin) handleRemoveLock(d map[string]interface{}) {
	store := t.newTopomStore(d)
	defer store.Close()

	log.Debugf("force remove-lock")
	if err := store.Release(true); err != nil {
		log.PanicErrorf(err, "force remove-lock failed")
	}
	log.Debugf("force remove-lock OK")
}

func (t *cmdAdmin) handleConfigDump(d map[string]interface{}) {
	switch {
	case d["-1"].(bool):
		t.dumpConfigV1(d)
	default:
		fallthrough
	case d["-2"].(bool):
		t.dumpConfigV2(d)
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

type ConfigV2 struct {
	Slots []*models.SlotMapping `json:"slots,omitempty"`
	Group []*models.Group       `json:"group,omitempty"`
	Proxy []*models.Proxy       `json:"proxy,omitempty"`
	Topom *models.Topom         `json:"topom,omitempty"`
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

	prefix := filepath.Join("/zk/codis", fmt.Sprintf("db_%s", t.product.name))
	log.Debugf("prefix = %s", prefix)

	config := make(map[string]interface{})

	dirs, err := client.List(prefix)
	if err != nil {
		log.PanicErrorf(err, "list path = %s failed", prefix)
	}
	if len(dirs) == 0 {
		log.Panicf("no such product = %s [v1]", t.product.name)
	}
	for _, dir := range dirs {
		config[filepath.Base(dir)] = t.dumpConfigV1Recursively(client, dir)
	}

	b, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}

func (t *cmdAdmin) dumpConfigV1Recursively(client models.Client, path string) interface{} {
	log.Debugf("dump path = %s", path)
	if plist, err := client.List(path); err != nil {
		log.PanicErrorf(err, "list path = %s failed", path)
	} else if plist != nil {
		var m = make(map[string]interface{})
		for _, path := range plist {
			m[filepath.Base(path)] = t.dumpConfigV1Recursively(client, path)
		}
		return m
	}
	b, err := client.Read(path)
	if err != nil {
		log.PanicErrorf(err, "dump path = %s failed", path)
	}
	if len(b) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		log.PanicErrorf(err, "json unmarshal failed")
	}
	return v
}

func (t *cmdAdmin) dumpConfigV2(d map[string]interface{}) {
	client := t.newTopomClient(d)
	defer client.Close()

	prefix := filepath.Join("/codis2", t.product.name)
	log.Debugf("prefix = %s", prefix)

	config := &ConfigV2{}

	dirs, err := client.List(prefix)
	if err != nil {
		log.PanicErrorf(err, "list path = %s failed", prefix)
	}
	if len(dirs) == 0 {
		log.Panicf("no such product = %s [v2]", t.product.name)
	}

	if plist, err := client.List(filepath.Join(prefix, "slots")); err != nil {
		log.PanicErrorf(err, "list slots failed")
	} else {
		sort.Sort(sort.StringSlice(plist))
		for _, path := range plist {
			s := &models.SlotMapping{}
			t.loadAndDecode(client, path, s)
			config.Slots = append(config.Slots, s)
		}
	}

	if plist, err := client.List(filepath.Join(prefix, "group")); err != nil {
		log.PanicErrorf(err, "list group failed")
	} else {
		sort.Sort(sort.StringSlice(plist))
		for _, path := range plist {
			g := &models.Group{}
			t.loadAndDecode(client, path, g)
			config.Group = append(config.Group, g)
		}
	}

	if plist, err := client.List(filepath.Join(prefix, "proxy")); err != nil {
		log.PanicErrorf(err, "list proxy failed")
	} else {
		sort.Sort(sort.StringSlice(plist))
		for _, path := range plist {
			p := &models.Proxy{}
			t.loadAndDecode(client, path, p)
			config.Proxy = append(config.Proxy, p)
		}
	}

	if b, err := client.Read(filepath.Join(prefix, "topom")); err != nil {
		log.PanicErrorf(err, "load topom failed")
	} else if b != nil {
		t := &models.Topom{}
		if err := json.Unmarshal(b, t); err != nil {
			log.PanicErrorf(err, "decode topom failed")
		}
		config.Topom = t
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

func (t *cmdAdmin) convertSlotsV1(smap map[int]*models.SlotMapping, v interface{}) {
	m := v.(map[string]interface{})
	slotId := int(m["id"].(float64))
	status := m["state"].(map[string]interface{})["status"].(string)
	log.Debugf("found slot-%04d status = %s", slotId, status)
	if status != "online" {
		if status == "offline" {
			return
		}
		log.Panicf("invalid slot status")
	}
	groupId := int(m["group_id"].(float64))
	if smap[slotId] != nil {
		log.Panicf("slot-%04d already exists", slotId)
	}
	smap[slotId] = &models.SlotMapping{
		Id: slotId, GroupId: groupId,
	}
}

func (t *cmdAdmin) convertGroupV1(gmap map[int]*models.Group, v interface{}) {
	m := v.(map[string]interface{})
	addr := m["addr"].(string)
	groupId := int(m["group_id"].(float64))
	isSlave := m["type"].(string) != "master"
	log.Debugf("found group-%04d %s slave = %t", groupId, addr, isSlave)
	if groupId <= 0 || groupId > models.MaxGroupId {
		log.Panicf("invalid group = %d", groupId)
	}
	g := gmap[groupId]
	if g == nil {
		g = &models.Group{Id: groupId}
		gmap[groupId] = g
	}
	if isSlave {
		g.Servers = append(g.Servers, addr)
	} else {
		g.Servers = append([]string{addr}, g.Servers...)
	}
}

func (t *cmdAdmin) handleConfigConvert(d map[string]interface{}) {
	defer func() {
		if x := recover(); x != nil {
			log.Panicf("convert config failed: %+v", x)
		}
	}()

	cfg1 := t.loadJsonConfigV1(d)
	cfg2 := &ConfigV2{}

	if slots := cfg1["slots"]; slots != nil {
		smap := make(map[int]*models.SlotMapping)
		for _, v := range slots.(map[string]interface{}) {
			t.convertSlotsV1(smap, v)
		}
		for _, s := range smap {
			cfg2.Slots = append(cfg2.Slots, s)
		}
		models.SortSlots(cfg2.Slots, func(s1, s2 *models.SlotMapping) bool {
			return s1.Id < s2.Id
		})
	}

	if servers := cfg1["servers"]; servers != nil {
		gmap := make(map[int]*models.Group)
		for _, g := range servers.(map[string]interface{}) {
			for _, v := range g.(map[string]interface{}) {
				t.convertGroupV1(gmap, v)
			}
		}
		for _, g := range gmap {
			cfg2.Group = append(cfg2.Group, g)
		}
		models.SortGroup(cfg2.Group, func(g1, g2 *models.Group) bool {
			return g1.Id < g2.Id
		})
	}

	b, err := json.MarshalIndent(cfg2, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}

func (t *cmdAdmin) loadJsonConfigV2(d map[string]interface{}) *ConfigV2 {
	b, err := ioutil.ReadFile(utils.ArgumentMust(d, "--input"))
	if err != nil {
		log.PanicErrorf(err, "read file failed")
	}
	config := &ConfigV2{}
	if err := json.Unmarshal(b, config); err != nil {
		log.PanicErrorf(err, "json unmarshal failed")
	}

	var pmap = make(map[int]*models.Proxy)
	for _, p := range config.Proxy {
		if pmap[p.Id] != nil {
			log.Panicf("proxy-%04d already exists", p.Id)
		}
		pmap[p.Id] = p
	}

	var gmap = make(map[int]*models.Group)
	for _, g := range config.Group {
		if g.Id <= 0 || g.Id > models.MaxGroupId {
			log.Panicf("invalid group id = %d", g.Id)
		}
		if gmap[g.Id] != nil {
			log.Panicf("group-%04d already exists", g.Id)
		}
		if g.Promoting {
			log.Panicf("gorup-%04d is promoting", g.Id)
		}
		gmap[g.Id] = g
	}

	var xmap = make(map[string]bool)
	for _, g := range gmap {
		for _, x := range g.Servers {
			if xmap[x] {
				log.Panicf("server %s already exists", x)
			}
			xmap[x] = true
		}
	}

	var smap = make(map[int]*models.SlotMapping)
	for _, s := range config.Slots {
		if s.Id < 0 || s.Id >= models.MaxSlotNum {
			log.Panicf("invalid slot id = %d", s.Id)
		}
		if smap[s.Id] != nil {
			log.Panicf("slot-%04d already exists", s.Id)
		}
		if s.Action.State != "" || s.Action.Index != 0 || s.Action.TargetId != 0 {
			log.Panicf("slot-%04d action is not empty", s.Id)
		}
		if g := gmap[s.GroupId]; g == nil || len(g.Servers) == 0 {
			log.Panicf("slot-%04d with group-%04d doesn't exist or empty", s.Id, s.GroupId)
		}
		smap[s.Id] = s
	}

	return config
}

func (t *cmdAdmin) handleConfigRestore(d map[string]interface{}) {
	config := t.loadJsonConfigV2(d)

	store := t.newTopomStore(d)
	defer store.Close()

	if err := store.Acquire(&models.Topom{}); err != nil {
		log.PanicErrorf(err, "acquire store lock failed")
	}

	if plist, err := store.ListProxy(); err != nil {
		log.PanicErrorf(err, "list proxy failed")
	} else if len(plist) != 0 {
		log.Panicf("list of proxy is not empty")
	}

	if glist, err := store.ListGroup(); err != nil {
		log.PanicErrorf(err, "list group failed")
	} else if len(glist) != 0 {
		log.Panicf("list of group is not empty")
	}

	for _, s := range config.Slots {
		if err := store.SaveSlotMapping(s.Id, s); err != nil {
			log.PanicErrorf(err, "save slot-%04d failed", s.Id)
		}
		log.Debugf("update slot-%04d OK", s.Id)
	}

	for _, g := range config.Group {
		if err := store.CreateGroup(g.Id, g); err != nil {
			log.PanicErrorf(err, "create group-%04d failed", g.Id)
		}
		log.Debugf("create group-%04d OK", g.Id)
	}

	for _, p := range config.Proxy {
		if err := store.CreateProxy(p.Id, p); err != nil {
			log.PanicErrorf(err, "create proxy-%04d failed", p.Id)
		}
		log.Debugf("create proxy-%04d OK", p.Id)
	}

	if err := store.Release(false); err != nil {
		log.PanicErrorf(err, "release store lock failed")
	}
}
