package main

import (
	"encoding/json"
	"fmt"
	"github.com/ngaut/go-zookeeper/zk"
	"github.com/ngaut/zkhelper"
	"path"
	"time"
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
	// TODO

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
			Fatal(err)
		}
		err = t.run()
		if err != nil {
			Fatal(err)
		}
	}
}

func (m *MigrateManager) NextTask() *MigrateTaskInfo {
	tasks, _, _ := safeZkConn.Children(getMigrateTasksPath(m.productName))
	if len(tasks) == 0 {
		return nil
	}
	data, _, _ := safeZkConn.Get(getMigrateTasksPath(m.productName) + "/" + tasks[0])
	info := new(MigrateTaskInfo)
	json.Unmarshal(data, info)
	return info
}

func (m *MigrateManager) Tasks() (res []MigrateTaskInfo) {
	tasks, _, _ := safeZkConn.Children(getMigrateTasksPath(m.productName))
	for _, id := range tasks {
		data, _, _ := safeZkConn.Get(getMigrateTasksPath(m.productName) + "/" + id)
		info := new(MigrateTaskInfo)
		json.Unmarshal(data, info)
		res = append(res, *info)
	}
	return
}
