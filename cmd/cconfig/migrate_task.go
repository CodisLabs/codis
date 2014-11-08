// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"
	"sync"
	"time"

	"container/list"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"

	"github.com/juju/errors"

	log "github.com/ngaut/logging"
)

var pendingMigrateTask = list.New()
var curMigrateTask *MigrateTask
var lck = sync.RWMutex{}

const (
	MIGRATE_TASK_PENDING   string = "pending"
	MIGRATE_TASK_MIGRATING string = "migrating"
	MIGRATE_TASK_FINISHED  string = "finished"
	MIGRATE_TASK_ERR       string = "error"
)

type MigrateTaskForm struct {
	FromSlot   int    `json:"from"`
	ToSlot     int    `json:"to"`
	NewGroupId int    `json:"new_group"`
	Delay      int    `json:"delay"`
	CreateAt   string `json:"create_at"`
	Percent    int    `json:"percent"`
	Status     string `json:"status"`
	Id         string `json:"id"`
}

type MigrateTask struct {
	MigrateTaskForm

	stopChan chan struct{} `json:"-"`
}

func findPendingMigrateTask(id string) *MigrateTask {
	for e := pendingMigrateTask.Front(); e != nil; e = e.Next() {
		t := e.Value.(*MigrateTask)
		if t.Id == id {
			return t
		}
	}
	return nil
}

func removePendingMigrateTask(id string) bool {
	for e := pendingMigrateTask.Front(); e != nil; e = e.Next() {
		t := e.Value.(*MigrateTask)
		if t.Id == id && t.Status == "pending" {
			pendingMigrateTask.Remove(e)
			return true
		}
	}
	return false
}

// migrate multi slots
func RunMigrateTask(task *MigrateTask) error {
	conn := CreateZkConn()
	defer conn.Close()
	lock := utils.GetZkLock(conn, productName)

	to := task.NewGroupId
	task.Status = MIGRATE_TASK_MIGRATING
	for slotId := task.FromSlot; slotId <= task.ToSlot; slotId++ {
		err := func() error {
			log.Info("start migrate slot:", slotId)

			lock.Lock(fmt.Sprintf("migrate %d", slotId))
			defer func() {
				err := lock.Unlock()
				if err != nil {
					log.Info(err)
				}
			}()
			s, err := models.GetSlot(conn, productName, slotId)
			if err != nil {
				log.Error(err)
				return err
			}
			if s.State.Status != models.SLOT_STATUS_ONLINE && s.State.Status != models.SLOT_STATUS_MIGRATE {
				log.Warning("status is not online && migrate", s)
				return nil
			}

			from := s.GroupId
			if s.State.Status == models.SLOT_STATUS_MIGRATE {
				from = s.State.MigrateStatus.From
			}

			if from == to {
				log.Warning("from == to, ignore", s)
				return nil
			}

			// modify slot status
			if err := s.SetMigrateStatus(conn, from, to); err != nil {
				log.Error(err)
				return err
			}

			// do real migrate
			err = MigrateSingleSlot(conn, slotId, from, to, task.Delay, task.stopChan)
			if err != nil {
				log.Error(err)
				return err
			}

			// migrate done, change slot status back
			s.State.Status = models.SLOT_STATUS_ONLINE
			s.State.MigrateStatus.From = models.INVALID_ID
			s.State.MigrateStatus.To = models.INVALID_ID
			if err := s.Update(zkConn); err != nil {
				log.Error(err)
				return err
			}
			return nil
		}()
		if err == ErrStopMigrateByUser {
			log.Info("stop migration job by user")
			break
		} else if err != nil {
			task.Status = MIGRATE_TASK_ERR
			return err
		}
		task.Percent = (slotId - task.FromSlot + 1) * 100 / (task.ToSlot - task.FromSlot + 1)
		log.Info("total percent:", task.Percent)
	}
	task.Status = MIGRATE_TASK_FINISHED
	log.Info("migration finished")
	return nil
}

func preMigrateCheck(t *MigrateTask) (bool, error) {
	conn := CreateZkConn()
	defer conn.Close()

	slots, err := models.GetMigratingSlots(conn, productName)

	if err != nil {
		return false, err
	}
	// check if there is migrating slot
	if len(slots) == 0 {
		return true, nil
	} else if len(slots) > 1 {
		return false, errors.New("more than one slots are migrating, unknown error")
	} else if len(slots) == 1 {
		slot := slots[0]
		if t.NewGroupId != slot.State.MigrateStatus.To || t.FromSlot != slot.Id || t.ToSlot != slot.Id {
			return false, errors.Errorf("there is a migrating slot %+v, finish it first", slot)
		}
	}
	return true, nil
}

func migrateTaskWorker() {
	for {
		select {
		case <-time.After(1 * time.Second):
			{
				// check if there is new task
				lck.RLock()
				cnt := pendingMigrateTask.Len()
				lck.RUnlock()
				if cnt > 0 {
					lck.RLock()
					t := pendingMigrateTask.Front()
					lck.RUnlock()

					log.Info("new migrate task arrive")
					if t != nil {
						lck.Lock()
						curMigrateTask = t.Value.(*MigrateTask)
						lck.Unlock()

						if ok, err := preMigrateCheck(curMigrateTask); ok {
							RunMigrateTask(curMigrateTask)
						} else {
							log.Warning(err)
						}

						lck.Lock()
						curMigrateTask = nil
						lck.Unlock()
					}
					log.Info("migrate task", t, "done")

					lck.Lock()
					if t != nil {
						pendingMigrateTask.Remove(t)
					}
					lck.Unlock()
				}
			}
		}
	}
}
