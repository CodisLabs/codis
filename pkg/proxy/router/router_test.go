// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/garyburd/redigo/redis"
	"github.com/juju/errors"
	log "github.com/ngaut/logging"
	"github.com/ngaut/zkhelper"
	"github.com/wandoulabs/codis/pkg/models"
)

var (
	conf       *Conf
	s          *Server
	once       sync.Once
	waitonce   sync.Once
	conn       zkhelper.Conn
	redis1     *miniredis.Miniredis
	redis2     *miniredis.Miniredis
	proxyMutex sync.Mutex
)

func InitEnv() {
	go once.Do(func() {
		conn = zkhelper.NewConn()
		conf = &Conf{
			proxyId:     "proxy_test",
			productName: "test",
			zkAddr:      "localhost:2181",
			netTimeout:  5,
			f:           func(string) (zkhelper.Conn, error) { return conn, nil },
			proto:       "tcp4",
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

		//init  server group
		g1 := models.NewServerGroup(conf.productName, 1)
		g1.Create(conn)
		g2 := models.NewServerGroup(conf.productName, 2)
		g2.Create(conn)

		redis1, _ = miniredis.Run()
		redis2, _ = miniredis.Run()

		s1 := models.NewServer(models.SERVER_TYPE_MASTER, redis1.Addr())
		s2 := models.NewServer(models.SERVER_TYPE_MASTER, redis2.Addr())

		g1.AddServer(conn, s1)
		g2.AddServer(conn, s2)

		//set slot range
		err = models.SetSlotRange(conn, conf.productName, 0, 511, 1, models.SLOT_STATUS_ONLINE)
		if err != nil {
			log.Fatal(err)
		}

		err = models.SetSlotRange(conn, conf.productName, 512, 1023, 2, models.SLOT_STATUS_ONLINE)
		if err != nil {
			log.Fatal(err)
		}

		go func() { //set proxy online
			time.Sleep(3 * time.Second)
			err := models.SetProxyStatus(conn, conf.productName, conf.proxyId, models.PROXY_STATE_ONLINE)
			if err != nil {
				log.Fatal(errors.ErrorStack(err))
			}
			time.Sleep(2 * time.Second)
			proxyMutex.Lock()
			defer proxyMutex.Unlock()
			pi := s.getProxyInfo()
			if pi.State != models.PROXY_STATE_ONLINE {
				log.Fatalf("should be online, we got %s", pi.State)
			}
		}()

		proxyMutex.Lock()
		s = NewServer(":19000", ":11000",
			conf,
		)
		proxyMutex.Unlock()
		s.Run()
	})

	waitonce.Do(func() {
		time.Sleep(10 * time.Second)
	})
}

func TestSingleKeyRedisCmd(t *testing.T) {
	InitEnv()
	c, err := redis.Dial("tcp", "localhost:19000")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.Do("SET", "foo", "bar")
	if err != nil {
		t.Error(err)
	}

	if got, err := redis.String(c.Do("get", "foo")); err != nil || got != "bar" {
		t.Error("'foo' has the wrong value")
	}

	_, err = c.Do("SET", "bar", "foo")
	if err != nil {
		t.Error(err)
	}

	if got, err := redis.String(c.Do("get", "bar")); err != nil || got != "foo" {
		t.Error("'bar' has the wrong value")
	}
}

func TestMultiKeyRedisCmd(t *testing.T) {
	InitEnv()
	c, err := redis.Dial("tcp", "localhost:19000")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.Do("SET", "key1", "value1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Do("SET", "key2", "value2")
	if err != nil {
		t.Fatal(err)
	}

	var value1 string
	var value2 string
	var value3 string
	reply, err := redis.Values(c.Do("MGET", "key1", "key2", "key3"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := redis.Scan(reply, &value1, &value2, &value3); err != nil {
		t.Fatal(err)
	}

	if value1 != "value1" || value2 != "value2" || len(value3) != 0 {
		t.Error("value not match")
	}

	//test del
	if _, err := c.Do("del", "key1", "key2", "key3"); err != nil {
		t.Fatal(err)
	}

	//reset
	value1 = ""
	value2 = ""
	value3 = ""
	reply, err = redis.Values(c.Do("MGET", "key1", "key2", "key3"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := redis.Scan(reply, &value1, &value2, &value3); err != nil {
		t.Fatal(err)
	}

	if len(value1) != 0 || len(value2) != 0 || len(value3) != 0 {
		t.Error("value not match", value1, value2, value3)
	}

	//reset
	value1 = ""
	value2 = ""
	value3 = ""

	_, err = c.Do("MSET", "key1", "value1", "key2", "value2", "key3", "")
	if err != nil {
		t.Fatal(err)
	}

	reply, err = redis.Values(c.Do("MGET", "key1", "key2", "key3"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := redis.Scan(reply, &value1, &value2, &value3); err != nil {
		t.Fatal(err)
	}

	if value1 != "value1" || value2 != "value2" || len(value3) != 0 {
		t.Error("value not match", value1, value2, value3)
	}
}

func TestInvalidRedisCmdUnknown(t *testing.T) {
	InitEnv()
	c, err := redis.Dial("tcp", "localhost:19000")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if _, err := c.Do("unknown", "key1", "key2", "key3"); err == nil {
		t.Fatal(err)
	}
}

func TestInvalidRedisCmdPing(t *testing.T) {
	InitEnv()
	c, err := redis.Dial("tcp", "localhost:19000")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.Do("SAVE")
	if err != io.EOF {
		t.Fatal(err)
	}
}

func TestInvalidRedisCmdQuit(t *testing.T) {
	InitEnv()
	c, err := redis.Dial("tcp", "localhost:19000")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.Do("quit")
	if err != nil {
		t.Fatal(err)
	}
}

func TestInvalidRedisCmdEcho(t *testing.T) {
	InitEnv()
	c, err := redis.Dial("tcp", "localhost:19000")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.Do("echo", "xx")
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Do("echo")
	if err != nil {
		t.Fatal(err)
	}

}

//this should be the last test
func TestMarkOffline(t *testing.T) {
	InitEnv()

	suicide := int64(0)
	proxyMutex.Lock()
	s.OnSuicide = func() error {
		atomic.StoreInt64(&suicide, 1)
		return nil
	}
	proxyMutex.Unlock()

	err := models.SetProxyStatus(conn, conf.productName, conf.proxyId, models.PROXY_STATE_MARK_OFFLINE)
	if err != nil {
		t.Fatal(errors.ErrorStack(err))
	}

	time.Sleep(3 * time.Second)

	if atomic.LoadInt64(&suicide) == 0 {
		t.Error("shoud be suicided")
	}
}

func TestRedisRestart(t *testing.T) {
	InitEnv()

	c, err := redis.Dial("tcp", "localhost:19000")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.Do("SET", "key1", "value1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Do("SET", "key2", "value2")
	if err != nil {
		t.Fatal(err)
	}

	//close redis
	redis1.Close()
	redis2.Close()
	_, err = c.Do("SET", "key1", "value1")
	if err == nil {
		t.Fatal("should be error")
	}
	_, err = c.Do("SET", "key2", "value2")
	if err == nil {
		t.Fatal("should be error")
	}

	//restart redis
	redis1.Restart()
	redis2.Restart()
	time.Sleep(3 * time.Second)
	//proxy should closed our connection
	_, err = c.Do("SET", "key1", "value1")
	if err == nil {
		t.Error("should be error")
	}

	//now, proxy should recovered from connection error
	c, err = redis.Dial("tcp", "localhost:19000")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.Do("SET", "key1", "value1")
	if err != nil {
		t.Fatal(err)
	}
}
