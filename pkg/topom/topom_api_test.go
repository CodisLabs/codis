// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"testing"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils/assert"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

func newApiClient(t *Topom) *ApiClient {
	c := NewApiClient(t.model.AdminAddr)
	c.SetXAuth(t.config.ProductName)
	return c
}

func TestApiTopom(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)

	o, err := c.Model()
	assert.MustNoError(err)
	assert.Must(o != nil)

	assert.MustNoError(c.XPing())

	s, err := c.Stats()
	assert.MustNoError(err)
	assert.Must(s != nil)

	assert.MustNoError(c.LogLevel(log.LevelError))

	assert.MustNoError(c.Shutdown())
}

func TestApiSlots(x *testing.T) {
	t := openTopom()
	defer t.Close()

	s := newFakeServer()
	defer s.Close()

	const sid = 100
	const gid = 200

	c := newApiClient(t)

	assert.MustNoError(c.CreateGroup(gid))
	assert.MustNoError(c.GroupAddServer(gid, "", s.Addr))
	assert.MustNoError(c.SlotCreateAction(sid, gid))
	assert.MustNoError(c.SlotRemoveAction(sid))
	assert.MustNoError(c.SlotCreateActionRange(0, MaxSlotNum-1, gid))
	assert.MustNoError(c.SetSlotActionInterval(2000))
	assert.MustNoError(c.SetSlotActionDisabled(true))

	assert.MustNoError(c.SlotsAssignGroup([]*models.SlotMapping{
		&models.SlotMapping{Id: sid, GroupId: gid},
	}))

	slots, err := c.Slots()
	assert.MustNoError(err)
	assert.Must(len(slots) == MaxSlotNum)
	assert.Must(slots[sid].BackendAddr == s.Addr)
}

func TestApiGroup(x *testing.T) {
	t := openTopom()
	defer t.Close()

	s1 := newFakeServer()
	defer s1.Close()

	s2 := newFakeServer()
	defer s2.Close()

	const gid = 100

	c := newApiClient(t)

	assert.MustNoError(c.CreateGroup(gid))
	assert.MustNoError(c.GroupAddServer(gid, "", s1.Addr))
	assert.MustNoError(c.GroupAddServer(gid, "", s2.Addr))
	assert.MustNoError(c.SyncCreateAction(s1.Addr))
	assert.MustNoError(c.SyncRemoveAction(s1.Addr))
	assert.MustNoError(c.GroupPromoteServer(gid, s2.Addr))
	assert.MustNoError(c.GroupDelServer(gid, s1.Addr))
	assert.MustNoError(c.GroupDelServer(gid, s2.Addr))
	assert.MustNoError(c.RemoveGroup(gid))
}

func TestApiProxy(x *testing.T) {
	t := openTopom()
	defer t.Close()

	c := newApiClient(t)

	p, z := openProxy()
	defer z.Shutdown()

	assert.MustNoError(c.CreateProxy(p.AdminAddr))
	assert.MustNoError(c.ReinitProxy(p.Token))
	assert.MustNoError(c.RemoveProxy(p.Token, false))
}
