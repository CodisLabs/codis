// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/gzip"
	"github.com/martini-contrib/render"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/redis"
	"github.com/CodisLabs/codis/pkg/utils/rpc"
)

type apiServer struct {
	topom *Topom
}

func newApiServer(t *Topom) http.Handler {
	m := martini.New()
	m.Use(martini.Recovery())
	m.Use(render.Renderer())
	m.Use(func(w http.ResponseWriter, req *http.Request, c martini.Context) {
		path := req.URL.Path
		if req.Method != "GET" && strings.HasPrefix(path, "/api/") {
			var remoteAddr = req.RemoteAddr
			var headerAddr string
			for _, key := range []string{"X-Real-IP", "X-Forwarded-For"} {
				if val := req.Header.Get(key); val != "" {
					headerAddr = val
					break
				}
			}
			log.Warnf("[%p] API call %s from %s [%s]", t, path, remoteAddr, headerAddr)
		}
		c.Next()
	})
	m.Use(gzip.All())
	m.Use(func(c martini.Context, w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	})

	api := &apiServer{topom: t}

	r := martini.NewRouter()

	r.Get("/", func(r render.Render) {
		r.Redirect("/topom")
	})
	r.Any("/debug/**", func(w http.ResponseWriter, req *http.Request) {
		http.DefaultServeMux.ServeHTTP(w, req)
	})

	r.Group("/topom", func(r martini.Router) {
		r.Get("", api.Overview)
		r.Get("/model", api.Model)
		r.Get("/stats", api.StatsNoXAuth)
		r.Get("/slots", api.SlotsNoXAuth)
	})
	r.Group("/api/topom", func(r martini.Router) {
		r.Get("/model", api.Model)
		r.Get("/xping/:xauth", api.XPing)
		r.Get("/stats/:xauth", api.Stats)
		r.Get("/slots/:xauth", api.Slots)
		r.Put("/reload/:xauth", api.Reload)
		r.Put("/shutdown/:xauth", api.Shutdown)
		r.Put("/loglevel/:xauth/:value", api.LogLevel)
		r.Group("/proxy", func(r martini.Router) {
			r.Put("/create/:xauth/:addr", api.CreateProxy)
			r.Put("/online/:xauth/:addr", api.OnlineProxy)
			r.Put("/reinit/:xauth/:token", api.ReinitProxy)
			r.Put("/remove/:xauth/:token/:force", api.RemoveProxy)
		})
		r.Group("/group", func(r martini.Router) {
			r.Put("/create/:xauth/:gid", api.CreateGroup)
			r.Put("/remove/:xauth/:gid", api.RemoveGroup)
			r.Put("/resync/:xauth/:gid", api.ResyncGroup)
			r.Put("/resync-all/:xauth", api.ResyncGroupAll)
			r.Put("/add/:xauth/:gid/:addr", api.GroupAddServer)
			r.Put("/add/:xauth/:gid/:addr/:datacenter", api.GroupAddServer)
			r.Put("/del/:xauth/:gid/:addr", api.GroupDelServer)
			r.Put("/promote/:xauth/:gid/:addr", api.GroupPromoteServer)
			r.Put("/replica-groups/:xauth/:gid/:addr/:value", api.EnableReplicaGroups)
			r.Put("/replica-groups-all/:xauth/:value", api.EnableReplicaGroupsAll)
			r.Group("/action", func(r martini.Router) {
				r.Put("/create/:xauth/:addr", api.SyncCreateAction)
				r.Put("/remove/:xauth/:addr", api.SyncRemoveAction)
			})
			r.Get("/info/:addr", api.InfoServer)
		})
		r.Group("/slots", func(r martini.Router) {
			r.Group("/action", func(r martini.Router) {
				r.Put("/create/:xauth/:sid/:gid", api.SlotCreateAction)
				r.Put("/create-some/:xauth/:src/:dst/:num", api.SlotCreateActionSome)
				r.Put("/create-range/:xauth/:beg/:end/:gid", api.SlotCreateActionRange)
				r.Put("/remove/:xauth/:sid", api.SlotRemoveAction)
				r.Put("/interval/:xauth/:value", api.SetSlotActionInterval)
				r.Put("/disabled/:xauth/:value", api.SetSlotActionDisabled)
			})
			r.Put("/assign/:xauth", binding.Json([]*models.SlotMapping{}), api.SlotsAssignGroup)
			r.Put("/assign/:xauth/offline", binding.Json([]*models.SlotMapping{}), api.SlotsAssignOffline)
			r.Put("/rebalance/:xauth/:confirm", api.SlotsRebalance)
		})
		r.Group("/sentinels", func(r martini.Router) {
			r.Put("/add/:xauth/:addr", api.AddSentinel)
			r.Put("/del/:xauth/:addr/:force", api.DelSentinel)
			r.Put("/resync-all/:xauth", api.ResyncSentinels)
			r.Get("/info/:addr", api.InfoSentinel)
			r.Get("/info/:addr/monitored", api.InfoSentinelMonitored)
		})
	})

	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	return m
}

