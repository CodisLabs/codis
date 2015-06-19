// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/assert"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/zkhelper"
)

var (
	productName = "unit_test"
)

func waitForProxyMarkOffline(zkConn zkhelper.Conn, proxyName string) {
	_, _, c, _ := zkConn.GetW(path.Join(GetProxyPath(productName), proxyName))
	<-c
	info, _ := GetProxyInfo(zkConn, productName, proxyName)
	if info.State == PROXY_STATE_MARK_OFFLINE {
		SetProxyStatus(zkConn, productName, proxyName, PROXY_STATE_OFFLINE)
	}
}

func TestProxyOfflineInWaitActionReceiver(t *testing.T) {
	log.Infof("test proxy offline when waiting action response")
	fakeZkConn := zkhelper.NewConn()

	for i := 1; i <= 4; i++ {
		CreateProxyInfo(fakeZkConn, productName, &ProxyInfo{
			Id:    strconv.Itoa(i),
			State: PROXY_STATE_ONLINE,
		})
		go waitForProxyMarkOffline(fakeZkConn, strconv.Itoa(i))
	}

	lst, _ := ProxyList(fakeZkConn, productName, nil)
	assert.Must(len(lst) == 4)

	go func() {
		time.Sleep(500 * time.Millisecond)
		actionPath := path.Join(GetActionResponsePath(productName), fakeZkConn.Seq2Str(1))
		//create test response for proxy 4, means proxy 1,2,3 are timeout
		fakeZkConn.Create(path.Join(actionPath, "4"), nil,
			0, zkhelper.DefaultFileACLs())
	}()

	err := NewActionWithTimeout(fakeZkConn, productName, ACTION_TYPE_SLOT_CHANGED, nil, "desc", true, 3*1000)
	if err != nil {
		assert.Must(err.Error() == ErrReceiverTimeout.Error())
	}

	for i := 1; i <= 3; i++ {
		info, _ := GetProxyInfo(fakeZkConn, productName, strconv.Itoa(i))
		assert.Must(info.State == PROXY_STATE_OFFLINE)
	}
}

func TestNewAction(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	err := NewAction(fakeZkConn, productName, ACTION_TYPE_SLOT_CHANGED, nil, "desc", false)
	assert.MustNoError(err)

	prefix := GetWatchActionPath(productName)
	exist, _, err := fakeZkConn.Exists(prefix)
	assert.MustNoError(err)
	assert.Must(exist)

	//test if response node exists
	d, _, err := fakeZkConn.Get(prefix + "/0000000001")
	assert.MustNoError(err)

	//test get action data
	d, _, err = fakeZkConn.Get(GetActionResponsePath(productName) + "/0000000001")
	assert.MustNoError(err)

	var action Action
	err = json.Unmarshal(d, &action)
	assert.MustNoError(err)
	assert.Must(action.Desc == "desc")
	assert.Must(action.Type == ACTION_TYPE_SLOT_CHANGED)
}

func TestForceRemoveLock(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	zkLock := utils.GetZkLock(fakeZkConn, productName)
	assert.Must(zkLock != nil)

	zkLock.Lock("force remove lock")
	zkPath := fmt.Sprintf("/zk/codis/db_%s/LOCK", productName)
	children, _, err := fakeZkConn.Children(zkPath)
	assert.MustNoError(err)
	assert.Must(len(children) != 0)

	ForceRemoveLock(fakeZkConn, productName)
	children, _, err = fakeZkConn.Children(zkPath)
	assert.MustNoError(err)
	assert.Must(len(children) == 0)
}
