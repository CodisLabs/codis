// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"testing"

	"github.com/ngaut/zkhelper"
)

func TestProxy(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	path := GetSlotBasePath(productName)
	children, _, _ := fakeZkConn.Children(path)
	if len(children) != 0 {
		t.Error("slot is no empty")
	}

	g := NewServerGroup(productName, 1)
	g.Create(fakeZkConn)

	// test create new group
	_, err := ServerGroups(fakeZkConn, productName)
	if err != nil {
		t.Error(err)
	}

	ok, err := g.Exists(fakeZkConn)
	if !ok || err != nil {
		t.Error("create group error")
	}

	s1 := NewServer(SERVER_TYPE_MASTER, "localhost:1111")

	g.AddServer(fakeZkConn, s1)

	err = InitSlotSet(fakeZkConn, productName, 1024)
	if err != nil {
		t.Error(err)
	}

	children, _, _ = fakeZkConn.Children(path)
	if len(children) != 1024 {
		t.Error("init slots error")
	}

	s, err := GetSlot(fakeZkConn, productName, 1)
	if err != nil {
		t.Error(err)
	}

	if s.GroupId != -1 {
		t.Error("init slots error")
	}

	err = SetSlotRange(fakeZkConn, productName, 0, 1023, 1, SLOT_STATUS_ONLINE)
	if err != nil {
		t.Error(err)
	}

	pi := &ProxyInfo{
		Id:    "proxy_1",
		Addr:  "localhost:1234",
		State: PROXY_STATE_OFFLINE,
	}

	_, err = CreateProxyInfo(fakeZkConn, productName, pi)
	if err != nil {
		t.Error(err)
	}

	ps, err := ProxyList(fakeZkConn, productName, nil)
	if err != nil {
		t.Error(err)
	}

	if len(ps) != 1 || ps[0].Id != "proxy_1" {
		t.Error("create proxy error")
	}

	err = SetProxyStatus(fakeZkConn, productName, pi.Id, PROXY_STATE_ONLINE)
	if err != nil {
		t.Error(err)
	}

	p, err := GetProxyInfo(fakeZkConn, productName, pi.Id)
	if err != nil {
		t.Error(err)
	}

	if p.State != PROXY_STATE_ONLINE {
		t.Error("change status error")
	}
}