func (s *apiServer) verifyXAuth(params martini.Params) error {
	if s.topom.IsClosed() {
		return ErrClosedTopom
	}
	xauth := params["xauth"]
	if xauth == "" {
		return errors.New("missing xauth, please check product name & auth")
	}
	if xauth != s.topom.XAuth() {
		return errors.New("invalid xauth, please check product name & auth")
	}
	return nil
}

func (s *apiServer) Overview() (int, string) {
	o, err := s.topom.Overview()
	if err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(o)
	}
}

func (s *apiServer) Model() (int, string) {
	return rpc.ApiResponseJson(s.topom.Model())
}

func (s *apiServer) StatsNoXAuth() (int, string) {
	if stats, err := s.topom.Stats(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(stats)
	}
}

func (s *apiServer) SlotsNoXAuth() (int, string) {
	if slots, err := s.topom.Slots(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(slots)
	}
}

func (s *apiServer) XPing(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) Stats(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return s.StatsNoXAuth()
	}
}

func (s *apiServer) Slots(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return s.SlotsNoXAuth()
	}
}

func (s *apiServer) Reload(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.Reload(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) parseAddr(params martini.Params) (string, error) {
	addr := params["addr"]
	if addr == "" {
		return "", errors.New("missing addr")
	}
	return addr, nil
}

func (s *apiServer) parseToken(params martini.Params) (string, error) {
	token := params["token"]
	if token == "" {
		return "", errors.New("missing token")
	}
	return token, nil
}

func (s *apiServer) parseInteger(params martini.Params, entry string) (int, error) {
	text := params[entry]
	if text == "" {
		return 0, fmt.Errorf("missing %s", entry)
	}
	v, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", entry)
	}
	return v, nil
}

