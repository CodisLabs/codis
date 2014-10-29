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

var (
	conf    *Conf
	s       *Server
	once    sync.Once
	conn    zkhelper.Conn
	suicide = false
)

func InitEnv() {
	once.Do(func() {
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

		s = NewServer(":1900", ":11000",
			conf,
		)

		s.OnSuicide = func() error {
			suicide = true
			return nil
		}
	})
}

func TestMarkOffline(t *testing.T) {
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

	time.Sleep(3 * time.Second)
	err = models.SetProxyStatus(conn, conf.productName, conf.proxyId, models.PROXY_STATE_MARK_OFFLINE)
	if err != nil {
		t.Fatal(errors.ErrorStack(err))
	}

	time.Sleep(3 * time.Second)

	if !suicide {
		t.Error("shoud be suicided")
	}
}
