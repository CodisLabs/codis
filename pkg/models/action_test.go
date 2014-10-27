package models

import (
	"codis/pkg/utils"
	"codis/pkg/zkhelper"
	"encoding/json"
	"fmt"
	"testing"
)

var (
	productName = "unit_test"
)

func TestNewAction(t *testing.T) {
	fakeZkConn := zkhelper.NewFakeConn()
	err := NewAction(fakeZkConn, productName, ACTION_TYPE_SLOT_CHANGED, nil, "desc", false)

	if err != nil {
		t.Error(err)
	}
	prefix := GetWatchActionPath(productName)
	d, _, _ := fakeZkConn.Get(prefix + "/action_0000000001")
	var action Action
	json.Unmarshal(d, &action)
	if action.Desc != "desc" || action.Type != ACTION_TYPE_SLOT_CHANGED {
		t.Error("create action error")
	}
}

func TestForceRemoveLock(t *testing.T) {
	fakeZkConn := zkhelper.NewFakeConn()
	zkLock := utils.GetZkLock(fakeZkConn, productName)
	if zkLock == nil {
		t.Error("create lock error")
	}

	zkLock.Lock()
	zkPath := fmt.Sprintf("/zk/codis/db_%s/LOCK", productName)
	children, _, err := fakeZkConn.Children(zkPath)
	if err != nil {
		t.Error(err)
	}
	if len(children) == 0 {
		t.Error("create lock error")
	}
	ForceRemoveLock(fakeZkConn, productName)
	children, _, err = fakeZkConn.Children(zkPath)
	if err != nil {
		t.Error(err)
	}
	if len(children) != 0 {
		t.Error("remove lock error")
	}
}
