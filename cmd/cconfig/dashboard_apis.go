// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"

	"github.com/go-martini/martini"
	"github.com/juju/errors"
	"github.com/nu7hatch/gouuid"

	"github.com/ngaut/go-zookeeper/zk"
	log "github.com/ngaut/logging"
	"github.com/ngaut/zkhelper"
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
		log.Warning(err)
		return 500, err.Error()
	}

	return 200, string(b)
}

func apiOverview() (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	// get all server groups
	groups, err := models.ServerGroups(conn, globalEnv.ProductName())
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
			info, err := utils.GetRedisStat(instance)
			if err != nil {
				log.Error(err)
			}
			redisInfos = append(redisInfos, info)
		}
	}
	info["redis_infos"] = redisInfos

	b, err := json.MarshalIndent(info, " ", "  ")
	return 200, string(b)
}

func apiGetServerGroupList() (int, string) {
	conn := CreateZkConn()
	defer conn.Close()
	groups, err := models.ServerGroups(conn, globalEnv.ProductName())
	if err != nil {
		log.Warning(err)
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

	conn := CreateZkConn()
	defer conn.Close()

	if !isForce {
		s, _ := models.Slots(conn, globalEnv.ProductName())
		if len(s) > 0 {
			return 500, "slots already initialized, you may use 'is_force' flag and try again."
		}
	}

	if err := models.InitSlotSet(conn, globalEnv.ProductName(), models.DEFAULT_SLOT_NUM); err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiRedisStat(param martini.Params) (int, string) {
	addr := param["addr"]
	info, err := utils.GetRedisStat(addr)
	if err != nil {
		return 500, err.Error()
	}
	b, _ := json.MarshalIndent(info, " ", "  ")
	return 200, string(b)
}

func apiDoMigrate(taskForm MigrateTaskInfo, param martini.Params) (int, string) {
	// do migrate async
	taskForm.Status = MIGRATE_TASK_PENDING
	taskForm.CreateAt = strconv.FormatInt(time.Now().Unix(), 10)
	u, err := uuid.NewV4()
	if err != nil {
		return 500, err.Error()
	}
	taskForm.Id = u.String()
	task := NewMigrateTask(taskForm)
	globalMigrateManager.PostTask(task)
	return jsonRetSucc()
}

var isRebalancing bool
var rebalanceLck = sync.Mutex{}

func changeRebalanceStat(b bool) {
	rebalanceLck.Lock()
	defer rebalanceLck.Unlock()
	isRebalancing = b
}

func isOnRebalancing() bool {
	rebalanceLck.Lock()
	defer rebalanceLck.Unlock()
	return isRebalancing
}

func apiRebalanceStatus(param martini.Params) (int, string) {
	ret := map[string]interface{}{
		"is_rebalancing": isRebalancing,
	}
	b, _ := json.MarshalIndent(ret, " ", "  ")
	return 200, string(b)
}

func apiRebalance(param martini.Params) (int, string) {
	if isOnRebalancing() {
		return 500, "rebalancing..."
	}

	go func() {
		changeRebalanceStat(true)
		defer changeRebalanceStat(false)

		conn := CreateZkConn()
		defer conn.Close()

		if err := Rebalance(conn, 0); err != nil {
			log.Warning(errors.ErrorStack(err))
		}
	}()

	return jsonRetSucc()
}

func apiGetMigrateTasks() (int, string) {
	tasks := globalMigrateManager.Tasks()
	b, _ := json.MarshalIndent(tasks, " ", "  ")
	return 200, string(b)
}

func apiRemovePendingMigrateTask(param martini.Params) (int, string) {
	id := param["id"]
	if err := globalMigrateManager.RemovePendingTask(id); err != nil {
		return 500, "Error removing task: " + err.Error()
	}
	return jsonRetSucc()
}

func apiStopMigratingTask(param martini.Params) (int, string) {
	if err := globalMigrateManager.StopRunningTask(); err != nil {
		return 500, "Error stopping migrate task: " + err.Error()
	}
	return jsonRetSucc()
}

func apiGetServerGroup(param martini.Params) (int, string) {
	id := param["id"]
	groupId, err := strconv.Atoi(id)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	conn := CreateZkConn()
	defer conn.Close()
	group, err := models.GetGroup(conn, globalEnv.ProductName(), groupId)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	b, err := json.MarshalIndent(group, " ", "  ")
	return 200, string(b)
}

func apiMigrateStatus() (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	migrateSlots, err := models.GetMigratingSlots(conn, globalEnv.ProductName())
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
		log.Warning(err)
		return 500, err.Error()
	}
	slotInfo, err := utils.SlotsInfo(addr, slotId, slotId)
	if err != nil {
		log.Warning(err)
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
		log.Warning(err)
		return 500, err.Error()
	}
	slotId, err := strconv.Atoi(param["slot_id"])
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	conn := CreateZkConn()
	defer conn.Close()

	g, err := models.GetGroup(conn, globalEnv.ProductName(), groupId)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}

	s, err := g.Master(conn)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}

	if s == nil {
		return 500, "master not found"
	}

	slotInfo, err := utils.SlotsInfo(s.Addr, slotId, slotId)
	if err != nil {
		log.Warning(err)
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
	conn := CreateZkConn()
	defer conn.Close()

	lock := utils.GetZkLock(conn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("removing group %s", param["id"]))

	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	groupId, _ := strconv.Atoi(param["id"])
	serverGroup := models.NewServerGroup(globalEnv.ProductName(), groupId)
	if err := serverGroup.Remove(conn); err != nil {
		log.Error(errors.ErrorStack(err))
		return 500, err.Error()
	}

	return jsonRetSucc()
}

