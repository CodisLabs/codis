// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/cors"
	"github.com/martini-contrib/render"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type apiServer struct {
	topom *Topom
}

func newApiServer(t *Topom) http.Handler {
	m := martini.New()
	m.Use(martini.Recovery())
	m.Use(render.Renderer())

	m.Use(func(w http.ResponseWriter, req *http.Request, c martini.Context) {
		addr := req.Header.Get("X-Real-IP")
		if addr == "" {
			addr = req.Header.Get("X-Forwarded-For")
			if addr == "" {
				addr = req.RemoteAddr
			}
		}
		path := req.URL.Path
		if req.Method != "GET" && strings.HasPrefix(path, "/api/") {
			log.Infof("[%p] API from %s call %s", t, addr, path)
		}
		c.Next()
	})

	api := &apiServer{topom: t}

	r := martini.NewRouter()

	if binpath, err := filepath.Abs(filepath.Dir(os.Args[0])); err != nil {
		log.WarnErrorf(err, "obtain binpath failed")
	} else if info, err := os.Stat(filepath.Join(binpath, "assets")); err != nil || !info.IsDir() {
		log.WarnErrorf(err, "assets doesn't exist or isn't a directory")
	} else {
		m.Use(martini.Static(filepath.Join(binpath, "assets/statics"), martini.StaticOptions{SkipLogging: true}))
		m.Use(render.Renderer(render.Options{
			Directory:  filepath.Join(binpath, "assets/template"),
			Extensions: []string{".tmpl", ".html"},
			Charset:    "UTF-8",
			IndentJSON: true,
		}))
		m.Use(cors.Allow(&cors.Options{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"GET", "PUT"},
			AllowHeaders:     []string{"Origin", "x-requested-with", "Content-Type", "Content-Range", "Content-Disposition", "Content-Description"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: false,
		}))
		r.Get("/", func(r render.Render) {
			r.Redirect("/admin")
		})
	}

	r.Get("/api/topom", api.Overview)
	r.Get("/api/topom/model", api.Model)
	r.Get("/api/topom/xping/:xauth", api.XPing)
	r.Get("/api/topom/stats/:xauth", api.Stats)

	r.Put("/api/topom/proxy/create/:xauth/:xaddr", api.CreateProxy)
	r.Put("/api/topom/proxy/reinit/:xauth/:token", api.ReinitProxy)
	r.Put("/api/topom/proxy/remove/:xauth/:token/:force", api.RemoveProxy)

	r.Put("/api/topom/group/create/:xauth/:gid", api.CreateGroup)
	r.Put("/api/topom/group/remove/:xauth/:gid", api.RemoveGroup)

	r.Put("/api/topom/group/add/:xauth/:gid/:xaddr", api.GroupAddServer)
	r.Put("/api/topom/group/del/:xauth/:gid/:xaddr", api.GroupDelServer)
	r.Put("/api/topom/group/check/:xauth/:xaddr", api.GroupCheckServer)

	r.Put("/api/topom/group/promote/:xauth/:gid/:xaddr", api.GroupPromoteServer)
	r.Put("/api/topom/group/promote-commit/:xauth/:gid", api.GroupPromoteCommit)

	r.Put("/api/topom/group/repair-master/:xauth/:gid/:xaddr", api.GroupRepairMaster)

	r.Put("/api/topom/action/create/:xauth/:sid/:gid", api.SlotCreateAction)
	r.Put("/api/topom/action/create-range/:xauth/:beg/:end/:gid", api.SlotCreateActionRange)
	r.Put("/api/topom/action/remove/:xauth/:sid", api.SlotRemoveAction)

	r.Put("/api/topom/shutdown/:xauth", api.Shutdown)

	r.Put("/api/topom/set/action/interval/:xauth/:value", api.SetActionInterval)
	r.Put("/api/topom/set/action/disabled/:xauth/:value", api.SetActionDisabled)

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
		return errors.New("missing xauth")
	}
	if xauth != s.topom.GetXAuth() {
		return errors.New("invalid xauth")
	}
	return nil
}

type Overview struct {
	Version string        `json:"version"`
	Compile string        `json:"compile"`
	Config  *Config       `json:"config,omitempty"`
	Model   *models.Topom `json:"model,omitempty"`
	Stats   *Stats        `json:"stats,omitempty"`
}

func (s *apiServer) Overview() (int, string) {
	return rpc.ApiResponseJson(&Overview{
		Version: utils.Version,
		Compile: utils.Compile,
		Config:  s.topom.GetConfig(),
		Model:   s.topom.GetModel(),
		Stats:   s.topom.GetStats(),
	})
}

func (s *apiServer) Model() (int, string) {
	return rpc.ApiResponseJson(s.topom.GetModel())
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
		return rpc.ApiResponseJson(s.topom.GetStats())
	}
}

