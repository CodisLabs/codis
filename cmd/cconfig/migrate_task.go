// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/juju/errors"
	log "github.com/ngaut/logging"
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/zkhelper"
	"time"
)

type MigrateTaskInfo struct {
	SlotId     int    `json:"slot_id"`
	NewGroupId int    `json:"new_group"`
	Delay      int    `json:"delay"`
	CreateAt   string `json:"create_at"`
	Percent    int    `json:"percent"`
	Status     string `json:"status"`
	Id         string `json:"-"`
}

type SlotMigrateProgress struct {
	SlotId    int `json:"slot_id"`
	FromGroup int `json:"from"`
	ToGroup   int `json:"to"`
	Remain    int `json:"remain"`
}

func (p SlotMigrateProgress) String() string {
	return fmt.Sprintf("migrate Slot: slot_%d From: group_%d To: group_%d remain: %d keys", p.SlotId, p.FromGroup, p.ToGroup, p.Remain)
}

type MigrateTask struct {
	MigrateTaskInfo
	zkConn       zkhelper.Conn
	productName  string
	progressChan chan SlotMigrateProgress
}

func GetMigrateTask(info MigrateTaskInfo) *MigrateTask {
	return &MigrateTask{
		MigrateTaskInfo: info,
		productName:     globalEnv.ProductName(),
		zkConn:          safeZkConn,
	}
}

func (t *MigrateTask) UpdateStatus(status string) {
	t.Status = status
	b, _ := json.Marshal(t.MigrateTaskInfo)
	t.zkConn.Set(getMigrateTasksPath(t.productName)+"/"+t.Id, b, -1)
}

func (t *MigrateTask) UpdateFinish() {
	t.Status = MIGRATE_TASK_FINISHED
	t.zkConn.Delete(getMigrateTasksPath(t.productName)+"/"+t.Id, -1)
}
func (t *MigrateTask) migrateSingleSlot(slotId int, to int) error {
	// set slot status
	s, err := models.GetSlot(t.zkConn, t.productName, slotId)
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

	// make sure from group & target group exists
	exists, err := models.GroupExists(t.zkConn, t.productName, from)
	if err != nil {
		return errors.Trace(err)
	}
	if !exists {
		log.Errorf("src group %d not exist when migrate from %d to %d", from, from, to)
		return errors.NotFoundf("group %d", from)
	}

	exists, err = models.GroupExists(t.zkConn, t.productName, to)
	if err != nil {
		return errors.Trace(err)
	}
	if !exists {
		return errors.NotFoundf("group %d", to)
	}

	// cannot migrate to itself, just ignore
	if from == to {
		log.Warning("from == to, ignore", s)
		return nil
	}

	// modify slot status
	if err := s.SetMigrateStatus(t.zkConn, from, to); err != nil {
		log.Error(err)
		return err
	}

	err = t.Migrate(s, from, to, func(p SlotMigrateProgress) {
		// on migrate slot progress
		if p.Remain%5000 == 0 {
			log.Info(p)
		}
	})
	if err != nil {
		log.Error(err)
		return err
	}

	// migrate done, change slot status back
	s.State.Status = models.SLOT_STATUS_ONLINE
	s.State.MigrateStatus.From = models.INVALID_ID
	s.State.MigrateStatus.To = models.INVALID_ID
	if err := s.Update(t.zkConn); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func (t *MigrateTask) run() error {
	log.Infof("migration start: %+v", t.MigrateTaskInfo)
	to := t.NewGroupId
	t.UpdateStatus(MIGRATE_TASK_MIGRATING)
	err := t.migrateSingleSlot(t.SlotId, to)
	if err != nil {
		log.Error(err)
		t.UpdateStatus(MIGRATE_TASK_ERR)
		return err
	}
	t.UpdateFinish()
	log.Infof("migration finished: %+v", t.MigrateTaskInfo)
	return nil
}

// will block until all keys are migrated
func (task *MigrateTask) Migrate(slot *models.Slot, fromGroup, toGroup int, onProgress func(SlotMigrateProgress)) (err error) {
	groupFrom, err := models.GetGroup(task.zkConn, task.productName, fromGroup)
	if err != nil {
		return err
	}
	groupTo, err := models.GetGroup(task.zkConn, task.productName, toGroup)
	if err != nil {
		return err
	}

	fromMaster, err := groupFrom.Master(task.zkConn)
	if err != nil {
		return err
	}

	toMaster, err := groupTo.Master(task.zkConn)
	if err != nil {
		return err
	}

	if fromMaster == nil || toMaster == nil {
		return ErrGroupMasterNotFound
	}

	c, err := redis.Dial("tcp", fromMaster.Addr)
	if err != nil {
		return err
	}

	defer c.Close()

	_, remain, err := sendRedisMigrateCmd(c, slot.Id, toMaster.Addr)
	if err != nil {
		return err
	}

	for remain > 0 {
		if task.Delay > 0 {
			time.Sleep(time.Duration(task.Delay) * time.Millisecond)
		}
		_, remain, err = sendRedisMigrateCmd(c, slot.Id, toMaster.Addr)
		if remain >= 0 {
			onProgress(SlotMigrateProgress{
				SlotId:    slot.Id,
				FromGroup: fromGroup,
				ToGroup:   toGroup,
				Remain:    remain,
			})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *MigrateTask) preMigrateCheck() error {
	slots, err := models.GetMigratingSlots(safeZkConn, t.productName)

	if err != nil {
		return errors.Trace(err)
	}
	// check if there is migrating slot
	if len(slots) > 1 {
		return errors.New("more than one slots are migrating, unknown error")
	}
	if len(slots) == 1 {
		slot := slots[0]
		if t.NewGroupId != slot.State.MigrateStatus.To || t.SlotId != slot.Id {
			return errors.Errorf("there is a migrating slot %+v, finish it first", slot)
		}
	}
	return nil
}
