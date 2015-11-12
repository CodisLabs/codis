// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/assert"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

var config = NewDefaultConfig()

func init() {
	config.AdminAddr = "0.0.0.0:0"
	config.ProductName = "topom_test"
	config.ProductAuth = "topom_auth"
}

func openTopom() *Topom {
	t, err := New(newMemClient(nil), config)
	assert.MustNoError(err)
	return t
}

func openProxy() (*proxy.Proxy, *proxy.ApiClient, string) {
	config := newProxyConfig()

	s, err := proxy.New(config)
	assert.MustNoError(err)

	c := proxy.NewApiClient(s.Model().AdminAddr)
	c.SetXAuth(config.ProductName, config.ProductAuth, s.Token())
	return s, c, s.Model().AdminAddr
}

func newProxyConfig() *proxy.Config {
	config := proxy.NewDefaultConfig()
	config.AdminAddr = "0.0.0.0:0"
	config.ProxyAddr = "0.0.0.0:0"
	config.ProductName = "topom_test"
	config.ProductAuth = "topom_auth"
	return config
}

func TestTopomClose(x *testing.T) {
	t := openTopom()
	assert.Must(t.IsOnline() && !t.IsClosed())

	assert.Must(t.Close() == nil)

	assert.Must(!t.IsOnline() && t.IsClosed())
}

func TestTopomExclusive(x *testing.T) {
	store := newMemStore()

	t, err := New(newMemClient(store), config)
	assert.Must(err == nil)

	_, err = New(newMemClient(store), config)
	assert.Must(err != nil)

	t.Close()

	_, err = New(newMemClient(store), config)
	assert.Must(err == nil)
}

func assertProxyStats(t *Topom, c *ApiClient, fails []string) {
	wg := t.RefreshProxyStats(time.Second)
	assert.Must(wg != nil)
	wg.Wait()

	fn := func(m map[string]*ProxyStats) {
		for _, token := range fails {
			stats := m[token]
			assert.Must(stats != nil && stats.Error != nil)
		}
		var cnt int
		for _, stats := range m {
			if stats.Error != nil {
				cnt++
			}
		}
		assert.Must(cnt == len(fails))
	}
	fn(t.Stats().Proxy.Stats)
	if c != nil {
		stats, err := c.Stats()
		assert.Must(err == nil)
		fn(stats.Proxy.Stats)
	}
}

func TestProxyCreate(x *testing.T) {
	t := openTopom()
	defer t.Close()

	p1, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.Must(t.CreateProxy(addr1) == nil)
	assert.Must(t.CreateProxy(addr1) != nil)
	assert.Must(len(t.GetProxyModels()) == 1)

	_, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.Must(c2.Shutdown() == nil)

	assert.Must(t.CreateProxy(addr2) != nil)
	assert.Must(len(t.GetProxyModels()) == 1)

	assertProxyStats(t, nil, []string{})

	assert.Must(c1.Shutdown() == nil)

	assertProxyStats(t, nil, []string{p1.Token()})
}

func TestProxyRemove(x *testing.T) {
	t := openTopom()
	defer t.Close()

	p1, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.Must(t.CreateProxy(addr1) == nil)
	assert.Must(len(t.GetProxyModels()) == 1)

	assert.Must(t.RemoveProxy(p1.Token(), false) == nil)
	assert.Must(len(t.GetProxyModels()) == 0)

	p2, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.Must(t.CreateProxy(addr2) == nil)
	assert.Must(len(t.GetProxyModels()) == 1)

	assert.Must(c2.Shutdown() == nil)

	assert.Must(t.RemoveProxy(p2.Token(), false) != nil)
	assert.Must(t.RemoveProxy(p2.Token(), true) == nil)
	assert.Must(len(t.GetProxyModels()) == 0)
}

func assertGroupList(t *Topom, c *ApiClient, glist ...*models.Group) {
	fn := func(array []*models.Group) {
		var m = make(map[int]*models.Group)
		for _, x := range array {
			m[x.Id] = x
		}
		assert.Must(len(m) == len(glist))
		for _, g := range glist {
			x := m[g.Id]
			assert.Must(x != nil)
			assert.Must(x.Promoting == g.Promoting)
			assert.Must(len(x.Servers) == len(g.Servers))
			for i := 0; i < len(x.Servers); i++ {
				assert.Must(x.Servers[i] == g.Servers[i])
			}
		}
	}
	fn(t.GetGroupModels())
	if c != nil {
		stats, err := c.Stats()
		assert.Must(err == nil)
		fn(stats.Group.Models)
	}
}