func (s *apiServer) decodeXAddr(params martini.Params) (string, error) {
	xaddr := params["xaddr"]
	if xaddr == "" {
		return "", errors.New("missing xaddr")
	}
	b, err := base64.StdEncoding.DecodeString(xaddr)
	if err != nil {
		return "", errors.New("invalid xaddr")
	}
	return string(b), nil
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
	addr, err := s.decodeXAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.CreateProxy(addr); err != nil {
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
	groupId, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.CreateGroup(groupId); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) RemoveGroup(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	groupId, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.RemoveGroup(groupId); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) GroupAddServer(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	groupId, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.decodeXAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.GroupAddServer(groupId, addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) GroupDelServer(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	groupId, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.decodeXAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.GroupDelServer(groupId, addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) GroupCheckServer(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.decodeXAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	c, err := NewRedisClient(addr, s.topom.GetConfig().ProductAuth, time.Second)
	if err != nil {
		return rpc.ApiResponseError(fmt.Errorf("create redis-client to %s failed", addr))
	}
	defer c.Close()
	if _, err := c.SlotsInfo(); err != nil {
		return rpc.ApiResponseError(fmt.Errorf("check codis-support of %s failed", addr))
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) GroupPromoteServer(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	groupId, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.decodeXAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.GroupPromoteServer(groupId, addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) GroupPromoteCommit(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	groupId, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.GroupPromoteCommit(groupId); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) GroupRepairMaster(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	groupId, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	addr, err := s.decodeXAddr(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.GroupRepairMaster(groupId, addr); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SlotCreateAction(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	slotId, err := s.parseInteger(params, "sid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	groupId, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SlotCreateAction(slotId, groupId); err != nil {
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
	groupId, err := s.parseInteger(params, "gid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if beg >= 0 && beg <= end && end < models.MaxSlotNum {
		for slotId := beg; slotId <= end; slotId++ {
			if err := s.topom.SlotCreateAction(slotId, groupId); err != nil {
				return rpc.ApiResponseError(err)
			}
		}
		return rpc.ApiResponseJson("OK")
	} else {
		return rpc.ApiResponseError(fmt.Errorf("invalid slot range [%d,%d]", beg, end))
	}
}

func (s *apiServer) SlotRemoveAction(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	slotId, err := s.parseInteger(params, "sid")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.SlotRemoveAction(slotId); err != nil {
		return rpc.ApiResponseError(err)
	} else {
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

func (s *apiServer) SetActionInterval(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	value, err := s.parseInteger(params, "value")
	if err != nil {
		return rpc.ApiResponseError(err)
	} else {
		s.topom.SetActionInterval(value)
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) SetActionDisabled(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	value, err := s.parseInteger(params, "value")
	if err != nil {
		return rpc.ApiResponseError(err)
	} else {
		s.topom.SetActionDisabled(value != 0)
		return rpc.ApiResponseJson("OK")
	}
}

type ApiClient struct {
	addr  string
	xauth string
}

func NewApiClient(addr string) *ApiClient {
	return &ApiClient{addr: addr}
}

func (c *ApiClient) SetXAuth(name, auth string) {
	c.xauth = rpc.NewXAuth(name, auth)
}

func (c *ApiClient) encodeURL(format string, args ...interface{}) string {
	return rpc.EncodeURL(c.addr, format, args...)
}

func (c *ApiClient) encodeXAddr(addr string) string {
	return base64.StdEncoding.EncodeToString([]byte(addr))
}

func (c *ApiClient) Overview() (*Overview, error) {
	url := c.encodeURL("/api/topom")
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

func (c *ApiClient) CreateProxy(addr string) error {
	url := c.encodeURL("/api/topom/proxy/create/%s/%s", c.xauth, c.encodeXAddr(addr))
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

func (c *ApiClient) CreateGroup(groupId int) error {
	url := c.encodeURL("/api/topom/group/create/%s/%d", c.xauth, groupId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) RemoveGroup(groupId int) error {
	url := c.encodeURL("/api/topom/group/remove/%s/%d", c.xauth, groupId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupAddServer(groupId int, addr string) error {
	url := c.encodeURL("/api/topom/group/add/%s/%d/%s", c.xauth, groupId, c.encodeXAddr(addr))
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupDelServer(groupId int, addr string) error {
	url := c.encodeURL("/api/topom/group/del/%s/%d/%s", c.xauth, groupId, c.encodeXAddr(addr))
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupCheckServer(addr string) error {
	url := c.encodeURL("/api/topom/group/check/%s/%s", c.xauth, c.encodeXAddr(addr))
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupPromoteServer(groupId int, addr string) error {
	url := c.encodeURL("/api/topom/group/promote/%s/%d/%s", c.xauth, groupId, c.encodeXAddr(addr))
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupPromoteCommit(groupId int) error {
	url := c.encodeURL("/api/topom/group/promote-commit/%s/%d", c.xauth, groupId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupRepairMaster(groupId int, addr string) error {
	url := c.encodeURL("/api/topom/group/repair-master/%s/%d/%s", c.xauth, groupId, c.encodeXAddr(addr))
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotCreateAction(slotId int, groupId int) error {
	url := c.encodeURL("/api/topom/action/create/%s/%d/%d", c.xauth, slotId, groupId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotCreateActionRange(beg, end int, groupId int) error {
	url := c.encodeURL("/api/topom/action/create-range/%s/%d/%d/%d", c.xauth, beg, end, groupId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotRemoveAction(slotId int) error {
	url := c.encodeURL("/api/topom/action/remove/%s/%d", c.xauth, slotId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) Shutdown() error {
	url := c.encodeURL("/api/topom/shutdown/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SetActionInterval(msecs int) error {
	url := c.encodeURL("/api/topom/set/action/interval/%s/%d", c.xauth, msecs)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SetActionDisabled(disabled bool) error {
	var value int
	if disabled {
		value = 1
	}
	url := c.encodeURL("/api/topom/set/action/disabled/%s/%d", c.xauth, value)
	return rpc.ApiPutJson(url, nil, nil)
}
