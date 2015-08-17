// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	stdlog "log"

	"github.com/codegangsta/martini-contrib/binding"
	"github.com/codegangsta/martini-contrib/render"
	"github.com/docopt/docopt-go"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/cors"
	"github.com/wandoulabs/zkhelper"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func cmdDashboard(argv []string) (err error) {
	usage := `usage: codis-config dashboard [--addr=<address>] [--http-log=<log_file>]

options:
	--addr	listen ip:port, e.g. localhost:18087, :18087, [default: :18087]
	--http-log	http request log [default: request.log ]
`

	args, err := docopt.Parse(usage, argv, true, "", false)
	if err != nil {
		log.ErrorErrorf(err, "parse args failed")
		return err
	}
	log.Debugf("parse args = {%+v}", args)

	logFileName := "request.log"
	if args["--http-log"] != nil {
		logFileName = args["--http-log"].(string)
	}

	addr := ":18087"
	if args["--addr"] != nil {
		addr = args["--addr"].(string)
	}

	runDashboard(addr, logFileName)
	return nil
}

var (
	proxiesSpeed int64
	safeZkConn   zkhelper.Conn
	unsafeZkConn zkhelper.Conn
)

func jsonRet(output map[string]interface{}) (int, string) {
	b, err := json.Marshal(output)
	if err != nil {
		log.WarnErrorf(err, "to json failed")
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
	proxies, err := models.ProxyList(unsafeZkConn, globalEnv.ProductName(), nil)
	if err != nil {
		log.ErrorErrorf(err, "get proxy list failed")
		return -1
	}

	var total int64
	for _, p := range proxies {
		i, err := p.Ops()
		if err != nil {
			log.WarnErrorf(err, "get proxy ops failed")
		}
		total += i
	}
	return total
}

// for debug
func getAllProxyDebugVars() map[string]map[string]interface{} {
	proxies, err := models.ProxyList(unsafeZkConn, globalEnv.ProductName(), nil)
	if err != nil {
		log.ErrorErrorf(err, "get proxy list failed")
		return nil
	}

	ret := make(map[string]map[string]interface{})
	for _, p := range proxies {
		m, err := p.DebugVars()
		if err != nil {
			log.WarnErrorf(err, "get proxy debug varsfailed")
		}
		ret[p.Id] = m
	}
	return ret
}

func pageSlots(r render.Render) {
	r.HTML(200, "slots", nil)
}

func createDashboardNode() error {

	// make sure root dir is exists
	rootDir := fmt.Sprintf("/zk/codis/db_%s", globalEnv.ProductName())
	zkhelper.CreateRecursive(safeZkConn, rootDir, "", 0, zkhelper.DefaultDirACLs())

	zkPath := fmt.Sprintf("%s/dashboard", rootDir)
	// make sure we're the only one dashboard
	if exists, _, _ := safeZkConn.Exists(zkPath); exists {
		data, _, _ := safeZkConn.Get(zkPath)
		return errors.New("dashboard already exists: " + string(data))
	}

	content := fmt.Sprintf(`{"addr": "%v", "pid": %v}`, globalEnv.DashboardAddr(), os.Getpid())
	pathCreated, err := safeZkConn.Create(zkPath, []byte(content), 0, zkhelper.DefaultFileACLs())
	createdDashboardNode = true
	log.Infof("dashboard node created: %v, %s", pathCreated, string(content))
	log.Warn("********** Attention **********")
	log.Warn("You should use `kill {pid}` rather than `kill -9 {pid}` to stop me,")
	log.Warn("or the node resisted on zk will not be cleaned when I'm quiting and you must remove it manually")
	log.Warn("*******************************")
	return errors.Trace(err)
}

func releaseDashboardNode() {
	zkPath := fmt.Sprintf("/zk/codis/db_%s/dashboard", globalEnv.ProductName())
	if exists, _, _ := safeZkConn.Exists(zkPath); exists {
		log.Infof("removing dashboard node")
		safeZkConn.Delete(zkPath, 0)
	}
}

func runDashboard(addr string, httpLogFile string) {
	log.Infof("dashboard listening on addr: %s", addr)
	m := martini.Classic()
	f, err := os.OpenFile(httpLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.PanicErrorf(err, "open http log file failed")
	}
	defer f.Close()

	m.Map(stdlog.New(f, "[martini]", stdlog.LstdFlags))
	binRoot, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.PanicErrorf(err, "get binroot path failed")
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
	m.Post("/api/migrate", binding.Json(migrateTaskForm{}), apiDoMigrate)

	m.Post("/api/rebalance", apiRebalance)

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
	zkBuilder := utils.NewConnBuilder(globalEnv.NewZkConn)
	safeZkConn = zkBuilder.GetSafeConn()
	unsafeZkConn = zkBuilder.GetUnsafeConn()

	// create temp node in ZK
	if err := createDashboardNode(); err != nil {
		log.PanicErrorf(err, "create zk node failed") // do not release dashborad node here
	}

	// create long live migrate manager
	globalMigrateManager = NewMigrateManager(safeZkConn, globalEnv.ProductName())

	go func() {
		tick := time.Tick(time.Second)
		var lastCnt, qps int64
		for _ = range tick {
			cnt := getAllProxyOps()
			if cnt > 0 {
				qps = cnt - lastCnt
				lastCnt = cnt
			} else {
				qps = 0
			}
			atomic.StoreInt64(&proxiesSpeed, qps)
		}
	}()

	m.RunOnAddr(addr)
}
