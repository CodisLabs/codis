package router

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/juju/errors"
	"github.com/wandoulabs/codis/pkg/models"

	"github.com/ngaut/zkhelper"
)

var conf *Conf
var s *Server
var once sync.Once
var conn zkhelper.Conn

func InitEnv() {
	conn = zkhelper.NewConn()
	conf = &Conf{
		proxyId:     "proxy_test",
		productName: "test",
		zkAddr:      "localhost:2181",
		f:           func(string) (zkhelper.Conn, error) { return conn, nil },
	}

	//init action path
	prefix := models.GetWatchActionPath(conf.productName)

	err := models.CreateActionRootPath(conn, prefix)
	if err != nil {
		log.Fatal(err)
	}

	//init slot
	err = models.InitSlotSet(conn, conf.productName, 1024)
	if err != nil {
		log.Fatal(err)
	}

	err = models.SetSlotRange(conn, conf.productName, 0, 1023, 1, models.SLOT_STATUS_ONLINE)
	if err != nil {
		log.Fatal(err)
	}

	//init  server group
	g := models.NewServerGroup(conf.productName, 1)
	g.Create(conn)

	s1 := models.NewServer(models.SERVER_TYPE_MASTER, "localhost:1111")
	s2 := models.NewServer(models.SERVER_TYPE_MASTER, "localhost:2222")

	g.AddServer(conn, s1)
	g.AddServer(conn, s2)

	once.Do(func() {
		s = NewServer(":1900", ":11000",
			conf,
		)
	})
}

func TestWaitOnline(t *testing.T) {
	go InitEnv()
	time.Sleep(2 * time.Second)

	err := models.SetProxyStatus(conn, conf.productName, conf.proxyId, models.PROXY_STATE_ONLINE)
	if err != nil {
		t.Fatal(errors.ErrorStack(err))
	}
	time.Sleep(2 * time.Second)
	if s.pi.State != models.PROXY_STATE_ONLINE {
		t.Errorf("should be online, we got %s", s.pi.State)
	}
}