func (s *apiServer) CreateProxy(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.CreateProxy(addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) OnlineProxy(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.OnlineProxy(addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) ReinitProxy(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	token, err := s.parseToken(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.ReinitProxy(token); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) RemoveProxy(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	token, err := s.parseToken(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	force, err := s.parseInteger(params, "force")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.RemoveProxy(token, force != 0); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) CreateGroup(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	gid, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.CreateGroup(gid); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) RemoveGroup(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	gid, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.RemoveGroup(gid); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) ResyncGroup(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	gid, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.ResyncGroup(gid); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) ResyncGroupAll(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.ResyncGroupAll(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) GroupAddServer(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	gid, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	dc := params["datacenter"]
	c, err := redis.NewClient(addr, s.topom.Config().ProductAuth, time.Second)
	if err != nil {
		log.WarnErrorf(err, "create redis client to %s failed", addr)
		return rpc.ApiResponseError(err)
	}
	defer c.Close()
	if _, err := c.SlotsInfo(); err != nil {
		log.WarnErrorf(err, "redis %s check slots-info failed", addr)
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.GroupAddServer(gid, dc, addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) GroupDelServer(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	gid, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.GroupDelServer(gid, addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) GroupPromoteServer(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	gid, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.GroupPromoteServer(gid, addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) EnableReplicaGroups(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	gid, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	n, err := s.parseInteger(params, "value")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.EnableReplicaGroups(gid, addr, n != 0); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) EnableReplicaGroupsAll(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	n, err := s.parseInteger(params, "value")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.EnableReplicaGroupsAll(n != 0); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) AddSentinel(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.AddSentinel(addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) DelSentinel(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	force, err := s.parseInteger(params, "force")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.DelSentinel(addr, force != 0); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) ResyncSentinels(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.ResyncSentinels(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) InfoServer(params martini.Params) (int, string) {
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	c, err := redis.NewClient(addr, s.topom.Config().ProductAuth, time.Second)
	if err != nil {
		log.WarnErrorf(err, "create redis client to %s failed", addr)
		return rpc.ApiResponseError(err)
	}
	defer c.Close()
	if info, err := c.InfoFull(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(info)
	}
}

func (s *apiServer) InfoSentinel(params martini.Params) (int, string) {
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	c, err := redis.NewClientNoAuth(addr, time.Second)
	if err != nil {
		log.WarnErrorf(err, "create redis client to %s failed", addr)
		return rpc.ApiResponseError(err)
	}
	defer c.Close()
	if info, err := c.Info(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(info)
	}
}

func (s *apiServer) InfoSentinelMonitored(params martini.Params) (int, string) {
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	sentinel := redis.NewSentinel(s.topom.Config().ProductName, s.topom.Config().ProductAuth)
	if info, err := sentinel.MastersAndSlaves(addr, s.topom.Config().SentinelClientTimeout.Duration()); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(info)
	}
}

func (s *apiServer) SyncCreateAction(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SyncCreateAction(addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SyncRemoveAction(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.parseAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SyncRemoveAction(addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SlotCreateAction(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	sid, err := s.parseInteger(params, "sid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	gid, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SlotCreateAction(sid, gid); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SlotCreateActionSome(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	groupFrom, err := s.parseInteger(params, "src")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	groupTo, err := s.parseInteger(params, "dst")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	numSlots, err := s.parseInteger(params, "num")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SlotCreateActionSome(groupFrom, groupTo, numSlots); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SlotCreateActionRange(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	beg, err := s.parseInteger(params, "beg")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	end, err := s.parseInteger(params, "end")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	gid, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SlotCreateActionRange(beg, end, gid, true); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SlotRemoveAction(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	sid, err := s.parseInteger(params, "sid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SlotRemoveAction(sid); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) LogLevel(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	v := params["value"]
	if v == "" {
		return rpc.ApiResponseError(errors.New("missing loglevel"))
	}
	if !log.SetLevelString(v) {
		return rpc.ApiResponseError(errors.New("invalid loglevel"))
	} else {
		log.Warnf("set loglevel to %s", v)
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) Shutdown(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.Close(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SetSlotActionInterval(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	value, err := s.parseInteger(params, "value")
	if err != nil {
		return rpc.ApiResponseError(err)
	} else {
		s.topom.SetSlotActionInterval(value)
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SetSlotActionDisabled(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	value, err := s.parseInteger(params, "value")
	if err != nil {
		return rpc.ApiResponseError(err)
	} else {
		s.topom.SetSlotActionDisabled(value != 0)
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SlotsAssignGroup(slots []*models.SlotMapping, params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SlotsAssignGroup(slots); err != nil {
		return rpc.ApiResponseError(err)
	}
	return rpc.ApiResponseJson("OK")
}

func (s *apiServer) SlotsAssignOffline(slots []*models.SlotMapping, params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SlotsAssignOffline(slots); err != nil {
		return rpc.ApiResponseError(err)
	}
	return rpc.ApiResponseJson("OK")
}

func (s *apiServer) SlotsRebalance(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	confirm, err := s.parseInteger(params, "confirm")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if plans, err := s.topom.SlotsRebalance(confirm != 0); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		m := make(map[string]int)
		for sid, gid := range plans {
			m[strconv.Itoa(sid)] = gid
		}
		return rpc.ApiResponseJson(m)
	}
}

type ApiClient struct {
	addr  string
	xauth string
}

func NewApiClient(addr string) *ApiClient {
	return &ApiClient{addr: addr}
}

func (c *ApiClient) SetXAuth(name string) {
	c.xauth = rpc.NewXAuth(name)
}

func (c *ApiClient) encodeURL(format string, args ...interface{}) string {
	return rpc.EncodeURL(c.addr, format, args...)
}

func (c *ApiClient) Overview() (*Overview, error) {
	url := c.encodeURL("/topom")
	var o = &Overview{}
	if err := rpc.ApiGetJson(url, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (c *ApiClient) Model() (*models.Topom, error) {
	url := c.encodeURL("/api/topom/model")
	model := &models.Topom{}
	if err := rpc.ApiGetJson(url, model); err != nil {
		return nil, err
	}
	return model, nil
}

func (c *ApiClient) XPing() error {
	url := c.encodeURL("/api/topom/xping/%s", c.xauth)
	return rpc.ApiGetJson(url, nil)
}

func (c *ApiClient) Stats() (*Stats, error) {
	url := c.encodeURL("/api/topom/stats/%s", c.xauth)
	stats := &Stats{}
	if err := rpc.ApiGetJson(url, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func (c *ApiClient) Slots() ([]*models.Slot, error) {
	url := c.encodeURL("/api/topom/slots/%s", c.xauth)
	slots := []*models.Slot{}
	if err := rpc.ApiGetJson(url, &slots); err != nil {
		return nil, err
	}
	return slots, nil
}

func (c *ApiClient) Reload() error {
	url := c.encodeURL("/api/topom/reload/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) LogLevel(level log.LogLevel) error {
	url := c.encodeURL("/api/topom/loglevel/%s/%s", c.xauth, level)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) Shutdown() error {
	url := c.encodeURL("/api/topom/shutdown/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) CreateProxy(addr string) error {
	url := c.encodeURL("/api/topom/proxy/create/%s/%s", c.xauth, addr)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) OnlineProxy(addr string) error {
	url := c.encodeURL("/api/topom/proxy/online/%s/%s", c.xauth, addr)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) ReinitProxy(token string) error {
	url := c.encodeURL("/api/topom/proxy/reinit/%s/%s", c.xauth, token)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) RemoveProxy(token string, force bool) error {
	var value int
	if force {
		value = 1
	}
	url := c.encodeURL("/api/topom/proxy/remove/%s/%s/%d", c.xauth, token, value)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) CreateGroup(gid int) error {
	url := c.encodeURL("/api/topom/group/create/%s/%d", c.xauth, gid)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) RemoveGroup(gid int) error {
	url := c.encodeURL("/api/topom/group/remove/%s/%d", c.xauth, gid)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) ResyncGroup(gid int) error {
	url := c.encodeURL("/api/topom/group/resync/%s/%d", c.xauth, gid)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) ResyncGroupAll() error {
	url := c.encodeURL("/api/topom/group/resync-all/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupAddServer(gid int, dc, addr string) error {
	var url string
	if dc != "" {
		url = c.encodeURL("/api/topom/group/add/%s/%d/%s/%s", c.xauth, gid, addr, dc)
	} else {
		url = c.encodeURL("/api/topom/group/add/%s/%d/%s", c.xauth, gid, addr)
	}
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupDelServer(gid int, addr string) error {
	url := c.encodeURL("/api/topom/group/del/%s/%d/%s", c.xauth, gid, addr)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupPromoteServer(gid int, addr string) error {
	url := c.encodeURL("/api/topom/group/promote/%s/%d/%s", c.xauth, gid, addr)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) EnableReplicaGroups(gid int, addr string, value bool) error {
	var n int
	if value {
		n = 1
	}
	url := c.encodeURL("/api/topom/group/replica-groups/%s/%d/%s/%d", c.xauth, gid, addr, n)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) EnableReplicaGroupsAll(value bool) error {
	var n int
	if value {
		n = 1
	}
	url := c.encodeURL("/api/topom/group/replica-groups-all/%s/%d", c.xauth, n)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) AddSentinel(addr string) error {
	url := c.encodeURL("/api/topom/sentinels/add/%s/%s", c.xauth, addr)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) DelSentinel(addr string, force bool) error {
	var value int
	if force {
		value = 1
	}
	url := c.encodeURL("/api/topom/sentinels/del/%s/%s/%d", c.xauth, addr, value)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) ResyncSentinels() error {
	url := c.encodeURL("/api/topom/sentinels/resync-all/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SyncCreateAction(addr string) error {
	url := c.encodeURL("/api/topom/group/action/create/%s/%s", c.xauth, addr)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SyncRemoveAction(addr string) error {
	url := c.encodeURL("/api/topom/group/action/remove/%s/%s", c.xauth, addr)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotCreateAction(sid int, gid int) error {
	url := c.encodeURL("/api/topom/slots/action/create/%s/%d/%d", c.xauth, sid, gid)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotCreateActionSome(groupFrom, groupTo int, numSlots int) error {
	url := c.encodeURL("/api/topom/slots/action/create-some/%s/%d/%d/%d", c.xauth, groupFrom, groupTo, numSlots)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotCreateActionRange(beg, end int, gid int) error {
	url := c.encodeURL("/api/topom/slots/action/create-range/%s/%d/%d/%d", c.xauth, beg, end, gid)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotRemoveAction(sid int) error {
	url := c.encodeURL("/api/topom/slots/action/remove/%s/%d", c.xauth, sid)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SetSlotActionInterval(usecs int) error {
	url := c.encodeURL("/api/topom/slots/action/interval/%s/%d", c.xauth, usecs)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SetSlotActionDisabled(disabled bool) error {
	var value int
	if disabled {
		value = 1
	}
	url := c.encodeURL("/api/topom/slots/action/disabled/%s/%d", c.xauth, value)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotsAssignGroup(slots []*models.SlotMapping) error {
	url := c.encodeURL("/api/topom/slots/assign/%s", c.xauth)
	return rpc.ApiPutJson(url, slots, nil)
}

func (c *ApiClient) SlotsAssignOffline(slots []*models.SlotMapping) error {
	url := c.encodeURL("/api/topom/slots/assign/%s/offline", c.xauth)
	return rpc.ApiPutJson(url, slots, nil)
}

func (c *ApiClient) SlotsRebalance(confirm bool) (map[int]int, error) {
	var value int
	if confirm {
		value = 1
	}
	url := c.encodeURL("/api/topom/slots/rebalance/%s/%d", c.xauth, value)
	var plans = make(map[string]int)
	if err := rpc.ApiPutJson(url, nil, &plans); err != nil {
		return nil, err
	} else {
		var m = make(map[int]int)
		for sid, gid := range plans {
			n, err := strconv.Atoi(sid)
			if err != nil {
				return nil, errors.Trace(err)
			}
			m[n] = gid
		}
		return m, nil
	}
}
