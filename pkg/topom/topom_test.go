package topom_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/topom"
	"github.com/wandoulabs/codis/pkg/utils/assert"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

func openTopom() *topom.Topom {
	config := newTopomConfig()

	t, err := topom.NewWithConfig(newMemStore(), config)
	assert.MustNoError(err)
	return t
}

func newTopomConfig() *topom.Config {
	config := topom.NewDefaultConfig()
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

	err := t.Close()
	assert.MustNoError(err)

	assert.Must(!t.IsOnline() && t.IsClosed())
}

func TestTopomExclusive(x *testing.T) {
	store := newMemStore()
	defer store.Close()

	config := newTopomConfig()

	t1, err := topom.NewWithConfig(store, config)
	assert.MustNoError(err)

	_, err = topom.NewWithConfig(store, config)
	assert.Must(err != nil)

	t1.Close()

	t2, err := topom.NewWithConfig(store, config)
	assert.MustNoError(err)

	t2.Close()
}

func TestProxyCreate(x *testing.T) {
	t := openTopom()
	defer t.Close()

	_, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.MustNoError(t.CreateProxy(addr1))
	assert.Must(t.CreateProxy(addr1) != nil)
	assert.Must(len(t.ListProxy()) == 1)

	_, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.MustNoError(c2.Shutdown())

	assert.Must(t.CreateProxy(addr2) != nil)
	assert.Must(len(t.ListProxy()) == 1)

	errs1, err1 := t.XPingAll(false)
	assert.MustNoError(err1)
	assert.Must(len(errs1) == 0)

	assert.MustNoError(c1.Shutdown())

	errs2, err2 := t.XPingAll(false)
	assert.MustNoError(err2)
	assert.Must(len(errs2) == 1)
}

func TestProxyRemove(x *testing.T) {
	t := openTopom()
	defer t.Close()

	p1, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.MustNoError(t.CreateProxy(addr1))
	assert.Must(len(t.ListProxy()) == 1)

	assert.MustNoError(t.RemoveProxy(p1.GetToken(), false))
	assert.Must(len(t.ListProxy()) == 0)

	p2, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.MustNoError(t.CreateProxy(addr2))
	assert.Must(len(t.ListProxy()) == 1)

	assert.MustNoError(c2.Shutdown())

	assert.Must(t.RemoveProxy(p2.GetToken(), false) != nil)
	assert.MustNoError(t.RemoveProxy(p2.GetToken(), true))
	assert.Must(len(t.ListProxy()) == 0)
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