func TestGroupTest1(x *testing.T) {
	t := openTopom()
	defer t.Close()

	assert.Must(t.CreateGroup(0) != nil)
	assert.Must(t.CreateGroup(1) == nil)
	assert.Must(t.CreateGroup(1) != nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:      1,
			Servers: []string{},
		})

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(t.GroupAddServer(0, "") != nil)
	assert.Must(t.GroupAddServer(1, server0) == nil)
	assert.Must(t.GroupAddServer(1, server1) == nil)
	assert.Must(t.GroupAddServer(1, server1) != nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:      1,
			Servers: []string{server0, server1},
		})

	assert.Must(t.GroupDelServer(1, server0) != nil)
	assert.Must(t.GroupDelServer(1, server2) != nil)
	assert.Must(t.GroupDelServer(1, server1) == nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:      1,
			Servers: []string{server0},
		})

	assert.Must(t.CreateGroup(2) == nil)
	assert.Must(t.GroupAddServer(2, server0) != nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:      1,
			Servers: []string{server0},
		},
		&models.Group{
			Id:      2,
			Servers: []string{},
		})

	assert.Must(t.RemoveGroup(0) != nil)
	assert.Must(t.RemoveGroup(1) != nil)
	assert.Must(t.GroupDelServer(1, server0) == nil)
	assert.Must(t.RemoveGroup(1) == nil)
	assert.Must(t.RemoveGroup(1) != nil)

	assert.Must(t.GroupAddServer(2, server0) == nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:      2,
			Servers: []string{server0},
		})
}

func TestGroupTest2(x *testing.T) {
	t := openTopom()
	defer t.Close()

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(t.CreateGroup(1) == nil)
	assert.Must(t.GroupAddServer(1, server0) == nil)
	assert.Must(t.GroupAddServer(1, server1) == nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:      1,
			Servers: []string{server0, server1},
		})

	assert.Must(t.GroupPromoteServer(1, server0) != nil)
	assert.Must(t.GroupPromoteServer(1, server2) != nil)
	assert.Must(t.GroupPromoteServer(1, server1) == nil)
	assert.Must(t.GroupPromoteServer(1, server1) != nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:        1,
			Servers:   []string{server1, server0},
			Promoting: true,
		})
	assert.Must(t.GroupDelServer(1, server0) != nil)

	assert.Must(t.GroupPromoteCommit(0) != nil)
	assert.Must(t.GroupPromoteCommit(1) == nil)
	assert.Must(t.GroupDelServer(1, server0) == nil)
	assert.Must(t.GroupAddServer(1, server2) == nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:      1,
			Servers: []string{server1, server2},
		})

	assert.Must(t.SlotCreateAction(0, 1) == nil)

	p1, c1, addr1 := openProxy()
	defer c1.Shutdown()

	_, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.Must(t.CreateProxy(addr1) == nil)
	assert.Must(t.CreateProxy(addr2) == nil)
	assert.Must(c1.Shutdown() == nil)

	assert.Must(t.GroupPromoteServer(1, server2) == nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:        1,
			Servers:   []string{server2, server1},
			Promoting: true,
		})

	assert.Must(t.GroupPromoteCommit(1) != nil)
	assert.Must(t.RemoveProxy(p1.Token(), true) == nil)
	assert.Must(t.GroupPromoteCommit(1) == nil)
	assertGroupList(t, nil,
		&models.Group{
			Id:      1,
			Servers: []string{server2, server1},
		})
}

func TestGroupTest3(x *testing.T) {
	t := openTopom()
	defer t.Close()

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(t.CreateGroup(1) == nil)
	assert.Must(t.GroupAddServer(1, server0) == nil)
	assert.Must(t.SlotCreateAction(1, 1) == nil)

	assert.Must(t.CreateGroup(2) == nil)
	assert.Must(t.GroupAddServer(2, server1) == nil)
	assert.Must(t.GroupAddServer(2, server2) == nil)
	assert.Must(t.SlotCreateAction(2, 2) == nil)

	assert.Must(t.RemoveGroup(2) != nil)
	assert.Must(t.SlotRemoveAction(2) == nil)

	assert.Must(t.GroupDelServer(2, server2) == nil)
	assert.Must(t.GroupDelServer(2, server1) == nil)
	assert.Must(t.RemoveGroup(2) == nil)
}

