// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"strconv"
	"time"

	"github.com/juju/errors"
	log "github.com/ngaut/logging"
	"github.com/ngaut/zkhelper"
	"github.com/nu7hatch/gouuid"
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
)

type NodeInfo struct {
	GroupId   int
	CurSlots  []int
	MaxMemory int64
}

func getLivingNodeInfos(zkConn zkhelper.Conn) ([]*NodeInfo, error) {
	groups, err := models.ServerGroups(zkConn, globalEnv.ProductName())
	if err != nil {
		return nil, errors.Trace(err)
	}
	slots, err := models.Slots(zkConn, globalEnv.ProductName())
	slotMap := make(map[int][]int)
	for _, slot := range slots {
		if slot.State.Status == models.SLOT_STATUS_ONLINE {
			slotMap[slot.GroupId] = append(slotMap[slot.GroupId], slot.Id)
		}
	}
	var ret []*NodeInfo
	for _, g := range groups {
		master, err := g.Master(zkConn)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if master == nil {
			return nil, errors.Errorf("group %d has no master", g.Id)
		}
		out, err := utils.GetRedisConfig(master.Addr, "maxmemory")
		if err != nil {
			return nil, errors.Trace(err)
		}
		maxMem, err := strconv.ParseInt(out, 10, 64)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if maxMem <= 0 {
			return nil, errors.Errorf("redis %s should set maxmemory", master.Addr)
		}
		node := &NodeInfo{
			GroupId:   g.Id,
			CurSlots:  slotMap[g.Id],
			MaxMemory: maxMem,
		}
		ret = append(ret, node)
	}
	cnt := 0
	for _, info := range ret {
		cnt += len(info.CurSlots)
	}
	if cnt != models.DEFAULT_SLOT_NUM {
		return nil, errors.New("not all slots are online")
	}
	return ret, nil
}

func getQuotaMap(zkConn zkhelper.Conn) (map[int]int, error) {
	nodes, err := getLivingNodeInfos(zkConn)
	if err != nil {
		return nil, errors.Trace(err)
	}

	ret := make(map[int]int)
	var totalMem int64
	totalQuota := 0
	for _, node := range nodes {
		totalMem += node.MaxMemory
	}

	for _, node := range nodes {
		quota := int(models.DEFAULT_SLOT_NUM * node.MaxMemory * 1.0 / totalMem)
		ret[node.GroupId] = quota
		totalQuota += quota
	}

	// round up
	if totalQuota < models.DEFAULT_SLOT_NUM {
		for k, _ := range ret {
			ret[k] += models.DEFAULT_SLOT_NUM - totalQuota
			break
		}
	}

	return ret, nil
}

// experimental simple auto rebalance :)
func Rebalance(zkConn zkhelper.Conn, delay int) error {
	targetQuota, err := getQuotaMap(zkConn)
	if err != nil {
		return errors.Trace(err)
	}
	livingNodes, err := getLivingNodeInfos(zkConn)
	if err != nil {
		return errors.Trace(err)
	}
	log.Info("start rebalance")
	for _, node := range livingNodes {
		for len(node.CurSlots) > targetQuota[node.GroupId] {
			for _, dest := range livingNodes {
				if dest.GroupId != node.GroupId && len(dest.CurSlots) < targetQuota[dest.GroupId] {
					slot := node.CurSlots[len(node.CurSlots)-1]
					// create a migration task
					t := NewMigrateTask(MigrateTaskInfo{
						Delay:      delay,
						FromSlot:   slot,
						ToSlot:     slot,
						NewGroupId: dest.GroupId,
						Status:     MIGRATE_TASK_MIGRATING,
						CreateAt:   strconv.FormatInt(time.Now().Unix(), 10),
					})
					u, err := uuid.NewV4()
					if err != nil {
						return errors.Trace(err)
					}
					t.Id = u.String()

					if ok, err := preMigrateCheck(t); ok {
						// do migrate
						err := t.run()
						if err != nil {
							log.Warning(err)
							return errors.Trace(err)
						}
					} else {
						log.Warning(err)
						return errors.Trace(err)
					}
					node.CurSlots = node.CurSlots[0 : len(node.CurSlots)-1]
					dest.CurSlots = append(dest.CurSlots, slot)
				}
			}
		}
	}
	log.Info("rebalance finish")
	return nil
}
