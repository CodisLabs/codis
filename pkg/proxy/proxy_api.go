// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"net/http"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/router"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type apiServer struct {
	proxy *Proxy
}

func newApiServer(p *Proxy) http.Handler {
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
			log.Infof("[%p] API from %s call %s", p, addr, path)
		}
		c.Next()
	})

	api := &apiServer{proxy: p}

	r := martini.NewRouter()

	r.Get("/api/proxy", api.Overview)
	r.Get("/api/proxy/model", api.Model)
	r.Get("/api/proxy/xping/:xauth", api.XPing)
	r.Get("/api/proxy/stats/:xauth", api.Stats)
	r.Get("/api/proxy/slots/:xauth", api.Slots)

	r.Put("/api/proxy/start/:xauth", api.Start)
	r.Put("/api/proxy/shutdown/:xauth", api.Shutdown)
	r.Put("/api/proxy/fillslots/:xauth", binding.Json([]*models.Slot{}), api.FillSlots)

	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	return m
}

func (s *apiServer) verifyXAuth(params martini.Params) error {
	if s.proxy.IsClosed() {
		return ErrClosedProxy
	}
	xauth := params["xauth"]
	if xauth == "" {
		return errors.New("missing xauth")
	}
	if xauth != s.proxy.XAuth() {
		return errors.New("invalid xauth")
	}
	return nil
}

type Overview struct {
	Version string         `json:"version"`
	Compile string         `json:"compile"`
	Config  *Config        `json:"config,omitempty"`
	Model   *models.Proxy  `json:"model,omitempty"`
	Slots   []*models.Slot `json:"slots,omitempty"`
	Stats   *Stats         `json:"stats,omitempty"`
}

type Stats struct {
	Online bool `json:"online"`
	Closed bool `json:"closed"`

	Ops struct {
		Total int64             `json:"total"`
		Cmds  []*router.OpStats `json:"cmds,omitempty"`
	} `json:"ops"`

	Sessions struct {
		Total   int64 `json:"total"`
		Actived int64 `json:"actived"`
	} `json:"sessions"`
}

func (s *apiServer) Overview() (int, string) {
	return rpc.ApiResponseJson(&Overview{
		Version: utils.Version,
		Compile: utils.Compile,
		Config:  s.proxy.Config(),
		Model:   s.proxy.Model(),
		Slots:   s.proxy.Slots(),
		Stats:   s.NewStats(),
	})
}

func (s *apiServer) NewStats() *Stats {
	stats := &Stats{}
	stats.Online = s.proxy.IsOnline()
	stats.Closed = s.proxy.IsClosed()

	stats.Ops.Total = router.OpsTotal()
	stats.Ops.Cmds = router.GetAllOpStats()

	stats.Sessions.Total = router.SessionsTotal()
	stats.Sessions.Actived = router.SessionsActived()
	return stats
}

func (s *apiServer) Model() (int, string) {
	return rpc.ApiResponseJson(s.proxy.Model())
}

func (s *apiServer) Stats(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(s.NewStats())
	}
}

func (s *apiServer) XPing(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) Slots(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(s.proxy.Slots())
	}
}

func (s *apiServer) Start(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.proxy.Start(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) Shutdown(params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.proxy.Close(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) FillSlots(slots []*models.Slot, params martini.Params) (int, string) {
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.proxy.FillSlots(slots); err != nil {
		return rpc.ApiResponseError(err)
	}
	return rpc.ApiResponseJson("OK")
}

type ApiClient struct {
	addr  string
	xauth string
}

func NewApiClient(addr string) *ApiClient {
	return &ApiClient{addr: addr}
}

func (c *ApiClient) SetXAuth(name, auth string, token string) {
	c.xauth = rpc.NewXAuth(name, auth, token)
}

func (c *ApiClient) encodeURL(format string, args ...interface{}) string {
	return rpc.EncodeURL(c.addr, format, args...)
}

func (c *ApiClient) Overview() (*Overview, error) {
	url := c.encodeURL("/api/proxy")
	var o = &Overview{}
	if err := rpc.ApiGetJson(url, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (c *ApiClient) Model() (*models.Proxy, error) {
	url := c.encodeURL("/api/proxy/model")
	model := &models.Proxy{}
	if err := rpc.ApiGetJson(url, model); err != nil {
		return nil, err
	}
	return model, nil
}

func (c *ApiClient) XPing() error {
	url := c.encodeURL("/api/proxy/xping/%s", c.xauth)
	return rpc.ApiGetJson(url, nil)
}

func (c *ApiClient) Stats() (*Stats, error) {
	url := c.encodeURL("/api/proxy/stats/%s", c.xauth)
	stats := &Stats{}
	if err := rpc.ApiGetJson(url, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func (c *ApiClient) Slots() ([]*models.Slot, error) {
	url := c.encodeURL("/api/proxy/slots/%s", c.xauth)
	slots := []*models.Slot{}
	if err := rpc.ApiGetJson(url, &slots); err != nil {
		return nil, err
	}
	return slots, nil
}

func (c *ApiClient) Start() error {
	url := c.encodeURL("/api/proxy/start/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) Shutdown() error {
	url := c.encodeURL("/api/proxy/shutdown/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) FillSlots(slots ...*models.Slot) error {
	url := c.encodeURL("/api/proxy/fillslots/%s", c.xauth)
	return rpc.ApiPutJson(url, slots, nil)
}
