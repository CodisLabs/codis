package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"sort"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/models/store/etcd"
	"github.com/wandoulabs/codis/pkg/models/store/zk"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type cmdSuperAdmin struct {
	product struct {
		name string
	}
}

func (t *cmdSuperAdmin) Main(d map[string]interface{}) {
	if s, ok := d["--product-name"].(string); ok {
		t.product.name = s
	}

	switch {
	case d["--config-convert"].(bool):
	default:
		if !utils.IsValidName(t.product.name) {
			log.Panicf("invalid product name")
		}
		log.Debugf("args.product.name = %s", t.product.name)
	}

	switch {
	case d["--remove-lock"].(bool):
		t.handleRemoveLock(d)
	case d["--config-dump"].(bool):
		t.handleConfigDump(d)
	case d["--config-convert"].(bool):
		t.handleConfigConvert(d)
	}
}

func (t *cmdSuperAdmin) parseString(d map[string]interface{}, name string) string {
	if s, ok := d[name].(string); ok && s != "" {
		log.Debugf("parse %s = %s", name, s)
		return s
	}
	log.Panicf("parse argument %s failed, not found or blank string", name)
	return ""
}

func (t *cmdSuperAdmin) newTopomStore(d map[string]interface{}) models.Store {
	switch {
	case d["--zookeeper"] != nil:
		addr := t.parseString(d, "--zookeeper")
		s, err := zkstore.NewStore(addr, t.product.name)
		if err != nil {
			log.PanicErrorf(err, "create zkstore failed")
		}
		return s
	case d["--etcd"] != nil:
		addr := t.parseString(d, "--etcd")
		s, err := etcdstore.NewStore(addr, t.product.name)
		if err != nil {
			log.PanicErrorf(err, "create etcdstore failed")
		}
		return s
	}

	log.Panicf("nil store for topom")
	return nil
}

func (t *cmdSuperAdmin) handleRemoveLock(d map[string]interface{}) {
	store := t.newTopomStore(d)
	defer store.Close()

	log.Debugf("force remove-lock")
	if err := store.Release(true); err != nil {
		log.PanicErrorf(err, "force remove-lock failed")
	}
	log.Debugf("force remove-lock OK")
}

func (t *cmdSuperAdmin) handleConfigDump(d map[string]interface{}) {
	switch {
	case d["--zookeeper"] != nil:
		switch {
		case d["-1"].(bool):
			t.dumpConfigZooKeeperV1(d)
		default:
			fallthrough
		case d["-2"].(bool):
			t.dumpConfigZooKeeperV2(d)
		}
	case d["--etcd"] != nil:
		log.Panicf("not implement yet")
	}
}

