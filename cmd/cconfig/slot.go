// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"
	"strconv"

	"github.com/juju/errors"

	"github.com/docopt/docopt-go"
	log "github.com/ngaut/logging"
)

func cmdSlot(argv []string) (err error) {
	usage := `usage:
	codis-config slot init [-f]
	codis-config slot info <slot_id>
	codis-config slot set <slot_id> <group_id> <status>
	codis-config slot range-set <slot_from> <slot_to> <group_id> <status>
	codis-config slot migrate <slot_from> <slot_to> <group_id> [--delay=<delay_time_in_ms>]
	codis-config slot rebalance [--delay=<delay_time_in_ms>]
`

	args, err := docopt.Parse(usage, argv, true, "", false)
	if err != nil {
		log.Error(err)
		return errors.Trace(err)
	}
	log.Debug(args)

	// no need to lock here
	// locked in runmigratetask
	if args["migrate"].(bool) {
		delay := 0
		groupId, err := strconv.Atoi(args["<group_id>"].(string))
		if args["--delay"] != nil {
			delay, err = strconv.Atoi(args["--delay"].(string))
			if err != nil {
				log.Warning(err)
				return errors.Trace(err)
			}
		}
		slotFrom, err := strconv.Atoi(args["<slot_from>"].(string))
		if err != nil {
			log.Warning(err)
			return errors.Trace(err)
		}

		slotTo, err := strconv.Atoi(args["<slot_to>"].(string))
		if err != nil {
			log.Warning(err)
			return errors.Trace(err)
		}
		return runSlotMigrate(slotFrom, slotTo, groupId, delay)
	}
	if args["rebalance"].(bool) {
		delay := 0
		if args["--delay"] != nil {
			delay, err = strconv.Atoi(args["--delay"].(string))
			if err != nil {
				log.Warning(err)
				return errors.Trace(err)
			}
		}
		return runRebalance(delay)
	}

	if args["init"].(bool) {
		force := args["-f"].(bool)
		return runSlotInit(force)
	}

	if args["info"].(bool) {
		slotId, err := strconv.Atoi(args["<slot_id>"].(string))
		if err != nil {
			log.Warning(err)
			return errors.Trace(err)
		}
		return runSlotInfo(slotId)
	}

	groupId, err := strconv.Atoi(args["<group_id>"].(string))
	if err != nil {
		log.Warning(err)
		return errors.Trace(err)
	}

	if args["set"].(bool) {
		slotId, err := strconv.Atoi(args["<slot_id>"].(string))
		status := args["<status>"].(string)
		if err != nil {
			log.Warning(err)
			return errors.Trace(err)
		}
		return runSlotSet(slotId, groupId, status)
	}

	if args["range-set"].(bool) {
		status := args["<status>"].(string)
		slotFrom, err := strconv.Atoi(args["<slot_from>"].(string))
		if err != nil {
			log.Warning(err)
			return errors.Trace(err)
		}
		slotTo, err := strconv.Atoi(args["<slot_to>"].(string))
		if err != nil {
			log.Warning(err)
			return errors.Trace(err)
		}
		return errors.Trace(runSlotRangeSet(slotFrom, slotTo, groupId, status))
	}
	return nil
}

func runSlotInit(isForce bool) error {
	var v interface{}
	url := "/api/slots/init"
	if isForce {
		url += "?is_force=1"
	}
	err := callApi(METHOD_POST, url, nil, &v)
	if err != nil {
		return errors.Trace(err)
	}
	fmt.Println(jsonify(v))
	return nil
}

func runSlotInfo(slotId int) error {
	var v interface{}
	err := callApi(METHOD_GET, fmt.Sprintf("/api/slot/%d", slotId), nil, &v)
	if err != nil {
		return errors.Trace(err)
	}
	fmt.Println(jsonify(v))
	return nil
}

func runSlotRangeSet(fromSlotId, toSlotId int, groupId int, status string) error {
	t := RangeSetTask{
		FromSlot:   fromSlotId,
		ToSlot:     toSlotId,
		NewGroupId: groupId,
		Status:     status,
	}

	var v interface{}
	err := callApi(METHOD_POST, "/api/slot", t, &v)
	if err != nil {
		return errors.Trace(err)
	}
	fmt.Println(jsonify(v))
	return nil
}

func runSlotSet(slotId int, groupId int, status string) error {
	return runSlotRangeSet(slotId, slotId, groupId, status)
}

func runSlotMigrate(fromSlotId, toSlotId int, newGroupId int, delay int) error {
	migrateInfo := &MigrateTaskInfo{
		FromSlot:   fromSlotId,
		ToSlot:     toSlotId,
		NewGroupId: newGroupId,
		Delay:      delay,
	}

	var v interface{}
	err := callApi(METHOD_POST, "/api/migrate", migrateInfo, &v)
	if err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}

func runRebalance(delay int) error {
	var v interface{}
	err := callApi(METHOD_POST, "/api/rebalance", nil, &v)
	if err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}
