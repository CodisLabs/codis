package topom

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/assert"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

func openTopom() *Topom {
	config := newTopomConfig()

	t, err := NewWithConfig(newMemStore(), config)
	assert.MustNoError(err)
	return t
}

func newTopomConfig() *Config {
	config := NewDefaultConfig()
	config.AdminAddr = "0.0.0.0:0"
	config.ProductName = "topom_test"
	config.ProductAuth = "topom_auth"
	return config
}

func openProxy() (*proxy.Proxy, *proxy.ApiClient, string) {
	config := newProxyConfig()

	s, err := proxy.New(config)
	assert.MustNoError(err)

	c := proxy.NewApiClient(s.GetModel().AdminAddr)
	c.SetXAuth(config.ProductName, config.ProductAuth, s.GetToken())
	return s, c, s.GetModel().AdminAddr
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
	defer store.Close()

	config := newTopomConfig()

	t1, err := NewWithConfig(store, config)
	assert.Must(err == nil)

	_, err = NewWithConfig(store, config)
	assert.Must(err != nil)

	t1.Close()

	t2, err := NewWithConfig(store, config)
	assert.Must(err == nil)

	t2.Close()
}

func TestProxyCreate(x *testing.T) {
	t := openTopom()
	defer t.Close()

	_, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.Must(t.CreateProxy(addr1) == nil)
	assert.Must(t.CreateProxy(addr1) != nil)
	assert.Must(len(t.ListProxy()) == 1)

	_, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.Must(c2.Shutdown() == nil)

	assert.Must(t.CreateProxy(addr2) != nil)
	assert.Must(len(t.ListProxy()) == 1)

	errs1, err1 := t.XPingAll(false)
	assert.Must(err1 == nil && len(errs1) == 0)

	assert.Must(c1.Shutdown() == nil)

	errs2, err2 := t.XPingAll(false)
	assert.Must(err2 == nil && len(errs2) == 1)
}

func TestProxyRemove(x *testing.T) {
	t := openTopom()
	defer t.Close()

	p1, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.Must(t.CreateProxy(addr1) == nil)
	assert.Must(len(t.ListProxy()) == 1)

	assert.Must(t.RemoveProxy(p1.GetToken(), false) == nil)
	assert.Must(len(t.ListProxy()) == 0)

	p2, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.Must(t.CreateProxy(addr2) == nil)
	assert.Must(len(t.ListProxy()) == 1)

	assert.Must(c2.Shutdown() == nil)

	assert.Must(t.RemoveProxy(p2.GetToken(), false) != nil)
	assert.Must(t.RemoveProxy(p2.GetToken(), true) == nil)
	assert.Must(len(t.ListProxy()) == 0)
}

