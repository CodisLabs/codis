// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/juju/errors"
	"github.com/ngaut/zkhelper"

	"github.com/wandoulabs/codis/pkg/utils"
)

var (
	productName = "unit_test"
)

func TestNewAction(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	err := NewAction(fakeZkConn, productName, ACTION_TYPE_SLOT_CHANGED, nil, "desc", false)
	if err != nil {
		t.Error(errors.ErrorStack(err))
	}
	prefix := GetWatchActionPath(productName)
	if exist, _, err := fakeZkConn.Exists(prefix); !exist {
		t.Error(errors.ErrorStack(err))
	}

	d, _, err := fakeZkConn.Get(prefix + "/action_0000000001")
	if err != nil {
		t.Error(errors.ErrorStack(err))
	}

	var action Action
	json.Unmarshal(d, &action)
	if action.Desc != "desc" || action.Type != ACTION_TYPE_SLOT_CHANGED {
		t.Error("create action error")
	}
}

func TestForceRemoveLock(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	zkLock := utils.GetZkLock(fakeZkConn, productName)
	if zkLock == nil {
		t.Error("create lock error")
	}

	zkLock.Lock("force remove lock")
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
