package main

import (
	"encoding/json"
	"fmt"
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

	if !utils.IsValidName(t.product.name) {
		log.Panicf("invalid product name")
	}

	log.Debugf("args.product.name = %s", t.product.name)

	switch {
	case d["--remove-lock"].(bool):
		t.handleRemoveLock(d)
	case d["--config-dump"].(bool):
		t.handleConfigDump(d)
	}
}

func (t *cmdSuperAdmin) newTopomStore(d map[string]interface{}) models.Store {
	switch {
	case d["--zookeeper"] != nil:
		s, err := zkstore.NewStore(d["--zookeeper"].(string), t.product.name)
		if err != nil {
			log.PanicErrorf(err, "create zkstore failed")
		}
		return s
	case d["--etcd"] != nil:
		s, err := etcdstore.NewStore(d["--etcd"].(string), t.product.name)
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