func verifySlotsList(expect []*models.Slot, slots []*models.Slot) {
	var m = make(map[int]*models.Slot)
	for _, s := range expect {
		m[s.Id] = s
	}
	for _, s := range slots {
		var x = m[s.Id]
		assert.Must(x != nil)
		assert.Must(x.Locked == s.Locked)
		assert.Must(x.BackendAddr == s.BackendAddr)
		assert.Must(x.MigrateFrom == s.MigrateFrom)
	}
}

func assertSlotsList(t *Topom, clients []*proxy.ApiClient, expect ...*models.Slot) {
	verifySlotsList(t.GetSlots(), expect)
	for _, c := range clients {
		slots, err := c.Slots()
		assert.Must(err == nil)
		verifySlotsList(slots, expect)
	}
}

func TestSlotsTest1(x *testing.T) {
	t := openTopom()
	defer t.Close()

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(t.CreateGroup(1) == nil)
	assert.Must(t.CreateGroup(2) == nil)
	assert.Must(t.CreateGroup(3) == nil)
	assert.Must(t.GroupAddServer(1, server0) == nil)
	assert.Must(t.GroupAddServer(2, server1) == nil)
	assert.Must(t.GroupAddServer(3, server2) == nil)

	assert.Must(t.SlotCreateAction(1, 0) != nil)
	assert.Must(t.SlotCreateAction(1, 1) == nil)
	assert.Must(t.SlotCreateAction(1, 2) != nil)

	assert.Must(t.SlotRemoveAction(2) != nil)
	assert.Must(t.SlotCreateAction(2, 1) == nil)
	assert.Must(t.SlotCreateAction(2, 2) != nil)
	assert.Must(t.SlotRemoveAction(2) == nil)

	assert.Must(t.PrepareAction(2) != nil)
	assert.Must(t.SlotCreateAction(2, 3) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id: 2,
		})

	assert.Must(t.CompleteAction(2) != nil)

	assert.Must(t.PrepareAction(2) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server2,
			MigrateFrom: "",
		})

	assert.Must(t.CompleteAction(2) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			BackendAddr: server2,
		})

	assert.Must(t.SlotCreateAction(2, 2) == nil)
	assert.Must(t.PrepareAction(2) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(t.CompleteAction(2) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			BackendAddr: server1,
		})
}

func TestSlotTest2(x *testing.T) {
	t := openTopom()
	defer t.Close()

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(t.CreateGroup(1) == nil)
	assert.Must(t.CreateGroup(2) == nil)
	assert.Must(t.CreateGroup(3) == nil)
	assert.Must(t.GroupAddServer(1, server0) == nil)
	assert.Must(t.GroupAddServer(2, server1) == nil)
	assert.Must(t.GroupAddServer(3, server2) == nil)

	_, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.Must(t.CreateProxy(addr1) == nil)

	assert.Must(t.SlotCreateAction(1, 1) == nil)

	assert.Must(t.SlotCreateAction(2, 3) == nil)
	assert.Must(t.PrepareAction(2) == nil)
	assert.Must(t.CompleteAction(2) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1},
		&models.Slot{
			Id:          2,
			BackendAddr: server2,
		})

	assert.Must(t.SlotCreateAction(2, 2) == nil)
	assert.Must(t.PrepareAction(2) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1},
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	p2, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.Must(t.CreateProxy(addr2) == nil)

	assertSlotsList(t, []*proxy.ApiClient{c1, c2},
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(c2.Shutdown() == nil)
	assert.Must(t.CompleteAction(2) != nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(t.PrepareAction(2) != nil)
	assert.Must(t.RemoveProxy(p2.Token(), true) == nil)
	assert.Must(t.PrepareAction(2) == nil)
	assert.Must(t.CompleteAction(2) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1},
		&models.Slot{
			Id:          2,
			BackendAddr: server1,
		})
}

