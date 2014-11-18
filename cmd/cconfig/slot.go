// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/juju/errors"
	"github.com/wandoulabs/codis/pkg/models"

	"github.com/docopt/docopt-go"
	log "github.com/ngaut/logging"
	"github.com/nu7hatch/gouuid"
)

func cmdSlot(argv []string) (err error) {
	usage := `usage:
	cconfig slot init [-f]
	cconfig slot info <slot_id>
	cconfig slot set <slot_id> <group_id> <status>
	cconfig slot range-set <slot_from> <slot_to> <group_id> <status>
	cconfig slot migrate <slot_from> <slot_to> <group_id> [--delay=<delay_time_in_ms>]
	cconfig slot rebalance [--delay=<delay_time_in_ms>]
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

	zkLock.Lock(fmt.Sprintf("slot, %+v", argv))
	defer func() {
		err := zkLock.Unlock()
		if err != nil {
			log.Error(err)
		}
	}()

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
		return runSlotRangeSet(slotFrom, slotTo, groupId, status)
	}
	return nil
}

func runSlotInit(isForce bool) error {
	if !isForce {
		p := models.GetSlotBasePath(productName)
		exists, _, err := zkConn.Exists(p)
		if err != nil {
			return errors.Trace(err)
		}
		if exists {
			return errors.New("slots already exists. use -f flag to force init")
		}
	}
	err := models.InitSlotSet(zkConn, productName, models.DEFAULT_SLOT_NUM)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func runSlotInfo(slotId int) error {
	s, err := models.GetSlot(zkConn, productName, slotId)
	if err != nil {
		return errors.Trace(err)
	}
	b, _ := json.MarshalIndent(s, " ", "  ")
	fmt.Println(string(b))
	return nil
}

func runSlotRangeSet(fromSlotId, toSlotId int, groupId int, status string) error {
	err := models.SetSlotRange(zkConn, productName, fromSlotId, toSlotId, groupId, models.SlotStatus(status))
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func runSlotSet(slotId int, groupId int, status string) error {
	slot := models.NewSlot(productName, slotId)
	slot.GroupId = groupId
	slot.State.Status = models.SlotStatus(status)
	ts := time.Now().Unix()
	slot.State.LastOpTs = strconv.FormatInt(ts, 10)
	if err := slot.Update(zkConn); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func runSlotMigrate(fromSlotId, toSlotId int, newGroupId int, delay int) error {
	t := &MigrateTask{}
	t.Delay = delay
	t.FromSlot = fromSlotId
	t.ToSlot = toSlotId
	t.NewGroupId = newGroupId
	t.Status = "migrating"
	t.CreateAt = strconv.FormatInt(time.Now().Unix(), 10)
	u, err := uuid.NewV4()
	if err != nil {
		log.Warning(err)
		return errors.Trace(err)
	}
	t.Id = u.String()
	t.stopChan = make(chan struct{})

	// run migrate
	if ok, err := preMigrateCheck(t); ok {
		err = RunMigrateTask(t)
		if err != nil {
			log.Warning(err)
			return errors.Trace(err)
		}
	} else {
		log.Warning(err)
		return errors.Trace(err)
	}
	return nil
}

func runRebalance(delay int) error {
	err := Rebalance(zkConn, delay)
	if err != nil {
		log.Warning(err)
		return errors.Trace(err)
	}
	return nil
}
