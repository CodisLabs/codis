package topom

import (
	"path/filepath"
	"testing"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/assert"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

func TestSlotsCache(x *testing.T) {
	t := openTopom()
	defer t.Close()

	verify := func(sid, gid int) {
		ctx, err := t.newContext()
		assert.MustNoError(err)
		m, err := ctx.getSlotMapping(sid)
		assert.MustNoError(err)
		assert.Must(m.Id == sid && m.GroupId == gid)
	}

	m := &models.SlotMapping{Id: 0}
	verify(m.Id, 0)

	t.dirtySlotsCache(m.Id)
	m.GroupId = 100
	assert.MustNoError(t.storeUpdateSlotMapping(m))
	verify(m.Id, 100)

	t.dirtySlotsCache(m.Id)
	m.GroupId = 200
	verify(m.Id, 100)

	t.dirtyCacheAll()
	m.GroupId = 300
	assert.MustNoError(t.storeUpdateSlotMapping(m))
	verify(m.Id, 300)
}

func TestGroupCache(x *testing.T) {
	t := openTopom()
	defer t.Close()

	verify := func(gid int, exists bool, action string) {
		ctx, err := t.newContext()
		assert.MustNoError(err)
		if exists {
			g, err := ctx.getGroup(gid)
			assert.MustNoError(err)
			assert.Must(g.Id == gid && g.Promoting.State == action)
		} else {
			assert.Must(ctx.group[gid] == nil)
		}
	}

	g := &models.Group{Id: 100}
	verify(g.Id, false, "")

	t.dirtyGroupCache(g.Id)
	g.Promoting.State = models.ActionPreparing
	assert.MustNoError(t.storeCreateGroup(g))
	verify(g.Id, true, models.ActionPreparing)

	t.dirtyGroupCache(g.Id)
	g.Promoting.State = models.ActionPrepared
	verify(g.Id, true, models.ActionPreparing)

	t.dirtyGroupCache(g.Id)
	assert.MustNoError(t.storeUpdateGroup(g))
	verify(g.Id, true, models.ActionPrepared)

	t.dirtyGroupCache(g.Id)
	assert.MustNoError(t.storeRemoveGroup(g))
	verify(g.Id, false, "")
}

func TestProxyCache(x *testing.T) {
	t := openTopom()
	defer t.Close()

	verify := func(token string, exists bool) {
		ctx, err := t.newContext()
		assert.MustNoError(err)
		if exists {
			p, err := ctx.getProxy(token)
			assert.MustNoError(err)
			assert.Must(p.Token == token)
		} else {
			assert.Must(ctx.proxy[token] == nil)
		}
	}

	p := &models.Proxy{Token: "123"}
	verify(p.Token, false)

	t.dirtyProxyCache(p.Token)
	assert.MustNoError(t.storeCreateProxy(p))
	verify(p.Token, true)

	t.dirtyProxyCache(p.Token)
	assert.MustNoError(t.storeRemoveProxy(p))
	verify(p.Token, false)
}

type memStore struct {
	data map[string][]byte
}

func newMemStore() *memStore {
	return &memStore{make(map[string][]byte)}
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
	if _, ok := c.data[path]; ok {
		return errors.Errorf("node already exists")
	}
	c.data[path] = data
	return nil
}

func (c *memClient) Update(path string, data []byte) error {
	c.data[path] = data
	return nil
}

func (c *memClient) Delete(path string) error {
	delete(c.data, path)
	return nil
}

func (c *memClient) Read(path string) ([]byte, error) {
	return c.data[path], nil
}

func (c *memClient) List(path string) ([]string, error) {
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
	return nil
}
