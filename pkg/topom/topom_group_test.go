// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"testing"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func getGroup(t *Topom, gid int) *models.Group {
	ctx, err := t.newContext()
	assert.MustNoError(err)
	g, err := ctx.getGroup(gid)
	assert.MustNoError(err)
	return g
}

func TestGroupCreate(x *testing.T) {
	t := openTopom()
	defer t.Close()

	assert.Must(t.CreateGroup(0) != nil)
	assert.Must(t.CreateGroup(-1) != nil)
	assert.Must(t.CreateGroup(10000) != nil)

	assert.MustNoError(t.CreateGroup(1))
	assert.Must(t.CreateGroup(1) != nil)

	g := getGroup(t, 1)
	assert.Must(g.Id == 1)
	assert.Must(g.Promoting.State == models.ActionNothing)
	assert.Must(len(g.Servers) == 0)
}

func TestGroupRemove(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const gid = 200

	assert.Must(t.RemoveGroup(gid) != nil)

	g := &models.Group{Id: gid}
	contextCreateGroup(t, g)
	assert.MustNoError(t.RemoveGroup(gid))

	g.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: "server"},
	}
	contextUpdateGroup(t, g)
	assert.Must(t.RemoveGroup(gid) != nil)

	g.Servers = nil
	contextUpdateGroup(t, g)
	assert.MustNoError(t.RemoveGroup(gid))
}

func TestGroupAddServer(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const gid1 = 100
	const gid2 = 200
	const server1 = "server1:port"
	const server2 = "server2:port"

	assert.MustNoError(t.CreateGroup(gid1))
	assert.MustNoError(t.GroupAddServer(gid1, "", server1))
	assert.Must(t.GroupAddServer(gid1, "", server1) != nil)

	assert.MustNoError(t.CreateGroup(gid2))
	assert.MustNoError(t.GroupAddServer(gid2, "", server2))
	assert.Must(t.GroupAddServer(gid2, "", server1) != nil)

	g1 := getGroup(t, gid1)
	g1.Servers = nil
	contextUpdateGroup(t, g1)

	assert.MustNoError(t.GroupAddServer(gid2, "", server1))

	g2 := getGroup(t, gid2)
	assert.Must(len(g2.Servers) == 2)
	assert.Must(g2.Servers[0].Addr == server2)
	assert.Must(g2.Servers[1].Addr == server1)
}

func TestGroupDelServer(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const sid = 100
	const gid = 200
	const server1 = "server1:port"
	const server2 = "server2:port"

	reset := func() {
		g := &models.Group{Id: gid}
		g.Servers = []*models.GroupServer{
			&models.GroupServer{Addr: server1},
			&models.GroupServer{Addr: server2},
		}
		contextUpdateGroup(t, g)
	}

	reset()

	assert.Must(t.RemoveGroup(gid) != nil)
	assert.MustNoError(t.GroupDelServer(gid, server2))
	assert.MustNoError(t.GroupDelServer(gid, server1))
	assert.MustNoError(t.RemoveGroup(gid))

	reset()

	assert.Must(t.GroupDelServer(gid, server1) != nil)
	assert.MustNoError(t.GroupDelServer(gid, server2))
	assert.MustNoError(t.GroupDelServer(gid, server1))
	assert.Must(t.GroupDelServer(gid, server1) != nil)

	reset()

	m := &models.SlotMapping{Id: sid, GroupId: gid}
	contextUpdateSlotMapping(t, m)

	assert.MustNoError(t.GroupDelServer(gid, server2))
	assert.Must(t.GroupDelServer(gid, server1) != nil)

	m.GroupId = 0
	contextUpdateSlotMapping(t, m)

	assert.MustNoError(t.GroupDelServer(gid, server1))
	assert.MustNoError(t.RemoveGroup(gid))
}

func TestGroupPromote(x *testing.T) {
	t := openTopom()
	defer t.Close()

	s := newFakeServer()
	defer s.Close()

	const sid = 100
	const gid = 200
	const server1 = "server1:port"
	server2 := s.Addr

	reset := func() {
		g := &models.Group{Id: gid}
		g.Servers = []*models.GroupServer{
			&models.GroupServer{Addr: server1},
			&models.GroupServer{Addr: server2},
		}
		contextUpdateGroup(t, g)
	}

	reset()

	assert.Must(t.GroupPromoteServer(gid, server1) != nil)
	g1 := getGroup(t, gid)
	assert.Must(g1.Promoting.State == models.ActionNothing)
	assert.Must(len(g1.Servers) == 2)
	assert.Must(g1.Servers[0].Addr == server1)
	assert.Must(g1.Servers[1].Addr == server2)

	reset()

	assert.MustNoError(t.GroupPromoteServer(gid, server2))
	g2 := getGroup(t, gid)
	assert.Must(g2.Promoting.State == models.ActionNothing)
	assert.Must(len(g2.Servers) == 2)
	assert.Must(g2.Servers[0].Addr == server2)
	assert.Must(g2.Servers[1].Addr == server1)

	reset()

	m := &models.SlotMapping{Id: sid}
	m.Action.State = models.ActionMigrating
	m.Action.TargetId = gid
	contextUpdateSlotMapping(t, m)

	p, c := openProxy()
	defer c.Shutdown()

	contextCreateProxy(t, p)
	assert.MustNoError(c.Shutdown())

	assert.Must(t.GroupPromoteServer(gid, server2) != nil)

	g3 := getGroup(t, gid)
	assert.Must(g3.Promoting.State == models.ActionPreparing)
	contextRemoveProxy(t, p)
	assert.MustNoError(t.GroupPromoteServer(gid, server2))
	g4 := getGroup(t, gid)
	assert.Must(g4.Promoting.State == models.ActionNothing)
	assert.Must(len(g4.Servers) == 2)
	assert.Must(g4.Servers[0].Addr == server2)
	assert.Must(g4.Servers[1].Addr == server1)
}
