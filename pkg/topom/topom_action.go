// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

func (s *Topom) ProcessSlotAction() error {
	for !s.IsClosed() {
		if s.GetSlotActionDisabled() {
			time.Sleep(time.Second)
			continue
		}
		sid, err := s.SlotActionPrepare()
		if err != nil || sid < 0 {
			return err
		}
		if err := s.processSlotAction(sid); err != nil {
			return err
		}
	}
	return nil
}

func (s *Topom) processSlotAction(sid int) (err error) {
	defer func() {
		if err != nil {
			s.action.progress.failed.Set(true)
		} else {
			s.action.progress.remain.Set(0)
			s.action.progress.failed.Set(false)
		}
	}()
	for !s.IsClosed() {
		if s.GetSlotActionDisabled() {
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if exec, err := s.newSlotActionExecutor(sid); err != nil {
			return err
		} else if exec == nil {
			time.Sleep(time.Millisecond * 50)
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
			s.noopInterval()
		}
	}
	return nil
}

func (s *Topom) noopInterval() int {
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

	case models.ActionMigrating:

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

	case models.ActionFinished:

		return func() (int, error) {
			return 0, nil
		}, nil

	default:

		return nil, errors.Errorf("slot-[%d] action state is invalid", m.Id)

	}
}

func (s *Topom) ProcessSyncAction() error {
	addr, err := s.SyncActionPrepare()
	if err != nil || addr == "" {
		return err
	}
	if exec, err := s.newSyncActionExecutor(addr); err != nil {
		return err
	} else if err := exec(); err != nil {
		return err
	} else {
		return s.SyncActionComplete(addr)
	}
}

func (s *Topom) newSyncActionExecutor(addr string) (func() error, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return nil, err
	}

	g, index, err := ctx.getGroupByServer(addr)
	if err != nil {
		return nil, err
	}

	switch g.Servers[index].Action.State {

	case models.ActionSyncing:

		var master = "NO:ONE"
		if index != 0 {
			master = g.Servers[0].Addr
		}
		return func() error {
			c, err := NewRedisClient(addr, s.config.ProductAuth, time.Minute*15)
			if err != nil {
				return err
			}
			defer c.Close()
			return c.SetMaster(master)
		}, nil

	default:

		return nil, errors.Errorf("server-[%s] action state is invalid", addr)

	}

}
