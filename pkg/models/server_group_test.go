// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"bufio"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/juju/errors"

	"github.com/ngaut/zkhelper"
)

var (
	once sync.Once
	conn zkhelper.Conn
)

func runFakeRedisSrv(addr string) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}

		go func(c net.Conn) {
			w := bufio.NewWriter(c)
			w.WriteString("+OK\r\n")
			w.Flush()
		}(c)
	}
}

func resetEnv() {
	conn = zkhelper.NewConn()
	once.Do(func() {
		go runFakeRedisSrv("127.0.0.1:1111")
		go runFakeRedisSrv("127.0.0.1:2222")
		time.Sleep(1 * time.Second)
	})
}

func TestAddSlaveToEmptyGroup(t *testing.T) {
	resetEnv()
	g := NewServerGroup(productName, 1)
	g.Create(conn)

	s1 := NewServer(SERVER_TYPE_SLAVE, "127.0.0.1:1111")
	err := g.AddServer(conn, s1)
	if err != nil {
		t.Error(err)
	}

	if g.Servers[0].Type != SERVER_TYPE_MASTER {
		t.Error("add a slave to an empty group, this server should become master")
	}
}

func TestServerGroup(t *testing.T) {
	resetEnv()

	g := NewServerGroup(productName, 1)
	g.Create(conn)

	// test create new group
	groups, err := ServerGroups(conn, productName)
	if err != nil {
		t.Error(err)
		return
	}

	if len(groups) == 0 {
		t.Error("create group error")
		return
	}

	ok, err := g.Exists(conn)
	if !ok || err != nil {
		t.Error("create group error")
		return
	}

	gg, err := GetGroup(conn, productName, 1)
	if err != nil {
		t.Error(err)
		return
	}

	if gg == nil || gg.Id != g.Id {
		t.Error("get group error")
		return
	}

	s1 := NewServer(SERVER_TYPE_MASTER, "127.0.0.1:1111")
	s2 := NewServer(SERVER_TYPE_MASTER, "127.0.0.1:2222")

	err = g.AddServer(conn, s1)

	servers, err := g.GetServers(conn)
	if err != nil {
		t.Error("add server error")
		return
	}
	if len(servers) != 1 {
		t.Error("add server error", len(servers))
		return
	}

	g.AddServer(conn, s2)
	if len(g.Servers) != 1 {
		t.Error("add server error, cannot add 2 masters")
		return
	}

	s2.Type = SERVER_TYPE_SLAVE
	g.AddServer(conn, s2)
	if len(g.Servers) != 2 {
		t.Error("add slave server error")
		return
	}

	if err := g.Promote(conn, s2.Addr); err != nil {
		t.Error(errors.ErrorStack(err))
		return
	}

	s, err := g.Master(conn)
	if err != nil {
		t.Error(err)
		return
	}

	if s.Addr != s2.Addr {
		t.Error("promote error", s, s1)
		return
	}
}
