package topom

import (
	"testing"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/assert"
)

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

	ctx, err := t.newContext()
	assert.MustNoError(err)
	m, err := ctx.getSlotMapping(sid)
	assert.MustNoError(err)
	assert.Must(m.Id == sid && m.GroupId == 0)
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