func assertGroupList(t *Topom, glist ...*models.Group) {
	var m = make(map[int]*models.Group)
	for _, x := range t.ListGroup() {
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

func TestGroupTest1(x *testing.T) {
	t := openTopom()
	defer t.Close()

	assert.Must(t.CreateGroup(0) != nil)
	assert.Must(t.CreateGroup(1) == nil)
	assert.Must(t.CreateGroup(1) != nil)
	assertGroupList(t,
		&models.Group{
			Id:      1,
			Servers: []string{},
		})

	var server0 = "server0:19000"
	var server1 = "server1:19000"
	var server2 = "server2:19000"

	assert.Must(t.GroupAddNewServer(0, "") != nil)
	assert.Must(t.GroupAddNewServer(1, server0) == nil)
	assert.Must(t.GroupAddNewServer(1, server1) == nil)
	assert.Must(t.GroupAddNewServer(1, server1) != nil)
	assertGroupList(t,
		&models.Group{
			Id:      1,
			Servers: []string{server0, server1},
		})

	assert.Must(t.GroupRemoveServer(1, server0) != nil)
	assert.Must(t.GroupRemoveServer(1, server2) != nil)
	assert.Must(t.GroupRemoveServer(1, server1) == nil)
	assertGroupList(t,
		&models.Group{
			Id:      1,
			Servers: []string{server0},
		})

	assert.Must(t.CreateGroup(2) == nil)
	assert.Must(t.GroupAddNewServer(2, server0) != nil)
	assertGroupList(t,
		&models.Group{
			Id:      1,
			Servers: []string{server0},
		},
		&models.Group{
			Id:      2,
			Servers: []string{},
		})

	assert.Must(t.RemoveGroup(0) != nil)
	assert.Must(t.RemoveGroup(1) == nil)
	assert.Must(t.RemoveGroup(1) != nil)

	assert.Must(t.GroupAddNewServer(2, server0) == nil)
	assertGroupList(t,
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
	assert.Must(t.GroupAddNewServer(1, server0) == nil)
	assert.Must(t.GroupAddNewServer(1, server1) == nil)
	assertGroupList(t,
		&models.Group{
			Id:      1,
			Servers: []string{server0, server1},
		})

	assert.Must(t.GroupPromoteServer(1, server0) != nil)
	assert.Must(t.GroupPromoteServer(1, server2) != nil)
	assert.Must(t.GroupPromoteServer(1, server1) == nil)
	assert.Must(t.GroupPromoteServer(1, server1) != nil)
	assertGroupList(t,
		&models.Group{
			Id:        1,
			Servers:   []string{server1, server0},
			Promoting: true,
		})
	assert.Must(t.GroupRemoveServer(1, server0) != nil)

	assert.Must(t.GroupPromoteCommit(0) != nil)
	assert.Must(t.GroupPromoteCommit(1) == nil)
	assert.Must(t.GroupRemoveServer(1, server0) == nil)
	assert.Must(t.GroupAddNewServer(1, server2) == nil)
	assertGroupList(t,
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
	assertGroupList(t,
		&models.Group{
			Id:        1,
			Servers:   []string{server2, server1},
			Promoting: true,
		})

	assert.Must(t.GroupPromoteCommit(1) != nil)
	assert.Must(t.RemoveProxy(p1.GetToken(), true) == nil)
	assert.Must(t.GroupPromoteCommit(1) == nil)
	assertGroupList(t,
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
	assert.Must(t.GroupAddNewServer(1, server0) == nil)
	assert.Must(t.SlotCreateAction(1, 1) == nil)

	assert.Must(t.CreateGroup(2) == nil)
	assert.Must(t.GroupAddNewServer(2, server1) == nil)
	assert.Must(t.GroupAddNewServer(2, server2) == nil)
	assert.Must(t.SlotCreateAction(2, 2) == nil)

	assert.Must(t.RemoveGroup(2) != nil)
	assert.Must(t.SlotRemoveAction(2) == nil)

	assert.Must(t.GroupRemoveServer(2, server2) == nil)
	assert.Must(t.GroupRemoveServer(2, server1) == nil)
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

func assertSlotsList(t *Topom, clients []*proxy.ApiClient, slots ...*models.Slot) {
	verifySlotsList(t.GetSlots(), slots)
	for _, c := range clients {
		sum, err := c.Summary()
		assert.Must(err == nil)
		verifySlotsList(sum.Slots, slots)
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
	assert.Must(t.GroupAddNewServer(1, server0) == nil)
	assert.Must(t.GroupAddNewServer(2, server1) == nil)
	assert.Must(t.GroupAddNewServer(3, server2) == nil)

	assert.Must(t.SlotCreateAction(1, 0) != nil)
	assert.Must(t.SlotCreateAction(1, 1) == nil)
	assert.Must(t.SlotCreateAction(1, 2) != nil)

	assert.Must(t.SlotRemoveAction(2) != nil)
	assert.Must(t.SlotCreateAction(2, 1) == nil)
	assert.Must(t.SlotCreateAction(2, 2) != nil)
	assert.Must(t.SlotRemoveAction(2) == nil)

	assert.Must(t.prepareAction(2) != nil)
	assert.Must(t.SlotCreateAction(2, 3) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id: 2,
		})

	assert.Must(t.completeAction(2) != nil)

	assert.Must(t.prepareAction(2) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server2,
			MigrateFrom: "",
		})

	assert.Must(t.completeAction(2) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			BackendAddr: server2,
		})

	assert.Must(t.SlotCreateAction(2, 2) == nil)
	assert.Must(t.prepareAction(2) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(t.completeAction(2) == nil)
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
	assert.Must(t.GroupAddNewServer(1, server0) == nil)
	assert.Must(t.GroupAddNewServer(2, server1) == nil)
	assert.Must(t.GroupAddNewServer(3, server2) == nil)

	_, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.Must(t.CreateProxy(addr1) == nil)

	assert.Must(t.SlotCreateAction(1, 1) == nil)

	assert.Must(t.SlotCreateAction(2, 3) == nil)
	assert.Must(t.prepareAction(2) == nil)
	assert.Must(t.completeAction(2) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1},
		&models.Slot{
			Id:          2,
			BackendAddr: server2,
		})

	assert.Must(t.SlotCreateAction(2, 2) == nil)
	assert.Must(t.prepareAction(2) == nil)
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
	assert.Must(t.completeAction(2) != nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(t.prepareAction(2) != nil)
	assert.Must(t.RemoveProxy(p2.GetToken(), true) == nil)
	assert.Must(t.prepareAction(2) == nil)
	assert.Must(t.completeAction(2) == nil)
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
	assert.Must(t.GroupAddNewServer(1, server0) == nil)
	assert.Must(t.GroupAddNewServer(2, server1) == nil)
	assert.Must(t.GroupAddNewServer(3, server2) == nil)

	_, c1, addr1 := openProxy()
	defer c1.Shutdown()

	p2, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.Must(t.CreateProxy(addr1) == nil)
	assert.Must(t.CreateProxy(addr2) == nil)

	assert.Must(t.SlotCreateAction(1, 1) == nil)

	assert.Must(t.SlotCreateAction(2, 3) == nil)
	assert.Must(t.prepareAction(2) == nil)
	assert.Must(t.completeAction(2) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1, c2},
		&models.Slot{
			Id:          2,
			BackendAddr: server2,
		})

	assert.Must(c2.Shutdown() == nil)

	assert.Must(t.SlotCreateAction(2, 2) == nil)
	assert.Must(t.prepareAction(2) != nil)
	assertSlotsList(t, []*proxy.ApiClient{c1},
		&models.Slot{
			Id:          2,
			Locked:      true,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	p3, c3, addr3 := openProxy()
	defer c3.Shutdown()

	assert.Must(t.CreateProxy(addr3) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1, c3},
		&models.Slot{
			Id:          2,
			Locked:      true,
			BackendAddr: server1,
			MigrateFrom: server2,
		})
	assert.Must(t.RemoveProxy(p2.GetToken(), true) == nil)

	assert.Must(t.prepareAction(2) == nil)
	assertSlotsList(t, []*proxy.ApiClient{c1, c3},
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(c3.Shutdown() == nil)
	assert.Must(t.completeAction(2) != nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			Locked:      false,
			BackendAddr: server1,
			MigrateFrom: server2,
		})

	assert.Must(t.RemoveProxy(p3.GetToken(), true) == nil)
	assert.Must(t.prepareAction(2) == nil)
	assert.Must(t.completeAction(2) == nil)
	assertSlotsList(t, nil,
		&models.Slot{
			Id:          2,
			BackendAddr: server1,
		})
}

type memStore struct {
	mu sync.Mutex

	data map[string][]byte
}

var (
	ErrNodeExists    = errors.New("node already exists")
	ErrNodeNotExists = errors.New("node does not exist")
)

func newMemStore() *memStore {
	return &memStore{data: make(map[string][]byte)}
}

func (s *memStore) Acquire(name string, topom *models.Topom) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data["meta"]; ok {
		return errors.Trace(ErrNodeExists)
	}

	s.data["meta"] = topom.Encode()
	return nil
}

func (s *memStore) Release() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data["meta"]; !ok {
		return errors.Trace(ErrNodeNotExists)
	}

	delete(s.data, "meta")
	return nil
}

func (s *memStore) LoadSlotMapping(slotId int) (*models.SlotMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("slot-%04d", slotId)
	var m = &models.SlotMapping{}

	if b, ok := s.data[k]; ok {
		if err := json.Unmarshal(b, m); err != nil {
			return nil, errors.Trace(err)
		}
	} else {
		m.Id = slotId
	}
	return m, nil
}

func (s *memStore) SaveSlotMapping(slotId int, slot *models.SlotMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("slot-%04d", slotId)

	s.data[k] = slot.Encode()
	return nil
}

func (s *memStore) ListProxy() ([]*models.Proxy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var plist []*models.Proxy

	for k, b := range s.data {
		if strings.HasPrefix(k, "proxy-") {
			var p = &models.Proxy{}
			if err := json.Unmarshal(b, p); err != nil {
				return nil, errors.Trace(err)
			}
			plist = append(plist, p)
		}
	}
	return plist, nil
}

func (s *memStore) CreateProxy(proxyId int, proxy *models.Proxy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("proxy-%d", proxyId)

	if _, ok := s.data[k]; ok {
		return errors.Trace(ErrNodeExists)
	}

	s.data[k] = proxy.Encode()
	return nil
}

func (s *memStore) RemoveProxy(proxyId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("proxy-%d", proxyId)

	if _, ok := s.data[k]; !ok {
		return errors.Trace(ErrNodeNotExists)
	}

	delete(s.data, k)
	return nil
}

func (s *memStore) ListGroup() ([]*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var glist []*models.Group

	for k, b := range s.data {
		if strings.HasPrefix(k, "group-") {
			var g = &models.Group{}
			if err := json.Unmarshal(b, g); err != nil {
				return nil, errors.Trace(err)
			}
			glist = append(glist, g)
		}
	}
	return glist, nil
}

func (s *memStore) CreateGroup(groupId int, group *models.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("group-%d", groupId)

	if _, ok := s.data[k]; ok {
		return errors.Trace(ErrNodeExists)
	}

	s.data[k] = group.Encode()
	return nil
}

func (s *memStore) RemoveGroup(groupId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("group-%d", groupId)

	if _, ok := s.data[k]; !ok {
		return errors.Trace(ErrNodeNotExists)
	}

	delete(s.data, k)
	return nil
}

func (s *memStore) UpdateGroup(groupId int, group *models.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("group-%d", groupId)

	if _, ok := s.data[k]; !ok {
		return errors.Trace(ErrNodeNotExists)
	}

	s.data[k] = group.Encode()
	return nil
}

func (s *memStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return nil
}
