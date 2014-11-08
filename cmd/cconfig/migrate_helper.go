// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"errors"
	"strings"
	"time"

	"github.com/ngaut/zkhelper"
	"github.com/wandoulabs/codis/pkg/models"

	log "github.com/ngaut/logging"

	"github.com/garyburd/redigo/redis"
	_ "github.com/juju/errors"
)

const (
	MIGRATE_TIMEOUT = 30000
)

var ErrGroupMasterNotFound = errors.New("group master not found")
var ErrInvalidAddr = errors.New("invalid addr")

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

var ErrStopMigrateByUser = errors.New("migration stop by user")

func MigrateSingleSlot(zkConn zkhelper.Conn, slotId, fromGroup, toGroup int, delay int, stopChan <-chan struct{}) error {
	groupFrom, err := models.GetGroup(zkConn, productName, fromGroup)
	if err != nil {
		return err
	}
	groupTo, err := models.GetGroup(zkConn, productName, toGroup)
	if err != nil {
		return err
	}

	fromMaster, err := groupFrom.Master(zkConn)
	if err != nil {
		return err
	}

	toMaster, err := groupTo.Master(zkConn)
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

	_, remain, err := sendRedisMigrateCmd(c, slotId, toMaster.Addr)
	if err != nil {
		return err
	}

	for remain > 0 {
		if delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
		if stopChan != nil {
			select {
			case <-stopChan:
				return ErrStopMigrateByUser
			default:
			}
		}
		_, remain, err = sendRedisMigrateCmd(c, slotId, toMaster.Addr)
		if remain%500 == 0 && remain > 0 {
			log.Info("remain:", remain)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
