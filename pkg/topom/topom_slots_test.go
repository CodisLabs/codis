// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"testing"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy"
	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func getSlotMapping(t *Topom, sid int) *models.SlotMapping {
	ctx, err := t.newContext()
	assert.MustNoError(err)
	m, err := ctx.getSlotMapping(sid)
	assert.MustNoError(err)
	assert.Must(m.Id == sid)
	return m
}

func checkSlots(t *Topom, c *proxy.ApiClient) {
	ctx, err := t.newContext()
	assert.MustNoError(err)

	slots1 := ctx.toSlotSlice(ctx.slots, nil)
	assert.Must(len(slots1) == MaxSlotNum)

	slots2, err := c.Slots()
	assert.MustNoError(err)
	assert.Must(len(slots2) == MaxSlotNum)

	for i := 0; i < len(slots1); i++ {
		a := slots1[i]
		b := slots2[i]
		assert.Must(a.Id == b.Id)
		assert.Must(a.Locked == b.Locked)
		assert.Must(a.BackendAddr == b.BackendAddr)
		assert.Must(a.MigrateFrom == b.MigrateFrom)
	}
}

func TestSlotCreateAction(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const sid = 100
	const gid = 200

	assert.Must(t.SlotCreateAction(sid, gid) != nil)

	g := &models.Group{Id: gid}
	contextUpdateGroup(t, g)
	assert.Must(t.SlotCreateAction(sid, gid) != nil)

	g.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: "server"},
	}
	contextUpdateGroup(t, g)
	assert.MustNoError(t.SlotCreateAction(sid, gid))

	assert.Must(t.SlotCreateAction(sid, gid) != nil)

	m := getSlotMapping(t, sid)
	assert.Must(m.GroupId == 0)
	assert.Must(m.Action.State == models.ActionPending)
	assert.Must(m.Action.Index > 0 && m.Action.TargetId == gid)

	m = &models.SlotMapping{Id: sid, GroupId: gid}
	contextUpdateSlotMapping(t, m)
	assert.Must(t.SlotCreateAction(sid, gid) != nil)
}

func TestSlotRemoveAction(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const sid = 100
	assert.Must(t.SlotRemoveAction(sid) != nil)

	sstates := []string{
		models.ActionNothing,
		models.ActionPending,
		models.ActionPreparing,
		models.ActionPrepared,
		models.ActionMigrating,
		models.ActionFinished,
	}

	m := &models.SlotMapping{Id: sid}
	for _, m.Action.State = range sstates {
		contextUpdateSlotMapping(t, m)
		if m.Action.State == models.ActionPending {
			assert.MustNoError(t.SlotRemoveAction(sid))
		} else {
			assert.Must(t.SlotRemoveAction(sid) != nil)
		}
	}
}

func prepareSlotAction(t *Topom, sid int, must bool) *models.SlotMapping {
	i, ok, err := t.SlotActionPrepare()
	if must {
		assert.MustNoError(err)
		assert.Must(ok && sid == i)
	} else {
		assert.Must(ok == false)
	}
	return getSlotMapping(t, sid)
}

func completeSlotAction(t *Topom, sid int, must bool) *models.SlotMapping {
	err := t.SlotActionComplete(sid)
	if must {
		assert.MustNoError(err)
	}
	return getSlotMapping(t, sid)
}

func TestSlotActionSimple(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const sid = 100
	const gid = 200

	m := &models.SlotMapping{Id: sid}
	m.Action.State = models.ActionPending
	m.Action.TargetId = gid
	contextUpdateSlotMapping(t, m)

	m1 := prepareSlotAction(t, sid, true)
	assert.Must(m1.GroupId == 0)
	assert.Must(m1.Action.State == models.ActionMigrating)

	m2 := completeSlotAction(t, sid, true)
	assert.Must(m2.GroupId == gid)
	assert.Must(m2.Action.State == models.ActionNothing)
}

func TestSlotActionPending(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const sid = 100
	const gid = 200

	reset := func() {
		m := &models.SlotMapping{Id: sid}
		m.Action.State = models.ActionPending
		m.Action.TargetId = gid
		contextUpdateSlotMapping(t, m)
	}

	reset()

	m1 := prepareSlotAction(t, sid, true)
	assert.Must(m1.Action.State == models.ActionMigrating)

	reset()

	p, c := openProxy()
	defer c.Shutdown()

	contextCreateProxy(t, p)

	m2 := prepareSlotAction(t, sid, true)
	assert.Must(m2.Action.State == models.ActionMigrating)
	checkSlots(t, c)

	assert.MustNoError(c.Shutdown())

	reset()

	m3 := prepareSlotAction(t, sid, false)
	assert.Must(m3.Action.State == models.ActionPreparing)
}

