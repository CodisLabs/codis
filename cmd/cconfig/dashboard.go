package main

import (
	"encoding/json"
	"os"
	"time"

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
	usage := `usage: cconfig dashboard [--addr=<address>] [--http-log=<log_file>]

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

	if args["--addr"] != nil {
		runDashboard(args["--addr"].(string), logFileName)
	} else {
		runDashboard(":8086", logFileName)
	}
	return nil
}

var proxiesSpeed int64

func CreateZkConn() zkhelper.Conn {
	conn, _ := zkhelper.ConnectToZk(zkAddr)
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
	proxies, err := models.ProxyList(conn, productName, nil)
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
	proxies, err := models.ProxyList(conn, productName, nil)
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
	go func(c chan int64) {
		var lastCnt int64 = 0
		for {
			cnt := getAllProxyOps()
			if lastCnt > 0 {
				c <- cnt - lastCnt
			}
			lastCnt = cnt
			time.Sleep(1 * time.Second)
		}
	}(c)
	return c
}
func pageSlots(r render.Render) {
	r.HTML(200, "slots", nil)
}

func runDashboard(addr string, httpLogFile string) {
	log.Info("dashboard start listen in addr:", addr)
	m := martini.Classic()
	f, err := os.OpenFile(httpLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	m.Map(stdlog.New(f, "[martini]", stdlog.LstdFlags))

	m.Use(martini.Static("assets/statics"))
	m.Use(render.Renderer(render.Options{
		Directory:  "assets/template",
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
	m.Post("/api/migrate", binding.Json(MigrateTaskForm{}), apiDoMigrate)
	m.Get("/api/migrate/tasks", apiGetMigrateTasks)
	m.Delete("/api/migrate/pending_task/:id/remove", apiRemovePendingMigrateTask)
	m.Delete("/api/migrate/task/:id/stop", apiStopMigratingTask)

	m.Get("/api/slot/list", apiGetSlots)
	m.Get("/api/slots", apiGetSlots)
	m.Post("/api/slot", binding.Json(RangeSetTask{}), apiSlotRangeSet)
	m.Get("/api/proxy/list", apiGetProxyList)
	m.Get("/api/proxy/debug/vars", apiGetProxyDebugVars)
	m.Post("/api/proxy", binding.Json(models.ProxyInfo{}), apiSetProxyStatus)

	m.Get("/slots", pageSlots)
	m.Get("/", func(r render.Render) {
		r.Redirect("/admin")
	})

	go func() {
		c := getProxySpeedChan()
		for {
			atomic.StoreInt64(&proxiesSpeed, <-c)
		}
	}()

	go migrateTaskWorker()

	m.RunOnAddr(addr)
}