// create new server group
func apiAddServerGroup(newGroup models.ServerGroup) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	lock := utils.GetZkLock(conn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("add group %+v", newGroup))

	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	newGroup.ProductName = globalEnv.ProductName()

	exists, err := newGroup.Exists(conn)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	if exists {
		return 500, "group already exists"
	}
	err = newGroup.Create(conn)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	return jsonRetSucc()
}

// add redis server to exist server group
func apiAddServerToGroup(server models.Server, param martini.Params) (int, string) {
	groupId, _ := strconv.Atoi(param["id"])

	conn := CreateZkConn()
	defer conn.Close()

	lock := utils.GetZkLock(conn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("add server to group,  %+v", server))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()
	// check group exists first
	serverGroup := models.NewServerGroup(globalEnv.ProductName(), groupId)

	exists, err := serverGroup.Exists(conn)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}

	// create new group if not exists
	if !exists {
		if err := serverGroup.Create(conn); err != nil {
			return 500, err.Error()
		}
	}

	if err := serverGroup.AddServer(conn, &server); err != nil {
		log.Warning(errors.ErrorStack(err))
		return 500, err.Error()
	}

	return jsonRetSucc()
}

func apiPromoteServer(server models.Server, param martini.Params) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	lock := utils.GetZkLock(conn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("promote server %+v", server))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	group, err := models.GetGroup(conn, globalEnv.ProductName(), server.GroupId)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	err = group.Promote(conn, server.Addr)
	if err != nil {
		log.Warning(errors.ErrorStack(err))
		log.Warning(err)
		return 500, err.Error()
	}

	return jsonRetSucc()
}

func apiRemoveServerFromGroup(server models.Server, param martini.Params) (int, string) {
	groupId, _ := strconv.Atoi(param["id"])

	conn := CreateZkConn()
	defer conn.Close()

	lock := utils.GetZkLock(conn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("removing server from group, %+v", server))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	serverGroup := models.NewServerGroup(globalEnv.ProductName(), groupId)
	err := serverGroup.RemoveServer(conn, server.Addr)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiSetProxyStatus(proxy models.ProxyInfo, param martini.Params) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()
	err := models.SetProxyStatus(conn, globalEnv.ProductName(), proxy.Id, proxy.State)
	if err != nil {
		// if this proxy is not online, just return success
		if proxy.State == models.PROXY_STATE_MARK_OFFLINE && zkhelper.ZkErrorEqual(err, zk.ErrNoNode) {
			return jsonRetSucc()
		}
		log.Warning(errors.ErrorStack(err))
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiGetProxyList(param martini.Params) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	proxies, err := models.ProxyList(conn, globalEnv.ProductName(), nil)
	if err != nil {
		log.Warning(err)
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
	conn := CreateZkConn()
	defer conn.Close()

	slot, err := models.GetSlot(conn, globalEnv.ProductName(), id)
	if err != nil {
		log.Warning(errors.Trace(err))
		return 500, err.Error()
	}

	b, err := json.MarshalIndent(slot, " ", "  ")
	return 200, string(b)
}

func apiGetSlots() (int, string) {
	conn := CreateZkConn()
	defer conn.Close()
	slots, err := models.Slots(conn, globalEnv.ProductName())
	if err != nil {
		log.Warning("Error getting slot info, try init slots first? err: ", err)
		return 500, err.Error()
	}
	b, err := json.MarshalIndent(slots, " ", "  ")
	return 200, string(b)
}

func apiSlotRangeSet(task RangeSetTask) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	lock := utils.GetZkLock(conn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("set slot range, %+v", task))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	// default set online
	if len(task.Status) == 0 {
		task.Status = string(models.SLOT_STATUS_ONLINE)
	}

	err := models.SetSlotRange(conn, globalEnv.ProductName(), task.FromSlot, task.ToSlot, task.NewGroupId, models.SlotStatus(task.Status))
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}

	return jsonRetSucc()
}

// actions
func apiActionGC(r *http.Request) (int, string) {
	r.ParseForm()
	keep, _ := strconv.Atoi(r.FormValue("keep"))
	secs, _ := strconv.Atoi(r.FormValue("secs"))

	conn := CreateZkConn()
	defer conn.Close()
	lock := utils.GetZkLock(conn, globalEnv.ProductName())
	lock.Lock(fmt.Sprintf("action gc"))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	var err error
	if keep >= 0 {
		err = models.ActionGC(conn, globalEnv.ProductName(), models.GC_TYPE_N, keep)
	} else if secs > 0 {
		err = models.ActionGC(conn, globalEnv.ProductName(), models.GC_TYPE_SEC, secs)
	}
	if err != nil {
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiForceRemoveLocks() (int, string) {
	conn := CreateZkConn()
	defer conn.Close()
	err := models.ForceRemoveLock(conn, globalEnv.ProductName())
	if err != nil {
		log.Warning(errors.ErrorStack(err))
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiRemoveFence() (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	err := models.ForceRemoveDeadFence(conn, globalEnv.ProductName())
	if err != nil {
		log.Warning(errors.ErrorStack(err))
		return 500, err.Error()
	}
	return jsonRetSucc()

}
