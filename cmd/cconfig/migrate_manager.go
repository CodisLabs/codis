// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"time"

	"github.com/wandoulabs/go-zookeeper/zk"
	"github.com/wandoulabs/zkhelper"

	"github.com/wandoulabs/codis/pkg/utils/log"
)

const (
	MAX_LOCK_TIMEOUT = 10 * time.Second
)

const (
	MIGRATE_TASK_PENDING   string = "pending"
	MIGRATE_TASK_MIGRATING string = "migrating"
	MIGRATE_TASK_FINISHED  string = "finished"
	MIGRATE_TASK_ERR       string = "error"
)

// check if migrate task is valid
type MigrateTaskCheckFunc func(t *MigrateTask) (bool, error)

// migrate task will store on zk
type MigrateManager struct {
	runningTask *MigrateTask
	zkConn      zkhelper.Conn
	productName string
}

func getMigrateTasksPath(product string) string {
	return fmt.Sprintf("/zk/codis/db_%s/migrate_tasks", product)
}

func NewMigrateManager(zkConn zkhelper.Conn, pn string) *MigrateManager {
	m := &MigrateManager{
		zkConn:      zkConn,
		productName: pn,
	}
	zkhelper.CreateRecursive(m.zkConn, getMigrateTasksPath(m.productName), "", 0, zkhelper.DefaultDirACLs())
	m.mayRecover()
	go m.loop()
	return m
}

// if there are tasks that is not pending, process them.
func (m *MigrateManager) mayRecover() error {
	// It may be not need to do anything now.
	return nil
}

//add a new task to zk
func (m *MigrateManager) PostTask(info *MigrateTaskInfo) {
	b, _ := json.Marshal(info)
	p, _ := safeZkConn.Create(getMigrateTasksPath(m.productName)+"/", b, zk.FlagSequence, zkhelper.DefaultFileACLs())
	_, info.Id = path.Split(p)
}

func (m *MigrateManager) loop() error {
	for {
		time.Sleep(time.Second)
		info := m.NextTask()
		if info == nil {
			continue
		}
		t := GetMigrateTask(*info)
		err := t.preMigrateCheck()
		if err != nil {
			log.ErrorErrorf(err, "pre migrate check failed")
		}
		err = t.run()
		if err != nil {
			log.ErrorErrorf(err, "migrate failed")
		}
	}
}

func (m *MigrateManager) NextTask() *MigrateTaskInfo {
	ts := m.Tasks()
	if len(ts) == 0 {
		return nil
	}
	return &ts[0]
}

func (m *MigrateManager) Tasks() []MigrateTaskInfo {
	res := Tasks{}
	tasks, _, _ := safeZkConn.Children(getMigrateTasksPath(m.productName))
	for _, id := range tasks {
		data, _, _ := safeZkConn.Get(getMigrateTasksPath(m.productName) + "/" + id)
		info := new(MigrateTaskInfo)
		json.Unmarshal(data, info)
		info.Id = id
		res = append(res, *info)
	}
	sort.Sort(res)
	return res
}

type Tasks []MigrateTaskInfo

func (t Tasks) Len() int {
	return len(t)
}

func (t Tasks) Less(i, j int) bool {
	return t[i].Id <= t[j].Id
}

func (t Tasks) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}
