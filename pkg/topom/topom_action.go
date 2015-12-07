// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import "time"

func (s *Topom) ProcessSlotAction() error {
	for !s.IsClosed() {
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
		if exec, err := s.newSlotActionExecutor(sid); err != nil {
			return err
		} else if exec == nil {
			time.Sleep(time.Second)
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
			if ms := s.GetSlotActionInterval(); ms != 0 {
				time.Sleep(time.Millisecond * time.Duration(ms))
			}
		}
	}
	return nil
}

func (s *Topom) ProcessSyncAction() error {
	addr, err := s.SyncActionPrepare()
	if err != nil || addr == "" {
		return err
	}
	exec, err := s.newSyncActionExecutor(addr)
	if err != nil || exec == nil {
		return err
	}
	return s.SyncActionComplete(addr, exec() != nil)
}
