// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-martini/martini"

	"github.com/wandoulabs/go-zookeeper/zk"
	"github.com/wandoulabs/zkhelper"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

var globalMigrateManager *MigrateManager

type RangeSetTask struct {
	FromSlot   int    `json:"from"`
	ToSlot     int    `json:"to"`
	NewGroupId int    `json:"new_group"`
	Status     string `json:"status"`
}

func apiGetProxyDebugVars() (int, string) {
	m := getAllProxyDebugVars()
	if m == nil {
		return 500, "Error getting proxy debug vars"
	}

	b, err := json.MarshalIndent(m, " ", "  ")
	if err != nil {
		log.WarnErrorf(err, "to json failed")
		return 500, err.Error()
	}

	return 200, string(b)
}

func apiOverview() (int, string) {
	// get all server groups
	groups, err := models.ServerGroups(unsafeZkConn, globalEnv.ProductName())
	if err != nil && !zkhelper.ZkErrorEqual(err, zk.ErrNoNode) {
		return 500, err.Error()
	}

	var instances []string

	for _, group := range groups {
		for _, srv := range group.Servers {
			if srv.Type == "master" {
				instances = append(instances, srv.Addr)
			}
		}
	}

	info := make(map[string]interface{})
	info["product"] = globalEnv.ProductName()
	info["ops"] = proxiesSpeed

	redisInfos := make([]map[string]string, 0)

	if len(instances) > 0 {
		for _, instance := range instances {
			info, err := utils.GetRedisStat(instance, globalEnv.Password())
			if err != nil {
				log.ErrorErrorf(err, "get redis stat failed")
			}
			redisInfos = append(redisInfos, info)
		}
	}
	info["redis_infos"] = redisInfos

	b, err := json.MarshalIndent(info, " ", "  ")
	return 200, string(b)
}

func apiGetServerGroupList() (int, string) {
	groups, err := models.ServerGroups(safeZkConn, globalEnv.ProductName())
	if err != nil {
		log.ErrorErrorf(err, "get server groups failed")
		return 500, err.Error()
	}
	b, err := json.MarshalIndent(groups, " ", "  ")
	return 200, string(b)
}

