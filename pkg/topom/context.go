package topom

import (
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type context struct {
	slots []*models.SlotMapping
	group map[int]*models.Group
	proxy map[string]*models.Proxy
}

func (ctx *context) getSlotMapping(sid int) (*models.SlotMapping, error) {
	if sid >= 0 && sid < len(ctx.slots) {
		return ctx.slots[sid], nil
	}
	return nil, errors.Errorf("slot-[%d] doesn't exist", sid)
}

func (ctx *context) getSlotMappingByGroupId(gid int) []*models.SlotMapping {
	var slots = []*models.SlotMapping{}
	for _, m := range ctx.slots {
		if m.GroupId == gid || m.Action.TargetId == gid {
			slots = append(slots, m)
		}
	}
	return slots
}

func (ctx *context) maxSlotActionIndex() (maxIndex int) {
	for _, m := range ctx.slots {
		if m.Action.State != models.ActionNothing {
			maxIndex = utils.MaxInt(maxIndex, m.Action.Index)
		}
	}
	return maxIndex
}

func (ctx *context) minSlotActionIndex() (d *models.SlotMapping) {
	for _, m := range ctx.slots {
		if m.Action.State != models.ActionNothing {
			if d == nil || m.Action.Index < d.Action.Index {
				d = m
			}
		}
	}
	return d
}

func (ctx *context) isSlotLocked(m *models.SlotMapping) bool {
	switch m.Action.State {
	case models.ActionNothing, models.ActionPending:
		return ctx.isGroupLocked(m.GroupId)
	case models.ActionPreparing:
		return ctx.isGroupLocked(m.GroupId)
	case models.ActionPrepared:
		return true
	case models.ActionMigrating:
		return ctx.isGroupLocked(m.GroupId) || ctx.isGroupLocked(m.Action.TargetId)
	case models.ActionFinished:
		return ctx.isGroupLocked(m.Action.TargetId)
	default:
		log.Panicf("slot-[%d] action state is invalid:\n%s", m.Id, m.Encode())
	}
	return false
}

func (ctx *context) toSlot(m *models.SlotMapping) *models.Slot {
	slot := &models.Slot{
		Id:     m.Id,
		Locked: ctx.isSlotLocked(m),
	}
	switch m.Action.State {
	case models.ActionNothing, models.ActionPending:
		slot.BackendAddr = ctx.getGroupMaster(m.GroupId)
	case models.ActionPreparing:
		slot.BackendAddr = ctx.getGroupMaster(m.GroupId)
	case models.ActionPrepared:
		fallthrough
	case models.ActionMigrating:
		slot.BackendAddr = ctx.getGroupMaster(m.Action.TargetId)
		slot.MigrateFrom = ctx.getGroupMaster(m.GroupId)
	case models.ActionFinished:
		slot.BackendAddr = ctx.getGroupMaster(m.Action.TargetId)
	default:
		log.Panicf("slot-[%d] action state is invalid:\n%s", m.Id, m.Encode())
	}
	return slot
}

func (ctx *context) toSlotSlice(slots []*models.SlotMapping) []*models.Slot {
	var slice = make([]*models.Slot, len(slots))
	for i, m := range slots {
		slice[i] = ctx.toSlot(m)
	}
	return slice
}

func (ctx *context) getGroup(gid int) (*models.Group, error) {
	if g := ctx.group[gid]; g != nil {
		return g, nil
	}
	return nil, errors.Errorf("group-[%d] doesn't exist", gid)
}

func (ctx *context) getGroupIndex(g *models.Group, addr string) (int, error) {
	for i, x := range g.Servers {
		if x.Addr == addr {
			return i, nil
		}
	}
	return -1, errors.Errorf("group-[%d] doesn't have server-[%s]", g.Id, addr)
}

func (ctx *context) getGroupByServer(addr string) (*models.Group, int, error) {
	for _, g := range ctx.group {
		for i, x := range g.Servers {
			if x.Addr == addr {
				return g, i, nil
			}
		}
	}
	return nil, -1, errors.Errorf("server-[%s] doesn't exist", addr)
}

func (ctx *context) maxSyncActionIndex() (maxIndex int) {
	for _, g := range ctx.group {
		for _, x := range g.Servers {
			if x.Action.State == models.ActionPending {
				maxIndex = utils.MaxInt(maxIndex, x.Action.Index)
			}
		}
	}
	return maxIndex
}

func (ctx *context) minSyncActionIndex() string {
	var d *models.GroupServer
	for _, g := range ctx.group {
		for _, x := range g.Servers {
			if x.Action.State == models.ActionPending {
				if d == nil || x.Action.Index < d.Action.Index {
					d = x
				}
			}
		}
	}
	if d == nil {
		return ""
	}
	return d.Addr
}

func (ctx *context) getGroupMaster(gid int) string {
	if g := ctx.group[gid]; g != nil && len(g.Servers) != 0 {
		return g.Servers[0].Addr
	}
	return ""
}

func (ctx *context) isGroupInUse(gid int) bool {
	for _, m := range ctx.slots {
		if m.GroupId == gid || m.Action.TargetId == gid {
			return true
		}
	}
	return false
}

func (ctx *context) isGroupLocked(gid int) bool {
	if g := ctx.group[gid]; g != nil {
		switch g.Promoting.State {
		case models.ActionNothing:
			return false
		case models.ActionPreparing:
			return false
		case models.ActionPrepared:
			return true
		case models.ActionFinished:
			return false
		default:
			log.Panicf("invalid state of group-[%d] = %s", g.Id, g.Encode())
		}
	}
	return false
}

func (ctx *context) isGroupPromoting(gid int) bool {
	if g := ctx.group[gid]; g != nil {
		return g.Promoting.State != models.ActionNothing
	}
	return false
}

func (ctx *context) getProxy(token string) (*models.Proxy, error) {
	if p := ctx.proxy[token]; p != nil {
		return p, nil
	}
	return nil, errors.Errorf("proxy-[%s] doesn't exist", token)
}

func (ctx *context) maxProxyId() (maxId int) {
	for _, p := range ctx.proxy {
		maxId = utils.MaxInt(maxId, p.Id)
	}
	return maxId
}
