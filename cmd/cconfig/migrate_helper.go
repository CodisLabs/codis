// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"errors"
	"strings"

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