func apiInitSlots(r *http.Request) (int, string) {
	r.ParseForm()
	isForce := false
	val := r.FormValue("is_force")
	if len(val) > 0 && (val == "1" || val == "true") {
		isForce = true
	}
	if !isForce {
		s, _ := models.Slots(safeZkConn, globalEnv.ProductName())
		if len(s) > 0 {
			return 500, "slots already initialized, you may use 'is_force' flag and try again."
		}
	}

	if err := models.InitSlotSet(safeZkConn, globalEnv.ProductName(), models.DEFAULT_SLOT_NUM); err != nil {
		log.ErrorErrorf(err, "init slot set failed")
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiRedisStat(param martini.Params) (int, string) {
	addr := param["addr"]
	info, err := utils.GetRedisStat(addr, globalEnv.Password())
	if err != nil {
		return 500, err.Error()
	}
	b, _ := json.MarshalIndent(info, " ", "  ")
	return 200, string(b)
}

type migrateTaskForm struct {
	From  int `json:"from"`
	To    int `json:"to"`
	Group int `json:"new_group"`
	Delay int `json:"delay"`
}

func apiDoMigrate(form migrateTaskForm) (int, string) {
	for i := form.From; i <= form.To; i++ {
		task := MigrateTaskInfo{
			SlotId:     i,
			Delay:      form.Delay,
			NewGroupId: form.Group,
			Status:     MIGRATE_TASK_PENDING,
			CreateAt:   strconv.FormatInt(time.Now().Unix(), 10),
		}
		globalMigrateManager.PostTask(&task)
	}
	// do migrate async
	return jsonRetSucc()
}

func apiRebalance(param martini.Params) (int, string) {
	if len(globalMigrateManager.Tasks()) > 0 {
		return 500, "there are migration tasks running, you should wait them done"
	}
	if err := Rebalance(); err != nil {
		log.ErrorErrorf(err, "rebalance failed")
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiGetMigrateTasks() (int, string) {
	tasks := globalMigrateManager.Tasks()
	b, _ := json.MarshalIndent(tasks, " ", "  ")
	return 200, string(b)
}

func apiGetServerGroup(param martini.Params) (int, string) {
	id := param["id"]
	groupId, err := strconv.Atoi(id)
	if err != nil {
		log.ErrorErrorf(err, "parse groupid failed")
		return 500, err.Error()
	}
	group, err := models.GetGroup(safeZkConn, globalEnv.ProductName(), groupId)
	if err != nil {
		log.ErrorErrorf(err, "get group %d failed", groupId)
		return 500, err.Error()
	}
	b, err := json.MarshalIndent(group, " ", "  ")
	return 200, string(b)
}

func apiMigrateStatus() (int, string) {
	migrateSlots, err := models.GetMigratingSlots(safeZkConn, globalEnv.ProductName())
	if err != nil && !zkhelper.ZkErrorEqual(err, zk.ErrNoNode) {
		return 500, err.Error()
	}

	b, err := json.MarshalIndent(map[string]interface{}{
		"migrate_slots": migrateSlots,
		"migrate_task":  globalMigrateManager.runningTask,
	}, " ", "  ")
	return 200, string(b)
}

func apiGetRedisSlotInfo(param martini.Params) (int, string) {
	addr := param["addr"]
	slotId, err := strconv.Atoi(param["id"])
	if err != nil {
		log.ErrorErrorf(err, "parse slotid failed")
		return 500, err.Error()
	}
	slotInfo, err := utils.SlotsInfo(addr, globalEnv.Password(), slotId, slotId)
	if err != nil {
		log.ErrorErrorf(err, "get slot info %d failed", slotId)
		return 500, err.Error()
	}
	out, _ := json.MarshalIndent(map[string]interface{}{
		"keys":    slotInfo[slotId],
		"slot_id": slotId,
	}, " ", "  ")
	return 200, string(out)
}

func apiGetRedisSlotInfoFromGroupId(param martini.Params) (int, string) {
	groupId, err := strconv.Atoi(param["group_id"])
	if err != nil {
		log.ErrorErrorf(err, "parse groupid failed")
		return 500, err.Error()
	}
	slotId, err := strconv.Atoi(param["slot_id"])
	if err != nil {
		log.ErrorErrorf(err, "parse slotid failed")
		return 500, err.Error()
	}
	g, err := models.GetGroup(safeZkConn, globalEnv.ProductName(), groupId)
	if err != nil {
		log.ErrorErrorf(err, "get group %d failed", groupId)
		return 500, err.Error()
	}

	s, err := g.Master(safeZkConn)
	if err != nil {
		log.ErrorErrorf(err, "get master of group %d failed", groupId)
		return 500, err.Error()
	}

	if s == nil {
		return 500, "master not found"
	}

	slotInfo, err := utils.SlotsInfo(s.Addr, globalEnv.Password(), slotId, slotId)
	if err != nil {
		log.ErrorErrorf(err, "get slot info %d failed", slotId)
		return 500, err.Error()
	}

	out, _ := json.MarshalIndent(map[string]interface{}{
		"keys":     slotInfo[slotId],
		"slot_id":  slotId,
		"group_id": groupId,
		"addr":     s.Addr,
	}, " ", "  ")
	return 200, string(out)

}

func apiRemoveServerGroup(param martini.Params) (int, string) {
	lock := utils.GetZkLock(safeZkConn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("removing group %s", param["id"]))

	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.ErrorErrorf(err, "unlock node failed")
		}
	}()

	groupId, _ := strconv.Atoi(param["id"])
	serverGroup := models.NewServerGroup(globalEnv.ProductName(), groupId)
	if err := serverGroup.Remove(safeZkConn); err != nil {
		log.ErrorErrorf(err, "remove server group failed")
		return 500, err.Error()
	}

	return jsonRetSucc()
}

// create new server group
func apiAddServerGroup(newGroup models.ServerGroup) (int, string) {
	lock := utils.GetZkLock(safeZkConn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("add group %+v", newGroup))

	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.ErrorErrorf(err, "unlock node failed")
		}
	}()

	newGroup.ProductName = globalEnv.ProductName()

	exists, err := newGroup.Exists(safeZkConn)
	if err != nil {
		log.ErrorErrorf(err, "check group exits failed")
		return 500, err.Error()
	}
	if exists {
		return 500, "group already exists"
	}
	err = newGroup.Create(safeZkConn)
	if err != nil {
		log.ErrorErrorf(err, "create node for new group failed")
		return 500, err.Error()
	}
	return jsonRetSucc()
}

// add redis server to exist server group
func apiAddServerToGroup(server models.Server, param martini.Params) (int, string) {
	groupId, _ := strconv.Atoi(param["id"])
	lock := utils.GetZkLock(safeZkConn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("add server to group,  %+v", server))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.ErrorErrorf(err, "unlock node failed")
		}
	}()
	// check group exists first
	serverGroup := models.NewServerGroup(globalEnv.ProductName(), groupId)

	exists, err := serverGroup.Exists(safeZkConn)
	if err != nil {
		log.ErrorErrorf(err, "check group exits failed")
		return 500, err.Error()
	}

	// create new group if not exists
	if !exists {
		if err := serverGroup.Create(safeZkConn); err != nil {
			return 500, err.Error()
		}
	}

	if err := serverGroup.AddServer(safeZkConn, &server, globalEnv.Password()); err != nil {
		log.ErrorErrorf(err, "add server to group failed")
		return 500, err.Error()
	}

	return jsonRetSucc()
}

