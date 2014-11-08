// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"

	"github.com/go-martini/martini"
	"github.com/nu7hatch/gouuid"

	log "github.com/ngaut/logging"
)

type RangeSetTask struct {
	FromSlot   int `json:"from"`
	ToSlot     int `json:"to"`
	NewGroupId int `json:"new_group"`
}

func apiGetProxyDebugVars() (int, string) {
	m := getAllProxyDebugVars()
	if m == nil {
		return 500, "get proxy debug var error"
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
	groups, err := models.ServerGroups(conn, productName)
	if err != nil {
		log.Warning("get server groups error, maybe there is no any server groups? err:", err)
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

	var info map[string]interface{} = make(map[string]interface{})
	info["product"] = productName
	info["ops"] = proxiesSpeed

	var redisInfos []map[string]string = make([]map[string]string, 0)

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
	groups, err := models.ServerGroups(conn, productName)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	b, err := json.MarshalIndent(groups, " ", "  ")
	return 200, string(b)
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

func apiDoMigrate(taskForm MigrateTaskForm, param martini.Params) (int, string) {
	// do migrate async
	taskForm.Percent = 0
	taskForm.Status = "pending"
	taskForm.CreateAt = strconv.FormatInt(time.Now().Unix(), 10)
	u, err := uuid.NewV4()
	if err != nil {
		return 500, err.Error()
	}
	taskForm.Id = u.String()
	task := &MigrateTask{
		MigrateTaskForm: taskForm,
		stopChan:        make(chan struct{}),
	}

	lck.Lock()
	pendingMigrateTask.PushBack(task)
	lck.Unlock()

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
			log.Warning(err.Error())
		}
	}()

	return jsonRetSucc()
}

func apiGetMigrateTasks() (int, string) {
	lck.RLock()
	defer lck.RUnlock()

	var tasks []*MigrateTask
	for e := pendingMigrateTask.Front(); e != nil; e = e.Next() {
		tasks = append(tasks, e.Value.(*MigrateTask))
	}

	b, _ := json.MarshalIndent(tasks, " ", "  ")

	return 200, string(b)
}

func apiRemovePendingMigrateTask(param martini.Params) (int, string) {
	lck.Lock()
	defer lck.Unlock()
	id := param["id"]
	if removePendingMigrateTask(id) == true {
		return jsonRetSucc()
	}
	return 500, "remove task error"
}

func apiStopMigratingTask(param martini.Params) (int, string) {
	lck.RLock()
	defer lck.RUnlock()
	if curMigrateTask != nil {
		curMigrateTask.Status = "stopping"
		curMigrateTask.stopChan <- struct{}{}
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
	group, err := models.GetGroup(conn, productName, groupId)
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

	migrateSlots, err := models.GetMigratingSlots(conn, productName)
	if err != nil {
		log.Warning("get slots info error, maybe init slots first? err: ", err)
		return 500, err.Error()
	}

	b, err := json.MarshalIndent(map[string]interface{}{
		"migrate_slots": migrateSlots,
		"migrate_task":  curMigrateTask,
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

	g, err := models.GetGroup(conn, productName, groupId)
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
		log.Warning("this group has no master server")
		return 500, "this group has no master server"
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

	lock := utils.GetZkLock(conn, productName)
	lock.Lock(fmt.Sprintf("remove group %s", param["id"]))

	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	groupId, _ := strconv.Atoi(param["id"])
	serverGroup := models.NewServerGroup(productName, groupId)
	if err := serverGroup.Remove(conn); err != nil {
		log.Warning(err)
		return 500, err.Error()
	}

	return jsonRetSucc()
}

// create new server group
func apiAddServerGroup(newGroup models.ServerGroup) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	lock := utils.GetZkLock(conn, productName)
	lock.Lock(fmt.Sprintf("add group %+v", newGroup))

	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	newGroup.ProductName = productName

	exists, err := newGroup.Exists(conn)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	if exists {
		return jsonRet(map[string]interface{}{
			"ret": 0,
			"msg": "group already exists",
		})
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

	lock := utils.GetZkLock(conn, productName)
	lock.Lock(fmt.Sprintf("add server to group,  %+v", server))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()
	// check group exists first
	serverGroup := models.NewServerGroup(productName, groupId)

	exists, err := serverGroup.Exists(conn)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	if !exists {
		return jsonRetFail(-1, "group not exists")
	}

	if err := serverGroup.AddServer(conn, &server); err != nil {
		log.Warning(err)
		return 500, err.Error()
	}

	return jsonRetSucc()
}

func apiPromoteServer(server models.Server, param martini.Params) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	lock := utils.GetZkLock(conn, productName)
	lock.Lock(fmt.Sprintf("promote server %+v", server))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	group, err := models.GetGroup(conn, productName, server.GroupId)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	err = group.Promote(conn, server.Addr)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}

	return jsonRetSucc()
}

func apiRemoveServerFromGroup(server models.Server, param martini.Params) (int, string) {
	groupId, _ := strconv.Atoi(param["id"])

	conn := CreateZkConn()
	defer conn.Close()

	lock := utils.GetZkLock(conn, productName)
	lock.Lock(fmt.Sprintf("remove server from group, %+v", server))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	serverGroup := models.NewServerGroup(productName, groupId)
	err := serverGroup.RemoveServer(conn, server)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiSetProxyStatus(proxy models.ProxyInfo, param martini.Params) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()
	err := models.SetProxyStatus(conn, productName, proxy.Id, proxy.State)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	return jsonRetSucc()
}

func apiGetProxyList(param martini.Params) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()

	proxies, err := models.ProxyList(conn, productName, nil)
	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}
	b, err := json.MarshalIndent(proxies, " ", "  ")
	return 200, string(b)
}

func apiGetSlots() (int, string) {
	conn := CreateZkConn()
	defer conn.Close()
	slots, err := models.Slots(conn, productName)
	if err != nil {
		log.Warning("get slot info error, maybe init slots first? err:", err)
		return 500, err.Error()
	}
	b, err := json.MarshalIndent(slots, " ", "  ")
	return 200, string(b)
}

func apiSlotRangeSet(task RangeSetTask) (int, string) {
	conn := CreateZkConn()
	defer conn.Close()
	lock := utils.GetZkLock(conn, productName)
	lock.Lock(fmt.Sprintf("set slot range, %+v", task))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	err := models.SetSlotRange(conn, productName, task.FromSlot, task.ToSlot, task.NewGroupId, models.SLOT_STATUS_ONLINE)

	if err != nil {
		log.Warning(err)
		return 500, err.Error()
	}

	return jsonRetSucc()
}
