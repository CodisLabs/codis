// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/go-zookeeper/zk"
	"github.com/ngaut/zkhelper"
	"github.com/wandoulabs/codis/pkg/models"

	"sync/atomic"

	stdlog "log"

	"github.com/codegangsta/martini-contrib/binding"
	"github.com/codegangsta/martini-contrib/render"
	"github.com/docopt/docopt-go"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/cors"

	log "github.com/ngaut/logging"
)

func cmdDashboard(argv []string) (err error) {
	usage := `usage: codis-config dashboard [--addr=<address>] [--http-log=<log_file>]

options:
	--addr	listen ip:port, e.g. localhost:12345, :8086, [default: :8086]
	--http-log	http request log [default: request.log ]
`

	args, err := docopt.Parse(usage, argv, true, "", false)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debug(args)

	logFileName := "request.log"
	if args["--http-log"] != nil {
		logFileName = args["--http-log"].(string)
	}

	addr := ":8086"
	if args["--addr"] != nil {
		addr = args["--addr"].(string)
	}

	runDashboard(addr, logFileName)
	return nil
}

var proxiesSpeed int64

func CreateZkConn() zkhelper.Conn {
	conn, err := globalEnv.NewZkConn()
	if err != nil {
		Fatal("Failed to create zk connection: " + err.Error())
	}
	return conn
}

func jsonRet(output map[string]interface{}) (int, string) {
	b, err := json.Marshal(output)
	if err != nil {
		log.Warning(err)
	}
	return 200, string(b)
}

func jsonRetFail(errCode int, msg string) (int, string) {
	return jsonRet(map[string]interface{}{
		"ret": errCode,
		"msg": msg,
	})
}

func jsonRetSucc() (int, string) {
	return jsonRet(map[string]interface{}{
		"ret": 0,
		"msg": "OK",
	})
}

func getAllProxyOps() int64 {
	conn := CreateZkConn()
	defer conn.Close()
	proxies, err := models.ProxyList(conn, globalEnv.ProductName(), nil)
	if err != nil {
		log.Warning(err)
		return -1
	}

	var total int64
	for _, p := range proxies {
		i, err := p.Ops()
		if err != nil {
			log.Warning(err)
		}
		total += i
	}
	return total
}

// for debug
func getAllProxyDebugVars() map[string]map[string]interface{} {
	conn := CreateZkConn()
	defer conn.Close()
	proxies, err := models.ProxyList(conn, globalEnv.ProductName(), nil)
	if err != nil {
		log.Warning(err)
		return nil
	}

	ret := make(map[string]map[string]interface{})
	for _, p := range proxies {
		m, err := p.DebugVars()
		if err != nil {
			log.Warning(err)
		}
		ret[p.Id] = m
	}
	return ret
}

func getProxySpeedChan() <-chan int64 {
	c := make(chan int64)
	go func() {
		var lastCnt int64
		for {
			cnt := getAllProxyOps()
			if lastCnt > 0 {
				c <- cnt - lastCnt
			}
			lastCnt = cnt
			time.Sleep(1 * time.Second)
		}
	}()
	return c
}

func pageSlots(r render.Render) {
	r.HTML(200, "slots", nil)
}

func createDashboardNode() error {
	conn := CreateZkConn()
	defer conn.Close()

	// make sure root dir is exists
	rootDir := fmt.Sprintf("/zk/codis/db_%s", globalEnv.ProductName())
	zkhelper.CreateRecursive(conn, rootDir, "", 0, zkhelper.DefaultDirACLs())

	zkPath := fmt.Sprintf("%s/dashboard", rootDir)
	// make sure we're the only one dashboard
	if exists, _, _ := conn.Exists(zkPath); exists {
		data, _, _ := conn.Get(zkPath)
		return errors.New("dashboard already exists: " + string(data))
	}

	content := fmt.Sprintf(`{"addr": "%v", "pid": %v}`, globalEnv.DashboardAddr(), os.Getpid())
	pathCreated, err := conn.Create(zkPath, []byte(content),
		zk.FlagEphemeral, zkhelper.DefaultFileACLs())

	log.Info("dashboard node created:", pathCreated, string(content))

	return errors.Trace(err)
}

