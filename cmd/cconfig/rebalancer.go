// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"strconv"
	"time"

	"github.com/wandoulabs/zkhelper"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
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
		out, err := utils.GetRedisConfig(master.Addr, globalEnv.Password(), "maxmemory")
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
		return nil, errors.Errorf("not all slots are online")
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
func Rebalance() error {
	targetQuota, err := getQuotaMap(safeZkConn)
	if err != nil {
		return errors.Trace(err)
	}
	livingNodes, err := getLivingNodeInfos(safeZkConn)
	if err != nil {
		return errors.Trace(err)
	}
	log.Infof("start rebalance")
	for _, node := range livingNodes {
		for len(node.CurSlots) > targetQuota[node.GroupId] {
			for _, dest := range livingNodes {
				if dest.GroupId != node.GroupId && len(dest.CurSlots) < targetQuota[dest.GroupId] {
					slot := node.CurSlots[len(node.CurSlots)-1]
					// create a migration task
					info := &MigrateTaskInfo{
						Delay:      0,
						SlotId:     slot,
						NewGroupId: dest.GroupId,
						Status:     MIGRATE_TASK_PENDING,
						CreateAt:   strconv.FormatInt(time.Now().Unix(), 10),
					}
					globalMigrateManager.PostTask(info)

					node.CurSlots = node.CurSlots[0 : len(node.CurSlots)-1]
					dest.CurSlots = append(dest.CurSlots, slot)
				}
			}
		}
	}
	log.Infof("rebalance tasks submit finish")
	return nil
}