func TestSlotTest3(x *testing.T) {
	t := openTopom()
	defer t.Close()

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(t.CreateGroup(1) == nil)
	assert.Must(t.CreateGroup(2) == nil)
	assert.Must(t.CreateGroup(3) == nil)
	assert.Must(t.GroupAddServer(1, server0) == nil)
	assert.Must(t.GroupAddServer(2, server1) == nil)
	assert.Must(t.GroupAddServer(3, server2) == nil)

	_, c1, addr1 := openProxy()
	defer c1.Shutdown()

	p2, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.Must(t.CreateProxy(addr1) == nil)
	assert.Must(t.CreateProxy(addr2) == nil)

	assert.Must(t.SlotCreateAction(1, 1) == nil)

	assert.Must(t.SlotCreateAction(2, 3) == nil)
	assert.Must(t.PrepareAction(2) == nil)
	assert.Must(t.CompleteAction(2) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1, c2},
		&models.Slot{
			Id:          2,
			BackendAddr: server2,
		})

	assert.Must(c2.Shutdown() == nil)

	assert.Must(t.SlotCreateAction(2, 2) == nil)
	assert.Must(t.PrepareAction(2) != nil)
	assert.Must(t.RemoveProxy(p2.Token(), true) == nil)
	assert.Must(t.PrepareAction(2) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1},
		&models.Slot{
			Id:          2,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	p3, c3, addr3 := openProxy()
	defer c3.Shutdown()

	assert.Must(t.CreateProxy(addr3) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1, c3},
		&models.Slot{
			Id:          2,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(t.PrepareAction(2) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1, c3},
		&models.Slot{
			Id:          2,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(c3.Shutdown() == nil)
	assert.Must(t.CompleteAction(2) != nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(t.RemoveProxy(p3.Token(), true) == nil)
	assert.Must(t.PrepareAction(2) == nil)
	assert.Must(t.CompleteAction(2) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			BackendAddr: server1,
		})
}

func newApiClient(t *Topom) *ApiClient {
	config := t.Config()
	c := NewApiClient(t.Model().AdminAddr)
	c.SetXAuth(config.ProductName, config.ProductAuth)
	return c
}

func TestApiModel(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)

	p, err := c.Model()
	assert.Must(err == nil)
	assert.Must(p.ProductName == t.Config().ProductName)
}

func TestApiXPing(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)
	assert.Must(c.XPing() == nil)

	assert.Must(c.Shutdown() == nil)
	assert.Must(c.XPing() != nil)
}

func TestApiStats1(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(t.CreateGroup(1) == nil)
	assert.Must(t.GroupAddServer(1, server0) == nil)
	assert.Must(t.GroupAddServer(1, server1) == nil)

	assertGroupList(t, c,
		&models.Group{
			Id:      1,
			Servers: []string{server0, server1},
		})

	assert.Must(t.CreateGroup(2) == nil)
	assert.Must(t.GroupAddServer(2, server2) == nil)

	assertGroupList(t, c,
		&models.Group{
			Id:      1,
			Servers: []string{server0, server1},
		},
		&models.Group{
			Id:      2,
			Servers: []string{server2},
		})
}

func TestApiStats2(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)
	assertProxyStats(t, c, []string{})

	p1, c1, addr1 := openProxy()
	defer c1.Shutdown()
	assert.Must(t.CreateProxy(addr1) == nil)
	assert.Must(len(t.GetProxyModels()) == 1)

	p2, c2, addr2 := openProxy()
	defer c2.Shutdown()
	assert.Must(t.CreateProxy(addr2) == nil)
	assert.Must(len(t.GetProxyModels()) == 2)
	assertProxyStats(t, c, []string{})

	assert.Must(c1.Shutdown() == nil)
	assertProxyStats(t, c, []string{p1.Token()})

	assert.Must(c2.Shutdown() == nil)
	assertProxyStats(t, c, []string{p1.Token(), p2.Token()})

	assert.Must(t.RemoveProxy(p1.Token(), true) == nil)
	assert.Must(t.RemoveProxy(p2.Token(), true) == nil)
	assertProxyStats(t, c, []string{})
}

