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
	"github.com/wandoulabs/codis/pkg/utils/errors"
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
	err := WaitForReceiverWithTimeout(fakeZkConn, productName, GetActionResponsePath(productName)+"/1", proxies, 1000*10)
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
	err := WaitForReceiverWithTimeout(fakeZkConn, productName, GetActionResponsePath(productName)+"/1", proxies, 1000*10)
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
