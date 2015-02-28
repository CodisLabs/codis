// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"errors"
	"strings"
	"time"

	"github.com/wandoulabs/codis/pkg/models"

	"github.com/garyburd/redigo/redis"
	_ "github.com/juju/errors"
)

const (
	MIGRATE_TIMEOUT = 30000
)

var ErrGroupMasterNotFound = errors.New("group master not found")
var ErrInvalidAddr = errors.New("invalid addr")
var ErrStopMigrateByUser = errors.New("migration stopped by user")

// return: success_count, remain_count, error
// slotsmgrt host port timeout slotnum count
func sendRedisMigrateCmd(c redis.Conn, slotId int, toAddr string) (int, int, error) {
	addrParts := strings.Split(toAddr, ":")
	if len(addrParts) != 2 {
		return -1, -1, ErrInvalidAddr
	}

	reply, err := redis.Values(c.Do("SLOTSMGRTTAGSLOT", addrParts[0], addrParts[1], MIGRATE_TIMEOUT, slotId))
	if err != nil {
		return -1, -1, err
	}

	var succ, remain int
	if _, err := redis.Scan(reply, &succ, &remain); err != nil {
		return -1, -1, err
	}
	return succ, remain, nil
}

// Migrator Implement
type CodisSlotMigrator struct{}

func (m *CodisSlotMigrator) Migrate(slot *models.Slot, fromGroup, toGroup int, task *MigrateTask, onProgress func(SlotMigrateProgress)) (err error) {
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
		if task.stopChan != nil {
			select {
			case <-task.stopChan:
				return ErrStopMigrateByUser
			default:
			}
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