func TestSlotActionPreparing(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const sid = 100
	const gid = 200

	reset := func() {
		m := &models.SlotMapping{Id: sid}
		m.Action.State = models.ActionPreparing
		m.Action.TargetId = gid
		contextUpdateSlotMapping(t, m)
	}

	reset()

	m1 := prepareSlotAction(t, sid, true)
	assert.Must(m1.Action.State == models.ActionMigrating)

	reset()

	p1, c1 := openProxy()
	defer c1.Shutdown()

	p2, c2 := openProxy()
	defer c2.Shutdown()

	contextCreateProxy(t, p1)
	contextCreateProxy(t, p2)

	m2 := prepareSlotAction(t, sid, true)
	assert.Must(m2.Action.State == models.ActionMigrating)
	checkSlots(t, c1)
	checkSlots(t, c2)

	assert.MustNoError(c1.Shutdown())

	reset()

	m3 := prepareSlotAction(t, sid, false)
	assert.Must(m3.Action.State == models.ActionPreparing)
	checkSlots(t, c2)
}

func TestSlotActionPrepared(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const gid1, gid2 = 200, 300
	const server1 = "server1:port"
	const server2 = "server2:port"

	g1 := &models.Group{Id: gid1}
	g1.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: server1},
	}
	contextCreateGroup(t, g1)
	g2 := &models.Group{Id: gid2}
	g2.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: server2},
	}
	contextCreateGroup(t, g2)

	const sid = 100

	reset := func() {
		m := &models.SlotMapping{Id: sid, GroupId: gid1}
		m.Action.State = models.ActionPrepared
		m.Action.TargetId = gid2
		contextUpdateSlotMapping(t, m)
	}

	reset()

	m1 := prepareSlotAction(t, sid, true)
	assert.Must(m1.Action.State == models.ActionMigrating)

	reset()

	p1, c1 := openProxy()
	defer c1.Shutdown()

	p2, c2 := openProxy()
	defer c2.Shutdown()

	contextCreateProxy(t, p1)
	contextCreateProxy(t, p2)

	m2 := prepareSlotAction(t, sid, true)
	assert.Must(m2.Action.State == models.ActionMigrating)
	checkSlots(t, c1)
	checkSlots(t, c2)

	assert.MustNoError(c1.Shutdown())

	reset()

	m3 := prepareSlotAction(t, sid, false)
	assert.Must(m3.Action.State == models.ActionPrepared)

	slots, err := c2.Slots()
	assert.MustNoError(err)
	assert.Must(len(slots) == MaxSlotNum)

	s := slots[sid]
	assert.Must(s.Locked == false)
	assert.Must(s.BackendAddr == server2 && s.MigrateFrom == server1)
}

func TestSlotActionMigrating(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const gid1, gid2 = 200, 300
	const server1 = "server1:port"
	const server2 = "server2:port"

	g1 := &models.Group{Id: gid1}
	g1.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: server1},
	}
	contextCreateGroup(t, g1)
	g2 := &models.Group{Id: gid2}
	g2.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: server2},
	}
	contextCreateGroup(t, g2)

	const sid = 100

	reset := func() {
		m := &models.SlotMapping{Id: sid, GroupId: gid1}
		m.Action.State = models.ActionMigrating
		m.Action.TargetId = gid2
		contextUpdateSlotMapping(t, m)
	}

	reset()

	m1 := completeSlotAction(t, sid, true)
	assert.Must(m1.GroupId == gid2)
	assert.Must(m1.Action.State == models.ActionNothing)

	reset()

	p1, c1 := openProxy()
	defer c1.Shutdown()

	p2, c2 := openProxy()
	defer c2.Shutdown()

	contextCreateProxy(t, p1)
	contextCreateProxy(t, p2)

	m2 := completeSlotAction(t, sid, true)
	assert.Must(m2.GroupId == gid2)
	assert.Must(m2.Action.State == models.ActionNothing)
	checkSlots(t, c1)
	checkSlots(t, c2)

	assert.MustNoError(c1.Shutdown())

	reset()

	m3 := completeSlotAction(t, sid, false)
	assert.Must(m3.GroupId == gid1)
	assert.Must(m3.Action.TargetId == gid2)
	assert.Must(m3.Action.State == models.ActionFinished)

	slots, err := c2.Slots()
	assert.MustNoError(err)
	assert.Must(len(slots) == MaxSlotNum)

	s := slots[sid]
	assert.Must(s.Locked == false)
	assert.Must(s.BackendAddr == server2 && s.MigrateFrom == "")
}

