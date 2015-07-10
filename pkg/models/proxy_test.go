// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"testing"

	"github.com/wandoulabs/codis/pkg/utils/assert"
	"github.com/wandoulabs/zkhelper"
)

func TestProxy(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	path := GetSlotBasePath(productName)
	children, _, _ := fakeZkConn.Children(path)
	assert.Must(len(children) == 0)

	g := NewServerGroup(productName, 1)
	g.Create(fakeZkConn)

	// test create new group
	_, err := ServerGroups(fakeZkConn, productName)
	assert.MustNoError(err)

	ok, err := g.Exists(fakeZkConn)
	assert.MustNoError(err)
	assert.Must(ok)

	s1 := NewServer(SERVER_TYPE_MASTER, "localhost:1111")

	g.AddServer(fakeZkConn, s1, "")

	err = InitSlotSet(fakeZkConn, productName, 1024)
	assert.MustNoError(err)

	children, _, _ = fakeZkConn.Children(path)
	assert.Must(len(children) == 1024)

	s, err := GetSlot(fakeZkConn, productName, 1)
	assert.MustNoError(err)
	assert.Must(s.GroupId == -1)

	err = SetSlotRange(fakeZkConn, productName, 0, 1023, 1, SLOT_STATUS_ONLINE)
	assert.MustNoError(err)

	pi := &ProxyInfo{
		Id:    "proxy_1",
		Addr:  "localhost:1234",
		State: PROXY_STATE_OFFLINE,
	}

	_, err = CreateProxyInfo(fakeZkConn, productName, pi)
	assert.MustNoError(err)

	ps, err := ProxyList(fakeZkConn, productName, nil)
	assert.MustNoError(err)

	assert.Must(len(ps) == 1)
	assert.Must(ps[0].Id == "proxy_1")

	err = SetProxyStatus(fakeZkConn, productName, pi.Id, PROXY_STATE_ONLINE)
	assert.MustNoError(err)

	p, err := GetProxyInfo(fakeZkConn, productName, pi.Id)
	assert.MustNoError(err)
	assert.Must(p.State == PROXY_STATE_ONLINE)
}
