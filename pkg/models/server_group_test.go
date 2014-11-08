// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"testing"

	"github.com/ngaut/zkhelper"
)

func TestServerGroup(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	g := NewServerGroup(productName, 1)
	g.Create(fakeZkConn)

	// test create new group
	groups, err := ServerGroups(fakeZkConn, productName)
	if err != nil {
		t.Error(err)
	}

	if len(groups) == 0 {
		t.Error("create group error")
	}

	ok, err := g.Exists(fakeZkConn)
	if !ok || err != nil {
		t.Error("create group error")
	}

	gg, err := GetGroup(fakeZkConn, productName, 1)
	if err != nil {
		t.Error(err)
	}
	if gg == nil || gg.Id != g.Id {
		t.Error("get group error")
	}

	s1 := NewServer(SERVER_TYPE_MASTER, "localhost:1111")
	s2 := NewServer(SERVER_TYPE_MASTER, "localhost:2222")

	g.AddServer(fakeZkConn, s1)

	servers, err := g.GetServers(fakeZkConn)
	if err != nil {
		t.Error("add server error")
	}
	if len(servers) != 1 {
		t.Error("add server error", len(servers))
	}

	g.AddServer(fakeZkConn, s1)

	if len(g.Servers) != 1 {
		t.Error("create group error")
	}

	g.AddServer(fakeZkConn, s2)

	// add another master
	if len(g.Servers) != 1 {
		t.Error("add server error")
	}

	s2.Type = SERVER_TYPE_SLAVE
	g.AddServer(fakeZkConn, s2)
	if len(g.Servers) != 2 {
		t.Error("add server error")
	}

	g.Promote(fakeZkConn, s2.Addr)
	s, err := g.Master(fakeZkConn)
	if err != nil {
		t.Error(err)
	}
	// already exist master
	if s.Addr != s1.Addr {
		t.Error("prompt error")
	}

	s, err = g.Master(fakeZkConn)
	if err != nil {
		t.Error(err)
	}
	// already exist master
	if s.Addr != s1.Addr {
		t.Error("master error")
	}
}