func TestSlotActionFinished(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const gid1, gid2 = 200, 300
	const server1 = "server1:port"
	const server2 = "server2:port"

	g1 := &models.Group{Id: gid1}
	g1.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: server1},
	}
	contextCreateGroup(t, g1)
	g2 := &models.Group{Id: gid2}
	g2.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: server2},
	}
	contextCreateGroup(t, g2)

	const sid = 100

	reset := func() {
		m := &models.SlotMapping{Id: sid, GroupId: gid1}
		m.Action.State = models.ActionFinished
		m.Action.TargetId = gid2
		contextUpdateSlotMapping(t, m)
	}

	reset()

	m1 := completeSlotAction(t, sid, true)
	assert.Must(m1.GroupId == gid2)
	assert.Must(m1.Action.State == models.ActionNothing)

	reset()

	p1, c1 := openProxy()
	defer c1.Shutdown()

	p2, c2 := openProxy()
	defer c2.Shutdown()

	contextCreateProxy(t, p1)
	contextCreateProxy(t, p2)

	m2 := completeSlotAction(t, sid, true)
	assert.Must(m2.GroupId == gid2)
	assert.Must(m2.Action.State == models.ActionNothing)
	checkSlots(t, c1)
	checkSlots(t, c2)

	assert.MustNoError(c1.Shutdown())

	reset()

	m3 := completeSlotAction(t, sid, false)
	assert.Must(m3.GroupId == gid1)
	assert.Must(m3.Action.TargetId == gid2)
	assert.Must(m3.Action.State == models.ActionFinished)

	slots, err := c2.Slots()
	assert.MustNoError(err)
	assert.Must(len(slots) == MaxSlotNum)

	s := slots[sid]
	assert.Must(s.Locked == false)
	assert.Must(s.BackendAddr == server2 && s.MigrateFrom == "")
}

func TestSlotsAssignGroup(x *testing.T) {
	t := openTopom()
	defer t.Close()

	m := &models.SlotMapping{Id: 100, GroupId: 200}
	m.Action.State = models.ActionPending

	assert.Must(t.SlotsAssignGroup([]*models.SlotMapping{m}) != nil)

	g := &models.Group{Id: 200, Servers: []*models.GroupServer{
		&models.GroupServer{Addr: "server"},
	}}
	contextCreateGroup(t, g)
	assert.Must(t.SlotsAssignGroup([]*models.SlotMapping{m}) != nil)

	m.Action.State = models.ActionNothing
	assert.MustNoError(t.SlotsAssignGroup([]*models.SlotMapping{m}))
}

func TestSlotsRebalance(x *testing.T) {
	t := openTopom()
	defer t.Close()

	groupBy := func(plans map[int]int) map[int]int {
		d := make(map[int]int)
		for sid, gid := range plans {
			assert.Must(sid >= 0 && sid < MaxSlotNum)
			m := getSlotMapping(t, sid)
			assert.Must(m.Action.State == models.ActionNothing)
			assert.Must(m.GroupId != gid)
			d[gid]++
		}
		return d
	}

	plans1, err := t.SlotsRebalance(false)
	assert.Must(plans1 == nil && err != nil)

	g1 := &models.Group{Id: 100, Servers: []*models.GroupServer{
		&models.GroupServer{Addr: "server1"},
	}}
	contextCreateGroup(t, g1)

	plans2, err := t.SlotsRebalance(false)
	assert.MustNoError(err)
	assert.Must(len(plans2) == MaxSlotNum)
	d2 := groupBy(plans2)
	assert.Must(len(d2) == 1 && d2[g1.Id] == MaxSlotNum)

	g2 := &models.Group{Id: 200, Servers: []*models.GroupServer{
		&models.GroupServer{Addr: "server2"},
	}}
	contextCreateGroup(t, g2)

	plans3, err := t.SlotsRebalance(false)
	assert.MustNoError(err)
	assert.Must(len(plans3) == MaxSlotNum)
	d3 := groupBy(plans3)
	assert.Must(len(d3) == 2 && d3[g1.Id] == d3[g2.Id])

	for i := 0; i < MaxSlotNum; i++ {
		m := &models.SlotMapping{Id: i, GroupId: g1.Id}
		contextUpdateSlotMapping(t, m)
	}
	plans4, err := t.SlotsRebalance(false)
	assert.MustNoError(err)
	assert.Must(len(plans4) == MaxSlotNum/2)
	d4 := groupBy(plans4)
	assert.Must(len(d4) == 1 && d4[g2.Id] == len(plans4))

	for i := 0; i < MaxSlotNum; i++ {
		m := &models.SlotMapping{Id: i}
		if i >= MaxSlotNum/4 {
			m.Action.State = models.ActionPending
			m.Action.TargetId = g1.Id
		}
		contextUpdateSlotMapping(t, m)
	}
	plans5, err := t.SlotsRebalance(false)
	assert.MustNoError(err)
	assert.Must(len(plans5) == MaxSlotNum/4)
	d5 := groupBy(plans5)
	assert.Must(len(d5) == 1 && d5[g2.Id] == len(plans5))
}
