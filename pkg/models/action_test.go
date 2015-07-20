// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"encoding/json"
	"fmt"
	"path"
	"testing"
	"time"

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

	//test if response node exists
	d, _, err := fakeZkConn.Get(prefix + "/0000000001")
	if err != nil {
		t.Error(errors.ErrorStack(err))
	}

	//test get action data
	d, _, err = fakeZkConn.Get(GetActionResponsePath(productName) + "/0000000001")
	if err != nil {
		t.Error(errors.ErrorStack(err))
	}

	var action Action
	json.Unmarshal(d, &action)
	if action.Desc != "desc" || action.Type != ACTION_TYPE_SLOT_CHANGED {
		t.Error("create action error")
	}
}

func TestWaitForReceiverTimeout(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	proxies := []ProxyInfo{}
	for i := 0; i < 5; i++ {
		proxies = append(proxies, ProxyInfo{
			Id:    fmt.Sprintf("proxy_%d", i),
			Addr:  fmt.Sprintf("localhost:%d", i+1234),
			State: PROXY_STATE_ONLINE,
		})
		CreateProxyInfo(fakeZkConn, productName, &proxies[i])
	}
	zkhelper.CreateRecursive(fakeZkConn, GetActionResponsePath(productName)+"/1", "", 0, zkhelper.DefaultDirACLs())
	go func() {
		time.Sleep(time.Second * 2)
		doResponseForTest(fakeZkConn, "1", &proxies[0])
		doResponseForTest(fakeZkConn, "1", &proxies[2])
		doResponseForTest(fakeZkConn, "1", &proxies[4])
		for {
			for i := 0; i < 5; i++ {
				pname := fmt.Sprintf("proxy_%d", i)
				p, _ := GetProxyInfo(fakeZkConn, productName, pname)
				if p != nil && p.State == PROXY_STATE_MARK_OFFLINE {
					zkhelper.DeleteRecursive(fakeZkConn, path.Join(GetProxyPath(productName), pname), -1)
				}
			}
		}
	}()
	err := WaitForReceiver(fakeZkConn, productName, GetActionResponsePath(productName)+"/1", proxies)
	if err != ErrReceiverTimeout {
		t.Error("there is no timeout as expected")
	}
	p, _ := GetProxyInfo(fakeZkConn, productName, "proxy_0")
	if p == nil || p.State != PROXY_STATE_ONLINE {
		t.Error("proxy_0 status is not as expected")
	}
	p, _ = GetProxyInfo(fakeZkConn, productName, "proxy_1")
	if p != nil {
		t.Error("proxy_1 status is not as expected")
	}
	p, _ = GetProxyInfo(fakeZkConn, productName, "proxy_2")
	if p == nil || p.State != PROXY_STATE_ONLINE {
		t.Error("proxy_2 status is not as expected")
	}
	p, _ = GetProxyInfo(fakeZkConn, productName, "proxy_3")
	if p != nil {
		t.Error("proxy_3 status is not as expected")
	}
	p, _ = GetProxyInfo(fakeZkConn, productName, "proxy_4")
	if p == nil || p.State != PROXY_STATE_ONLINE {
		t.Error("proxy_4 status is not as expected")
	}
}

func TestWaitForReceiver(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	proxies := []ProxyInfo{}
	for i := 0; i < 5; i++ {
		proxies = append(proxies, ProxyInfo{
			Id:    fmt.Sprintf("proxy_%d", i),
			Addr:  fmt.Sprintf("localhost:%d", i+1234),
			State: PROXY_STATE_ONLINE,
		})
		CreateProxyInfo(fakeZkConn, productName, &proxies[i])
	}
	zkhelper.CreateRecursive(fakeZkConn, GetActionResponsePath(productName)+"/1", "", 0, zkhelper.DefaultDirACLs())
	go func() {
		time.Sleep(time.Second * 2)
		doResponseForTest(fakeZkConn, "1", &proxies[0])
		doResponseForTest(fakeZkConn, "1", &proxies[1])
		doResponseForTest(fakeZkConn, "1", &proxies[2])
		doResponseForTest(fakeZkConn, "1", &proxies[3])
		doResponseForTest(fakeZkConn, "1", &proxies[4])
		for {
			for i := 0; i < 5; i++ {
				pname := fmt.Sprintf("proxy_%d", i)
				p, _ := GetProxyInfo(fakeZkConn, productName, pname)
				if p != nil && p.State == PROXY_STATE_MARK_OFFLINE {
					zkhelper.DeleteRecursive(fakeZkConn, path.Join(GetProxyPath(productName), pname), -1)
				}
			}
		}
	}()
	err := WaitForReceiver(fakeZkConn, productName, GetActionResponsePath(productName)+"/1", proxies)
	if err != nil {
		t.Error("there is error not as expected")
	}
	p, _ := GetProxyInfo(fakeZkConn, productName, "proxy_0")
	if p == nil || p.State != PROXY_STATE_ONLINE {
		t.Error("proxy_0 status is not as expected")
	}
	p, _ = GetProxyInfo(fakeZkConn, productName, "proxy_1")
	if p == nil || p.State != PROXY_STATE_ONLINE {
		t.Error("proxy_1 status is not as expected")
	}
	p, _ = GetProxyInfo(fakeZkConn, productName, "proxy_2")
	if p == nil || p.State != PROXY_STATE_ONLINE {
		t.Error("proxy_2 status is not as expected")
	}
	p, _ = GetProxyInfo(fakeZkConn, productName, "proxy_3")
	if p == nil || p.State != PROXY_STATE_ONLINE {
		t.Error("proxy_3 status is not as expected")
	}
	p, _ = GetProxyInfo(fakeZkConn, productName, "proxy_4")
	if p == nil || p.State != PROXY_STATE_ONLINE {
		t.Error("proxy_4 status is not as expected")
	}
}

func doResponseForTest(conn zkhelper.Conn, seq string, pi *ProxyInfo) error {
	actionPath := GetActionResponsePath(productName) + "/" + seq
	data, err := json.Marshal(pi)
	if err != nil {
		return errors.Trace(err)
	}

	_, err = conn.Create(path.Join(actionPath, pi.Id), data,
		0, zkhelper.DefaultFileACLs())
	return err
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