func apiPromoteServer(server models.Server, param martini.Params) (int, string) {
	lock := utils.GetZkLock(safeZkConn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("promote server %+v", server))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.ErrorErrorf(err, "unlock node failed")
		}
	}()

	group, err := models.GetGroup(safeZkConn, globalEnv.ProductName(), server.GroupId)
	if err != nil {
		log.ErrorErrorf(err, "get group %d failed", server.GroupId)
		return 500, err.Error()
	}
	err = group.Promote(safeZkConn, server.Addr, globalEnv.Password())
	if err != nil {
		log.ErrorErrorf(err, "promote group %d failed", server.GroupId)
		return 500, err.Error()
	}

	return jsonRetSucc()
}

func apiRemoveServerFromGroup(server models.Server, param martini.Params) (int, string) {
	groupId, _ := strconv.Atoi(param["id"])
	lock := utils.GetZkLock(safeZkConn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("removing server from group, %+v", server))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.ErrorErrorf(err, "unlock node failed")
		}
	}()

	serverGroup := models.NewServerGroup(globalEnv.ProductName(), groupId)
	err := serverGroup.RemoveServer(safeZkConn, server.Addr)
	if err != nil {
		log.ErrorErrorf(err, "remove group %d failed", groupId)
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiSetProxyStatus(proxy models.ProxyInfo, param martini.Params) (int, string) {
	err := models.SetProxyStatus(safeZkConn, globalEnv.ProductName(), proxy.Id, proxy.State)
	if err != nil {
		// if this proxy is not online, just return success
		if proxy.State == models.PROXY_STATE_MARK_OFFLINE && zkhelper.ZkErrorEqual(err, zk.ErrNoNode) {
			return jsonRetSucc()
		}
		log.ErrorErrorf(err, "set proxy states failed: %+v", proxy)
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiGetProxyList(param martini.Params) (int, string) {
	proxies, err := models.ProxyList(safeZkConn, globalEnv.ProductName(), nil)
	if err != nil {
		log.ErrorErrorf(err, "get proxy list failed")
		return 500, err.Error()
	}
	b, err := json.MarshalIndent(proxies, " ", "  ")
	return 200, string(b)
}

func apiGetSingleSlot(param martini.Params) (int, string) {
	id, err := strconv.Atoi(param["id"])
	if err != nil {
		return 500, err.Error()
	}

	slot, err := models.GetSlot(safeZkConn, globalEnv.ProductName(), id)
	if err != nil {
		log.ErrorErrorf(err, "get slot %d failed", id)
		return 500, err.Error()
	}

	b, err := json.MarshalIndent(slot, " ", "  ")
	return 200, string(b)
}

func apiGetSlots() (int, string) {
	slots, err := models.Slots(safeZkConn, globalEnv.ProductName())
	if err != nil {
		log.ErrorErrorf(err, "Error getting slot info, try init slots first?")
		return 500, err.Error()
	}
	b, err := json.MarshalIndent(slots, " ", "  ")
	return 200, string(b)
}

func apiSlotRangeSet(task RangeSetTask) (int, string) {
	lock := utils.GetZkLock(safeZkConn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("set slot range, %+v", task))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.ErrorErrorf(err, "unlock node failed")
		}
	}()

	// default set online
	if len(task.Status) == 0 {
		task.Status = string(models.SLOT_STATUS_ONLINE)
	}

	err := models.SetSlotRange(safeZkConn, globalEnv.ProductName(), task.FromSlot, task.ToSlot, task.NewGroupId, models.SlotStatus(task.Status))
	if err != nil {
		log.ErrorErrorf(err, "set slot range [%d,%d] failed", task.FromSlot, task.ToSlot)
		return 500, err.Error()
	}

	return jsonRetSucc()
}

// actions
func apiActionGC(r *http.Request) (int, string) {
	r.ParseForm()
	keep, _ := strconv.Atoi(r.FormValue("keep"))
	secs, _ := strconv.Atoi(r.FormValue("secs"))
	lock := utils.GetZkLock(safeZkConn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("action gc"))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.ErrorErrorf(err, "unlock node failed")
		}
	}()

	var err error
	if keep >= 0 {
		err = models.ActionGC(safeZkConn, globalEnv.ProductName(), models.GC_TYPE_N, keep)
	} else if secs > 0 {
		err = models.ActionGC(safeZkConn, globalEnv.ProductName(), models.GC_TYPE_SEC, secs)
	}
	if err != nil {
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiForceRemoveLocks() (int, string) {
	err := models.ForceRemoveLock(safeZkConn, globalEnv.ProductName())
	if err != nil {
		log.ErrorErrorf(err, "force remove lock failed")
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiRemoveFence() (int, string) {
	err := models.ForceRemoveDeadFence(safeZkConn, globalEnv.ProductName())
	if err != nil {
		log.ErrorErrorf(err, "force remove fence failed")
		return 500, err.Error()
	}
	return jsonRetSucc()

}