func (t *cmdSuperAdmin) newZooKeeperClient(d map[string]interface{}) *zkstore.ZkClient {
	client, err := zkstore.NewClientWithLogfunc(d["--zookeeper"].(string), time.Second*5, func(format string, v ...interface{}) {
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

func (t *cmdSuperAdmin) loadAndDecodeZooKeeper(client *zkstore.ZkClient, path string, v interface{}) {
	b, err := client.LoadData(path)
	if err != nil {
		log.PanicErrorf(err, "load path = %s failed", path)
	}
	fmt.Println(string(b))
	if err := json.Unmarshal(b, v); err != nil {
		log.PanicErrorf(err, "decode path = %s failed", path)
	}
	log.Debugf("load & decode path = %s", path)
}

func (t *cmdSuperAdmin) dumpConfigZooKeeperV1(d map[string]interface{}) {
	client := t.newZooKeeperClient(d)
	defer client.Close()

	prefix := filepath.Join("/zk/codis", fmt.Sprintf("db_%s", t.product.name))
	log.Debugf("prefix = %s", prefix)

	config := make(map[string]interface{})

	dirs, err := client.ListFile(prefix)
	if err != nil {
		log.PanicErrorf(err, "list path = %s failed", prefix)
	}
	if len(dirs) == 0 {
		log.Panicf("no such product = %s [v1]", t.product.name)
	}
	for _, dir := range dirs {
		config[filepath.Base(dir)] = t.dumpConfigZooKeeperV1Recursively(client, dir)
	}

	b, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}

func (t *cmdSuperAdmin) dumpConfigZooKeeperV1Recursively(client *zkstore.ZkClient, path string) interface{} {
	log.Debugf("dump path = %s", path)
	if plist, err := client.ListFile(path); err != nil {
		log.PanicErrorf(err, "list path = %s failed", path)
	} else if plist != nil {
		var m = make(map[string]interface{})
		for _, path := range plist {
			m[filepath.Base(path)] = t.dumpConfigZooKeeperV1Recursively(client, path)
		}
		return m
	}
	b, err := client.LoadData(path)
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

func (t *cmdSuperAdmin) dumpConfigZooKeeperV2(d map[string]interface{}) {
	client := t.newZooKeeperClient(d)
	defer client.Close()

	prefix := filepath.Join("/zk/codis2", t.product.name)
	log.Debugf("prefix = %s", prefix)

	config := &ConfigV2{}

	dirs, err := client.ListFile(prefix)
	if err != nil {
		log.PanicErrorf(err, "list path = %s failed", prefix)
	}
	if len(dirs) == 0 {
		log.Panicf("no such product = %s [v2]", t.product.name)
	}

	if plist, err := client.ListFile(filepath.Join(prefix, "slots")); err != nil {
		log.PanicErrorf(err, "list slots failed")
	} else {
		sort.Sort(sort.StringSlice(plist))
		for _, path := range plist {
			s := &models.SlotMapping{}
			t.loadAndDecodeZooKeeper(client, path, s)
			config.Slots = append(config.Slots, s)
		}
	}

	if plist, err := client.ListFile(filepath.Join(prefix, "group")); err != nil {
		log.PanicErrorf(err, "list group failed")
	} else {
		sort.Sort(sort.StringSlice(plist))
		for _, path := range plist {
			g := &models.Group{}
			t.loadAndDecodeZooKeeper(client, path, g)
			config.Group = append(config.Group, g)
		}
	}

	if plist, err := client.ListFile(filepath.Join(prefix, "proxy")); err != nil {
		log.PanicErrorf(err, "list proxy failed")
	} else {
		sort.Sort(sort.StringSlice(plist))
		for _, path := range plist {
			p := &models.Proxy{}
			t.loadAndDecodeZooKeeper(client, path, p)
			config.Proxy = append(config.Proxy, p)
		}
	}

	if b, err := client.LoadData(filepath.Join(prefix, "topom")); err != nil {
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

func (t *cmdSuperAdmin) loadJsonConfigV1(d map[string]interface{}) map[string]interface{} {
	b, err := ioutil.ReadFile(t.parseString(d, "--input"))
	if err != nil {
		log.PanicErrorf(err, "read file failed")
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		log.PanicErrorf(err, "json unmarshal failed")
	}
	return v.(map[string]interface{})
}

func (t *cmdSuperAdmin) convertSlotsV1(slots map[int]*models.SlotMapping, v interface{}) {
	submap := v.(map[string]interface{})
	slotId := int(submap["id"].(float64))
	status := submap["state"].(map[string]interface{})["status"].(string)
	log.Debugf("found slot-%04d status = %s", slotId, status)
	if status != "online" {
		if status == "offline" {
			return
		}
		log.Panicf("invalid slot status")
	}
	groupId := int(submap["group_id"].(float64))
	if slots[slotId] != nil {
		log.Panicf("slot-%04d already exists", slotId)
	}
	slots[slotId] = &models.SlotMapping{
		Id: slotId, GroupId: groupId,
	}
}

func (t *cmdSuperAdmin) convertGroupV1(groups map[int]*models.Group, v interface{}) {
	submap := v.(map[string]interface{})
	addr := submap["addr"].(string)
	groupId := int(submap["group_id"].(float64))
	isSlave := submap["type"].(string) != "master"
	log.Debugf("found group-%04d %s slave = %t", groupId, addr, isSlave)
	if groupId <= 0 || groupId > math.MaxInt16 {
		log.Panicf("invalid group = %d", groupId)
	}
	g := groups[groupId]
	if g == nil {
		g = &models.Group{Id: groupId}
		groups[groupId] = g
	}
	if isSlave {
		g.Servers = append(g.Servers, addr)
	} else {
		g.Servers = append([]string{addr}, g.Servers...)
	}
}

func (t *cmdSuperAdmin) handleConfigConvert(d map[string]interface{}) {
	defer func() {
		if x := recover(); x != nil {
			log.Panicf("convert config failed: %+v", x)
		}
	}()

	cfg1 := t.loadJsonConfigV1(d)
	cfg2 := &ConfigV2{}

	if slots := cfg1["slots"]; slots != nil {
		mappings := make(map[int]*models.SlotMapping)
		for _, v := range slots.(map[string]interface{}) {
			t.convertSlotsV1(mappings, v)
		}
		for _, slot := range mappings {
			cfg2.Slots = append(cfg2.Slots, slot)
		}
		models.SortSlots(cfg2.Slots, func(s1, s2 *models.SlotMapping) bool {
			return s1.Id < s2.Id
		})
	}

	if servers := cfg1["servers"]; servers != nil {
		groups := make(map[int]*models.Group)
		for _, g := range servers.(map[string]interface{}) {
			for _, v := range g.(map[string]interface{}) {
				t.convertGroupV1(groups, v)
			}
		}
		for _, group := range groups {
			cfg2.Group = append(cfg2.Group, group)
		}
		models.SortGroup(cfg2.Group, func(g1, g2 *models.Group) bool {
			return g1.Id < g2.Id
		})
		for _, s := range cfg2.Slots {
			if groups[s.GroupId] == nil {
				log.Panicf("cann't find group-%04d for slot-%04d", s.GroupId, s.Id)
			}
		}
		var addrs = make(map[string]int)
		for _, g := range cfg2.Group {
			for _, x := range g.Servers {
				if _, ok := addrs[x]; ok {
					log.Panicf("server %s already exists", x)
				} else {
					addrs[x] = g.Id
				}
			}
		}
	}

	b, err := json.MarshalIndent(cfg2, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}
