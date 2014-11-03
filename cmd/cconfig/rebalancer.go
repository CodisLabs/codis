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

	"fmt"
)

type NodeInfo struct {
	GroupId   int
	CurSlots  []int
	MaxMemory int
}

var ErrNoMaster = fmt.Errorf("group has no master")
var ErrNotAllSlotsOnline = fmt.Errorf("not all slots are online")

func getLivingNodeInfos(zkConn zkhelper.Conn) ([]*NodeInfo, error) {
	groups, err := models.ServerGroups(zkConn, productName)
	if err != nil {
		return nil, err
	}
	slots, err := models.Slots(zkConn, productName)
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
			return nil, errors.Trace(ErrNoMaster)
		}
		log.Info(master.Addr)
		out, err := utils.GetRedisConfig(master.Addr, "maxmemory")
		if err != nil {
			return nil, errors.Trace(err)
		}
		maxMem, err := strconv.Atoi(out)
		if err != nil {
			return nil, errors.Trace(err)
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
	if cnt != 1024 {
		return nil, errors.Trace(ErrNotAllSlotsOnline)
	}
	return ret, nil
}

func getQuotaMap(zkConn zkhelper.Conn) (map[int]int, error) {
	nodes, err := getLivingNodeInfos(zkConn)
	if err != nil {
		return nil, err
	}

	ret := make(map[int]int)
	totalMem := 0
	totalQuota := 0
	for _, node := range nodes {
		totalMem += node.MaxMemory
	}

	for _, node := range nodes {
		quota := int(1024 * node.MaxMemory * 1.0 / totalMem)
		ret[node.GroupId] = quota
		totalQuota += quota
	}

	// round up
	if totalQuota < 1024 {
		for k, _ := range ret {
			ret[k] += 1024 - totalQuota
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
	for _, node := range livingNodes {
		for len(node.CurSlots) > targetQuota[node.GroupId] {
			for _, dest := range livingNodes {
				if dest.GroupId != node.GroupId && len(dest.CurSlots) < targetQuota[dest.GroupId] {
					slot := node.CurSlots[len(node.CurSlots)-1]
					// create a migration task
					t := &MigrateTask{}
					t.Delay = delay
					t.FromSlot = slot
					t.ToSlot = slot
					t.NewGroupId = dest.GroupId
					t.Status = "migrating"
					t.CreateAt = strconv.FormatInt(time.Now().Unix(), 10)
					u, err := uuid.NewV4()
					if err != nil {
						return err
					}
					t.Id = u.String()
					t.stopChan = make(chan struct{})

					if ok, err := preMigrateCheck(t); ok {
						err = RunMigrateTask(t)
						if err != nil {
							log.Warning(err)
							return err
						}
					} else {
						log.Warning(err)
						return err
					}

					node.CurSlots = node.CurSlots[0 : len(node.CurSlots)-1]
					dest.CurSlots = append(dest.CurSlots, slot)
				}
			}
		}
	}
	return nil
}
