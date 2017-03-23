// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"testing"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func TestSlotState(x *testing.T) {
	t := openTopom()
	defer t.Close()

	const sid = 1
	const gid1 = 1
	const gid2 = 2
	const server1 = "server1:port"
	const server2 = "server2:port"

	check := func() {
		ctx, err := t.newContext()
		assert.MustNoError(err)
		m, err := ctx.getSlotMapping(sid)
		assert.MustNoError(err)
		assert.Must(m.Id == sid)
		assert.Must(m.GroupId == gid1 && m.Action.TargetId == gid2)

		g1, err := ctx.getGroup(gid1)
		assert.MustNoError(err)
		assert.Must(ctx.getGroupMaster(gid1) == server1)

		g2, err := ctx.getGroup(gid2)
		assert.MustNoError(err)
		assert.Must(ctx.getGroupMaster(gid2) == server2)

		slot := ctx.toSlot(m, nil)

		switch m.Action.State {
		case models.ActionPrepared:
			assert.Must(slot.Locked == true)
		case models.ActionMigrating:
			switch {
			case g1.Promoting.State == models.ActionPrepared:
				assert.Must(slot.Locked == true)
			case g2.Promoting.State == models.ActionPrepared:
				assert.Must(slot.Locked == true)
			default:
				assert.Must(slot.Locked == false)
				assert.Must(slot.BackendAddr == server2)
				assert.Must(slot.MigrateFrom == server1)
			}
		case models.ActionFinished:
			switch {
			case g2.Promoting.State == models.ActionPrepared:
				assert.Must(slot.Locked == true)
			default:
				assert.Must(slot.Locked == false)
				assert.Must(slot.BackendAddr == server2)
				assert.Must(slot.MigrateFrom == "")
			}
		default:
			switch {
			case g1.Promoting.State == models.ActionPrepared:
				assert.Must(slot.Locked == true)
			default:
				assert.Must(slot.Locked == false)
				assert.Must(slot.BackendAddr == server1)
				assert.Must(slot.MigrateFrom == "")
			}
		}
	}

	g1 := &models.Group{
		Id: gid1,
		Servers: []*models.GroupServer{
			&models.GroupServer{Addr: server1},
		},
	}
	g2 := &models.Group{
		Id: gid2,
		Servers: []*models.GroupServer{
			&models.GroupServer{Addr: server2},
		},
	}

	m := &models.SlotMapping{Id: sid}
	m.GroupId = gid1
	m.Action.TargetId = gid2

	sstates := []string{
		models.ActionNothing,
		models.ActionPending,
		models.ActionPreparing,
		models.ActionPrepared,
		models.ActionMigrating,
		models.ActionFinished,
	}

	gstates := []string{
		models.ActionNothing,
		models.ActionPreparing,
		models.ActionPrepared,
		models.ActionFinished,
	}

	for _, m.Action.State = range sstates {
		contextUpdateSlotMapping(t, m)
		for _, g1.Promoting.State = range gstates {
			contextUpdateGroup(t, g1)
			for _, g2.Promoting.State = range gstates {
				contextUpdateGroup(t, g2)
				check()
			}
		}
	}
}