func releaseDashboardNode() {
	conn := CreateZkConn()
	defer conn.Close()

	zkPath := fmt.Sprintf("/zk/codis/db_%s/dashboard", globalEnv.ProductName())
	if exists, _, _ := conn.Exists(zkPath); exists {
		log.Info("removing dashboard node")
		conn.Delete(zkPath, 0)
	}
}

func runDashboard(addr string, httpLogFile string) {
	log.Info("dashboard listening on addr: ", addr)
	m := martini.Classic()
	f, err := os.OpenFile(httpLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		Fatal(err)
	}
	defer f.Close()

	m.Map(stdlog.New(f, "[martini]", stdlog.LstdFlags))
	binRoot, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		Fatal(err)
	}

	m.Use(martini.Static(filepath.Join(binRoot, "assets/statics")))
	m.Use(render.Renderer(render.Options{
		Directory:  filepath.Join(binRoot, "assets/template"),
		Extensions: []string{".tmpl", ".html"},
		Charset:    "UTF-8",
		IndentJSON: true,
	}))

	m.Use(cors.Allow(&cors.Options{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"POST", "GET", "DELETE", "PUT"},
		AllowHeaders:     []string{"Origin", "x-requested-with", "Content-Type", "Content-Range", "Content-Disposition", "Content-Description"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	m.Get("/api/server_groups", apiGetServerGroupList)
	m.Get("/api/overview", apiOverview)

	m.Get("/api/redis/:addr/stat", apiRedisStat)
	m.Get("/api/redis/:addr/:id/slotinfo", apiGetRedisSlotInfo)
	m.Get("/api/redis/group/:group_id/:slot_id/slotinfo", apiGetRedisSlotInfoFromGroupId)

	m.Put("/api/server_groups", binding.Json(models.ServerGroup{}), apiAddServerGroup)
	m.Put("/api/server_group/(?P<id>[0-9]+)/addServer", binding.Json(models.Server{}), apiAddServerToGroup)
	m.Delete("/api/server_group/(?P<id>[0-9]+)", apiRemoveServerGroup)

	m.Put("/api/server_group/(?P<id>[0-9]+)/removeServer", binding.Json(models.Server{}), apiRemoveServerFromGroup)
	m.Get("/api/server_group/(?P<id>[0-9]+)", apiGetServerGroup)
	m.Post("/api/server_group/(?P<id>[0-9]+)/promote", binding.Json(models.Server{}), apiPromoteServer)

	m.Get("/api/migrate/status", apiMigrateStatus)
	m.Get("/api/migrate/tasks", apiGetMigrateTasks)
	m.Delete("/api/migrate/pending_task/:id/remove", apiRemovePendingMigrateTask)
	m.Delete("/api/migrate/task/:id/stop", apiStopMigratingTask)
	m.Post("/api/migrate", binding.Json(MigrateTaskInfo{}), apiDoMigrate)

	m.Post("/api/rebalance", apiRebalance)
	m.Get("/api/rebalance/status", apiRebalanceStatus)

	m.Get("/api/slot/list", apiGetSlots)
	m.Get("/api/slot/:id", apiGetSingleSlot)
	m.Post("/api/slots/init", apiInitSlots)
	m.Get("/api/slots", apiGetSlots)
	m.Post("/api/slot", binding.Json(RangeSetTask{}), apiSlotRangeSet)
	m.Get("/api/proxy/list", apiGetProxyList)
	m.Get("/api/proxy/debug/vars", apiGetProxyDebugVars)
	m.Post("/api/proxy", binding.Json(models.ProxyInfo{}), apiSetProxyStatus)

	m.Get("/api/action/gc", apiActionGC)
	m.Get("/api/force_remove_locks", apiForceRemoveLocks)
	m.Get("/api/remove_fence", apiRemoveFence)

	m.Get("/slots", pageSlots)
	m.Get("/", func(r render.Render) {
		r.Redirect("/admin")
	})

	// create temp node in ZK
	if err := createDashboardNode(); err != nil {
		Fatal(err)
	}
	defer releaseDashboardNode()

	// create long live migrate manager
	conn := CreateZkConn()
	defer conn.Close()
	globalMigrateManager = NewMigrateManager(conn, globalEnv.ProductName(), preMigrateCheck)
	defer globalMigrateManager.removeNode()

	go func() {
		c := getProxySpeedChan()
		for {
			atomic.StoreInt64(&proxiesSpeed, <-c)
		}
	}()

	m.RunOnAddr(addr)
}
