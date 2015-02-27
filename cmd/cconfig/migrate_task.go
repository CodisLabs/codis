// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"

	"github.com/wandoulabs/codis/pkg/models"

	"github.com/juju/errors"

	log "github.com/ngaut/logging"
	"github.com/ngaut/zkhelper"
)

type MigrateTaskInfo struct {
	FromSlot   int    `json:"from"`
	ToSlot     int    `json:"to"`
	NewGroupId int    `json:"new_group"`
	Delay      int    `json:"delay"`
	CreateAt   string `json:"create_at"`
	Percent    int    `json:"percent"`
	Status     string `json:"status"`
	Id         string `json:"id"`
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
	stopChan     chan struct{}
	zkConn       zkhelper.Conn
	productName  string
	slotMigrator SlotMigrator
	progressChan chan SlotMigrateProgress
}

func NewMigrateTask(info MigrateTaskInfo) *MigrateTask {
	return &MigrateTask{
		MigrateTaskInfo: info,
		slotMigrator:    &CodisSlotMigrator{},
		stopChan:        make(chan struct{}),
		productName:     globalEnv.ProductName(),
	}
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

	err = t.slotMigrator.Migrate(s, from, to, t, func(p SlotMigrateProgress) {
		// on migrate slot progress
		if p.Remain%500 == 0 {
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

func (t *MigrateTask) stop() error {
	if t.Status == MIGRATE_TASK_MIGRATING {
		t.stopChan <- struct{}{}
	}
	return nil
}

// migrate multi slots
func (t *MigrateTask) run() error {
	// create zk conn on demand
	t.zkConn = CreateZkConn()
	defer t.zkConn.Close()

	to := t.NewGroupId
	t.Status = MIGRATE_TASK_MIGRATING
	for slotId := t.FromSlot; slotId <= t.ToSlot; slotId++ {
		err := t.migrateSingleSlot(slotId, to)
		if err == ErrStopMigrateByUser {
			log.Info("stop migration job by user")
			break
		} else if err != nil {
			log.Error(err)
			t.Status = MIGRATE_TASK_ERR
			return err
		}
		t.Percent = (slotId - t.FromSlot + 1) * 100 / (t.ToSlot - t.FromSlot + 1)
		log.Info("total percent:", t.Percent)
	}
	t.Status = MIGRATE_TASK_FINISHED
	log.Info("migration finished")
	return nil
}

func preMigrateCheck(t *MigrateTask) (bool, error) {
	conn := CreateZkConn()
	defer conn.Close()

	slots, err := models.GetMigratingSlots(conn, t.productName)

	if err != nil {
		return false, errors.Trace(err)
	}
	// check if there is migrating slot
	if len(slots) > 1 {
		return false, errors.New("more than one slots are migrating, unknown error")
	}
	if len(slots) == 1 {
		slot := slots[0]
		if t.NewGroupId != slot.State.MigrateStatus.To || t.FromSlot != slot.Id || t.ToSlot != slot.Id {
			return false, errors.Errorf("there is a migrating slot %+v, finish it first", slot)
		}
	}
	return true, nil
}
