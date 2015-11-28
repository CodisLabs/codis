// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

func (s *Topom) ProcessSlotAction(sid int) (err error) {
	defer func() {
		if err != nil {
			s.action.progress.failed.Set(true)
		} else {
			s.action.progress.remain.Set(0)
			s.action.progress.failed.Set(false)
		}
	}()
	if err := s.SlotActionPrepare(sid); err != nil {
		return err
	}
	for {
		for s.GetSlotActionDisabled() {
			time.Sleep(time.Millisecond * 10)
		}
		if exec, err := s.newSlotActionExecutor(sid); err != nil {
			return err
		} else if exec == nil {
			time.Sleep(time.Millisecond * 10)
		} else {
			n, err := exec()
			if err != nil {
				return err
			}
			if n == 0 {
				return s.SlotActionComplete(sid)
			}
			s.action.progress.remain.Set(int64(n))
			s.action.progress.failed.Set(false)
			s.NoopInterval()
		}
	}
}

func (s *Topom) NoopInterval() int {
	var ms int
	for !s.IsClosed() {
		if d := s.GetSlotActionInterval() - ms; d <= 0 {
			return ms
		} else {
			d = utils.MinInt(d, 50)
			time.Sleep(time.Millisecond * time.Duration(d))
			ms += d
		}
	}
	return ms
}

func (s *Topom) FirstSlotAction() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return -1
	}

	if s.GetSlotActionDisabled() {
		return -1
	}

	var sid = -1
	var index int
	for _, m := range ctx.slots {
		if m.Action.State != models.ActionNothing {
			if index == 0 || m.Action.Index < index {
				sid, index = m.Id, m.Action.Index
			}
		}
	}
	return sid
}

func (s *Topom) newSlotActionExecutor(sid int) (func() (int, error), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return nil, err
	}

	m, err := ctx.getSlotMapping(sid)
	if err != nil {
		return nil, err
	}

	switch m.Action.State {

	case models.ActionMigrating, models.ActionFinished:

		if ctx.isSlotLocked(m) {
			return nil, nil
		}

		from := ctx.getGroupMaster(m.GroupId)
		dest := ctx.getGroupMaster(m.Action.TargetId)

		s.action.executor.Incr()

		return func() (int, error) {
			defer s.action.executor.Decr()
			if from == "" {
				return 0, nil
			}
			return s.redisp.MigrateSlot(sid, from, dest)
		}, nil

	default:

		return nil, errors.Errorf("action of slot-[%d] is not migrating", sid)

	}
}

func (s *Topom) ProcessSyncAction(gid int, addr string) error {
	if err := s.GroupSyncActionPrepare(gid, addr); err != nil {
		return err
	}
	if exec, err := s.newSyncActionExecutor(gid, addr); err != nil {
		return err
	} else if err := exec(); err != nil {
		return err
	} else {
		return s.GroupSyncActionComplete(gid, addr)
	}
}

func (s *Topom) FirstSyncAction() (int, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return -1, ""
	}

	var gid, addr = -1, ""
	var index int
	for _, g := range ctx.group {
		for _, x := range g.Servers {
			if x.Action.State != models.ActionNothing {
				if index == 0 || x.Action.Index < index {
					gid, addr, index = g.Id, x.Addr, x.Action.Index
				}
			}
		}
	}
	return gid, addr
}

func (s *Topom) newSyncActionExecutor(gid int, addr string) (func() error, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return nil, err
	}

	g, err := ctx.getGroup(gid)
	if err != nil {
		return nil, err
	}

	var index = g.IndexOfServer(addr)

	if index < 0 {
		return nil, errors.Errorf("group-[%d] doesn't have server %s", gid, addr)
	}

	if g.Servers[index].Action.State != models.ActionSyncing {
		return nil, errors.Errorf("action of server-[%s] is not syncing", addr)
	}

	var master = "NO:ONE"
	if index != 0 {
		master = ctx.getGroupMaster(gid)
	}
	return func() error {
		c, err := NewRedisClient(addr, s.config.ProductAuth, time.Minute*15)
		if err != nil {
			return err
		}
		defer c.Close()
		return c.SetMaster(master)
	}, nil
}
