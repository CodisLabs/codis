package topom

import (
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/sync2"
)

type context struct {
	config *Config

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
	var slice = []*models.SlotMapping{}
	for _, m := range ctx.slots {
		if m.GroupId == gid || m.Action.TargetId == gid {
			slice = append(slice, m)
		}
	}
	return slice
}

func (ctx *context) maxSlotActionIndex() (maxIndex int) {
	for _, m := range ctx.slots {
		if m.Action.State != models.ActionNothing {
			maxIndex = utils.MaxInt(maxIndex, m.Action.Index)
		}
	}
	return maxIndex
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
		log.Panicf("invalid slot-[%d] action state:\n%s", m.Id, m.Encode())
	}
	return false
}

func (ctx *context) toSlot(m *models.SlotMapping, forceLocked bool) *models.Slot {
	slot := &models.Slot{
		Id:     m.Id,
		Locked: forceLocked || ctx.isSlotLocked(m),
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
		log.Panicf("invalid slot-[%d] action state:\n%s", m.Id, m.Encode())
	}
	return slot
}

func (ctx *context) toSlotSlice(slots []*models.SlotMapping, forceLocked bool) []*models.Slot {
	var slice = make([]*models.Slot, len(slots))
	for i, m := range slots {
		slice[i] = ctx.toSlot(m, forceLocked)
	}
	return slice
}

func (ctx *context) getGroup(gid int) (*models.Group, error) {
	if g := ctx.group[gid]; g != nil {
		return g, nil
	}
	return nil, errors.Errorf("group-[%d] doesn't exist", gid)
}

func (ctx *context) getGroupByServer(addr string) *models.Group {
	for _, g := range ctx.group {
		for _, x := range g.Servers {
			if x.Addr == addr {
				return g
			}
		}
	}
	return nil
}

func (ctx *context) maxGroupSyncActionIndex() (maxIndex int) {
	for _, g := range ctx.group {
		for _, x := range g.Servers {
			if x.Action.State != models.ActionNothing {
				maxIndex = utils.MaxInt(maxIndex, x.Action.Index)
			}
		}
	}
	return maxIndex
}

func (ctx *context) getGroupMaster(gid int) string {
	if g := ctx.group[gid]; g != nil && len(g.Servers) != 0 {
		return g.Servers[0].Addr
	}
	return ""
}

func (ctx *context) isGroupIsBusy(gid int) bool {
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

func (ctx *context) newProxyClient(p *models.Proxy) *proxy.ApiClient {
	cfg := ctx.config
	c := proxy.NewApiClient(p.AdminAddr)
	c.SetXAuth(cfg.ProductName, cfg.ProductAuth, p.Token)
	return c
}

func (ctx *context) reinitProxy(p *models.Proxy, c *proxy.ApiClient) error {
	log.Infof("reinit proxy-[%s]:\n%s", p.Token, p.Encode())
	if err := c.FillSlots(ctx.toSlotSlice(ctx.slots, false)...); err != nil {
		log.ErrorErrorf(err, "proxy-[%s] fillslots failed", p.Token)
		return errors.Errorf("proxy-[%s] fillslots failed", p.Token)
	}
	if err := c.Start(); err != nil {
		log.ErrorErrorf(err, "proxy-[%s] start failed", p.Token)
		return errors.Errorf("proxy-[%s] start failed", p.Token)
	}
	return nil
}

func (ctx *context) resyncSlots(onError func(p *models.Proxy, err error), slots ...*models.Slot) error {
	if len(slots) == 0 {
		return nil
	}
	var fut sync2.Future
	for _, p := range ctx.proxy {
		fut.Add()
		go func(p *models.Proxy, c *proxy.ApiClient) {
			err := c.FillSlots(slots...)
			if err != nil && onError != nil {
				onError(p, err)
			}
			fut.Done(p.Token, err)
		}(p, ctx.newProxyClient(p))
	}
	for t, v := range fut.Wait() {
		switch err := v.(type) {
		case error:
			return errors.Errorf("proxy-[%s] fillslots failed", t)
		}
	}
	return nil
}