func TestApiProxy(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)

	p1, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.Must(c.CreateProxy(addr1) == nil)
	assert.Must(c.CreateProxy(addr1) != nil)
	assert.Must(c.RemoveProxy(p1.Token(), false) == nil)

	p2, c2, addr2 := openProxy()
	defer c2.Shutdown()
	assert.Must(c.ReinitProxy(p2.Token()) != nil)
	assert.Must(c.CreateProxy(addr2) == nil)
	assert.Must(c.ReinitProxy(p2.Token()) == nil)

	assert.Must(c2.Shutdown() == nil)
	assert.Must(c.ReinitProxy(p2.Token()) != nil)
}

func TestApiGroup(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)

	assert.Must(c.CreateGroup(0) != nil)
	assert.Must(c.CreateGroup(math.MaxInt32) != nil)

	assert.Must(c.CreateGroup(1) == nil)
	assert.Must(c.CreateGroup(1) != nil)

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(c.GroupAddServer(2, server0) != nil)
	assert.Must(c.GroupAddServer(1, server0) == nil)
	assert.Must(c.GroupAddServer(1, server1) == nil)
	assert.Must(c.GroupDelServer(1, server2) != nil)

	assertGroupList(t, c,
		&models.Group{
			Id:      1,
			Servers: []string{server0, server1},
		})
}

func TestApiGroupPromote(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)

	assert.Must(c.CreateGroup(1) == nil)

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(c.GroupAddServer(1, server0) == nil)
	assert.Must(c.GroupAddServer(1, server1) == nil)

	assert.Must(c.GroupPromoteServer(1, server2) != nil)
	assert.Must(c.GroupPromoteServer(1, server0) != nil)

	assert.Must(c.GroupPromoteServer(1, server1) == nil)

	assertGroupList(t, c,
		&models.Group{
			Id:        1,
			Servers:   []string{server1, server0},
			Promoting: true,
		})

	assert.Must(c.GroupAddServer(1, server2) != nil)
	assert.Must(c.GroupPromoteCommit(1) == nil)

	assertGroupList(t, c,
		&models.Group{
			Id:      1,
			Servers: []string{server1, server0},
		})

	assert.Must(c.GroupDelServer(1, server1) != nil)
	assert.Must(c.GroupDelServer(1, server0) == nil)
	assert.Must(c.GroupDelServer(1, server1) == nil)
}

func TestApiAction(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)

	var server0 = "server0:19000"

	assert.Must(c.CreateGroup(1) == nil)
	assert.Must(c.SlotCreateAction(0, 1) != nil)

	assert.Must(c.GroupAddServer(1, server0) == nil)
	assert.Must(c.SlotCreateAction(0, 1) == nil)

	assert.Must(c.SlotCreateAction(1, 1) == nil)
	assert.Must(c.SlotRemoveAction(1) == nil)

	assert.Must(c.SlotCreateAction(1, 1) == nil)
	assert.Must(t.PrepareAction(1) == nil)
	assert.Must(c.SlotRemoveAction(1) != nil)
	assert.Must(c.SlotCreateAction(1, 1) != nil)

	assert.Must(t.CompleteAction(1) == nil)
	assert.Must(c.SlotRemoveAction(1) != nil)
}

type memStore struct {
	sync.Mutex
	data map[string][]byte
}

func newMemStore() *memStore {
	return &memStore{data: make(map[string][]byte)}
}

type memClient struct {
	*memStore
}

func newMemClient(store *memStore) models.Client {
	if store == nil {
		store = newMemStore()
	}
	return &memClient{store}
}

func (c *memClient) Create(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.data[path]; ok {
		return errors.Errorf("node already exists")
	}
	c.data[path] = data
	return nil
}

func (c *memClient) Update(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	c.data[path] = data
	return nil
}

func (c *memClient) Delete(path string) error {
	c.Lock()
	defer c.Unlock()
	delete(c.data, path)
	return nil
}

func (c *memClient) Read(path string) ([]byte, error) {
	c.Lock()
	defer c.Unlock()
	return c.data[path], nil
}

func (c *memClient) List(path string) ([]string, error) {
	c.Lock()
	defer c.Unlock()
	path = filepath.Clean(path)
	var list []string
	for k, _ := range c.data {
		if path == filepath.Dir(k) {
			list = append(list, k)
		}
	}
	return list, nil
}

func (c *memClient) Close() error {
	c.Lock()
	defer c.Unlock()
	return nil
}
