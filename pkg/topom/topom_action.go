// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"fmt"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/log"
)

func (s *Topom) ProcessSlotAction() error {
	for s.IsOnline() {
		sid, ok, err := s.SlotActionPrepare()
		if err != nil || !ok {
			return err
		}
		if err := s.processSlotAction(sid); err != nil {
			return err
		}
		time.Sleep(time.Millisecond * 10)
	}
	return nil
}

func (s *Topom) processSlotAction(sid int) (err error) {
	defer func() {
		if err != nil {
			s.action.progress.status.Store(fmt.Sprintf("[X] %s", err))
		} else {
			s.action.progress.status.Store("")
		}
	}()
	log.Warnf("slot-[%d] process action", sid)

	var db = 0
	for s.IsOnline() {
		if exec, err := s.newSlotActionExecutor(sid); err != nil {
			return err
		} else if exec == nil {
			time.Sleep(time.Second)
		} else {
			n, nextdb, err := exec(db)
			if err != nil {
				return err
			}
			log.Debugf("slot-[%d] action executor %d", sid, n)

			if n == 0 && nextdb == -1 {
				return s.SlotActionComplete(sid)
			}
			status := fmt.Sprintf("[O] SLOT[%04d]@[%d]: %d", sid, db, n)
			s.action.progress.status.Store(status)

			if us := s.GetSlotActionInterval(); us != 0 {
				time.Sleep(time.Microsecond * time.Duration(us))
			}
			db = nextdb
		}
	}
	return nil
}

func (s *Topom) ProcessSyncAction() error {
	addr, err := s.SyncActionPrepare()
	if err != nil || addr == "" {
		return err
	}
	log.Warnf("sync-[%s] process action", addr)

	exec, err := s.newSyncActionExecutor(addr)
	if err != nil || exec == nil {
		return err
	}
	return s.SyncActionComplete(addr, exec() != nil)
}
