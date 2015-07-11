// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"bufio"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/wandoulabs/codis/pkg/utils/assert"
	"github.com/wandoulabs/zkhelper"
)

var (
	once sync.Once
	conn zkhelper.Conn
)

func runFakeRedisSrv(addr string) {
	l, err := net.Listen("tcp", addr)
	assert.MustNoError(err)
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
	err := g.AddServer(conn, s1, "")
	assert.MustNoError(err)
	assert.Must(g.Servers[0].Type == SERVER_TYPE_MASTER)
}

func TestServerGroup(t *testing.T) {
	resetEnv()

	g := NewServerGroup(productName, 1)
	g.Create(conn)

	// test create new group
	groups, err := ServerGroups(conn, productName)
	assert.MustNoError(err)
	assert.Must(len(groups) != 0)

	ok, err := g.Exists(conn)
	assert.MustNoError(err)
	assert.Must(ok)

	gg, err := GetGroup(conn, productName, 1)
	assert.MustNoError(err)
	assert.Must(gg != nil && gg.Id == g.Id)

	s1 := NewServer(SERVER_TYPE_MASTER, "127.0.0.1:1111")
	s2 := NewServer(SERVER_TYPE_MASTER, "127.0.0.1:2222")

	err = g.AddServer(conn, s1, "")

	servers, err := g.GetServers(conn)
	assert.MustNoError(err)
	assert.Must(len(servers) == 1)

	g.AddServer(conn, s2, "")
	assert.Must(len(g.Servers) == 1)

	s2.Type = SERVER_TYPE_SLAVE
	g.AddServer(conn, s2, "")
	assert.Must(len(g.Servers) == 2)

	err = g.Promote(conn, s2.Addr, "")
	assert.MustNoError(err)

	s, err := g.Master(conn)
	assert.MustNoError(err)
	assert.Must(s.Addr == s2.Addr)
}
