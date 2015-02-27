package main

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/ngaut/go-zookeeper/zk"
	log "github.com/ngaut/logging"
	"github.com/ngaut/zkhelper"
	"github.com/wandoulabs/codis/pkg/models"
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

type SlotMigrator interface {
	Migrate(slot *models.Slot, fromGroup, toGroup int, task *MigrateTask, onProgress func(SlotMigrateProgress)) error
}

// check if migrate task is valid
type MigrateTaskCheckFunc func(t *MigrateTask) (bool, error)

// migrate task will store on zk
type MigrateManager struct {
	// pre migrate check functions
	preCheck     MigrateTaskCheckFunc
	pendingTasks *list.List
	runningTask  *MigrateTask
	// zkConn
	zkConn      zkhelper.Conn
	productName string
	lck         sync.RWMutex
}

func (m *MigrateManager) createNode() error {
	zkhelper.CreateRecursive(m.zkConn, fmt.Sprintf("/zk/codis/db_%s/migrate_tasks", m.productName), "", 0, zkhelper.DefaultDirACLs())
	_, err := m.zkConn.Create(fmt.Sprintf("/zk/codis/db_%s/migrate_manager", m.productName),
		[]byte(""), zk.FlagEphemeral, zkhelper.DefaultDirACLs())
	if err != nil {
		log.Error("there is another dashboard exists! ERR:", err)
	}
	return nil
}

func (m *MigrateManager) removeNode() error {
	return zkhelper.DeleteRecursive(m.zkConn, fmt.Sprintf("/zk/codis/db_%s/migrate_manager", m.productName), 0)
}

func NewMigrateManager(zkConn zkhelper.Conn, pn string, preTaskCheck MigrateTaskCheckFunc) *MigrateManager {
	m := &MigrateManager{
		pendingTasks: list.New(),
		preCheck:     preTaskCheck,
		zkConn:       zkConn,
		productName:  pn,
	}
	err := m.createNode()
	if err != nil {
		Fatal("check if there's another codis-config running? shut it down and try again")
	}
	go m.loop()
	return m
}

func (m *MigrateManager) PostTask(t *MigrateTask) {
	m.lck.Lock()
	m.pendingTasks.PushBack(t)
	m.lck.Unlock()
}

func (m *MigrateManager) loop() error {
	for {
		m.lck.RLock()
		ele := m.pendingTasks.Front()
		m.lck.RUnlock()
		if ele == nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// get pending task, and run
		m.lck.Lock()
		m.pendingTasks.Remove(ele)
		m.lck.Unlock()

		t := ele.Value.(*MigrateTask)
		t.zkConn = m.zkConn
		t.productName = m.productName

		m.runningTask = t
		if m.preCheck != nil {
			log.Info("start migrate task pre-check")
			if ok, err := m.preCheck(t); !ok {
				if err != nil {
					log.Error(err)
				}
				log.Error("migrate task pre-check error", t)
				continue
			}
		}
		log.Info("migrate task pre-check done")
		// do migrate
		err := t.run()
		if err != nil {
			log.Error(err)
		}

		// reset runningtask
		m.lck.Lock()
		m.runningTask = nil
		m.lck.Unlock()
	}
}

func (m *MigrateManager) StopTask(taskId string) error {
	m.lck.Lock()
	defer m.lck.Unlock()
	return nil
}

func (m *MigrateManager) Tasks() ([]*MigrateTask, error) {
	return nil, nil
}

func (m *MigrateManager) GetTaskById(taskId string) (*MigrateTask, error) {
	return nil, nil
}
