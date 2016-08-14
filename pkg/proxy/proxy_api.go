// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"net/http"
	"runtime"
	"strings"

	_ "net/http/pprof"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/gzip"
	"github.com/martini-contrib/render"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/rpc"
)

type apiServer struct {
	proxy *Proxy
}

func newApiServer(p *Proxy) http.Handler {
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
			log.Warnf("[%p] API call %s from %s [%s]", p, path, remoteAddr, headerAddr)
		}
		c.Next()
	})
	m.Use(gzip.All())
	m.Use(func(c martini.Context, w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	})

	api := &apiServer{proxy: p}

	r := martini.NewRouter()
	r.Get("/", func(r render.Render) {
		r.Redirect("/proxy")
	})
	r.Any("/debug/**", func(w http.ResponseWriter, req *http.Request) {
		http.DefaultServeMux.ServeHTTP(w, req)
	})

	r.Group("/proxy", func(r martini.Router) {
		r.Get("", api.Overview)
		r.Get("/model", api.Model)
		r.Get("/stats", api.StatsNoXAuth)
		r.Get("/slots", api.SlotsNoXAuth)
	})
	r.Group("/api/proxy", func(r martini.Router) {
		r.Get("/model", api.Model)
		r.Get("/xping/:xauth", api.XPing)
		r.Get("/stats/:xauth", api.Stats)
		r.Get("/slots/:xauth", api.Slots)
		r.Put("/start/:xauth", api.Start)
		r.Put("/shutdown/:xauth", api.Shutdown)
		r.Put("/loglevel/:xauth/:value", api.LogLevel)
		r.Put("/fillslots/:xauth", binding.Json([]*models.Slot{}), api.FillSlots)
	})

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
	Stats   *Stats         `json:"stats,omitempty"`
	Slots   []*models.Slot `json:"slots,omitempty"`
}

type Stats struct {
	Online bool `json:"online"`
	Closed bool `json:"closed"`

	Ops struct {
		Total int64      `json:"total"`
		Fails int64      `json:"fails"`
		Qps   int64      `json:"qps"`
		Cmd   []*OpStats `json:"cmd,omitempty"`
	} `json:"ops"`

	Sessions struct {
		Total int64 `json:"total"`
		Alive int64 `json:"alive"`
	} `json:"sessions"`

	Rusage struct {
		Mem int64   `json:"mem"`
		CPU float64 `json:"cpu"`
	} `json:"rusage"`

	Runtime struct {
		Alloc          uint64  `json:"alloc"`
		TotalAlloc     uint64  `json:"total_alloc"`
		Sys            uint64  `json:"sys"`
		Lookups        uint64  `json:"lookups"`
		Mallocs        uint64  `json:"mallocs"`
		Frees          uint64  `json:"frees"`
		HeapAlloc      uint64  `json:"heap_alloc"`
		HeapSys        uint64  `json:"heap_sys"`
		HeapIdle       uint64  `json:"heap_idle"`
		HeapInuse      uint64  `json:"heap_inuse"`
		HeapReleased   uint64  `json:"heap_released"`
		HeapObjects    uint64  `json:"heap_objects"`
		NumGC          uint32  `json:"num_gc"`
		NumProcs       int     `json:"num_procs"`
		NumGoroutines  int     `json:"num_goroutines"`
		NumCgoCall     int64   `json:"num_cgo_call"`
		GCCPUFraction  float64 `json:"gc_cpu_fraction"`
		GCPauseTotalNs uint64  `json:"gc_pause_total_nanoseconds"`
	} `json:"runtime"`
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

	stats.Ops.Total = OpTotal()
	stats.Ops.Fails = OpFails()
	stats.Ops.Qps = OpQps()
	stats.Ops.Cmd = GetOpStatsAll()

	stats.Sessions.Total = SessionsTotal()
	stats.Sessions.Alive = SessionsAlive()

	stats.Rusage.Mem = GetSysMemTotal()
	stats.Rusage.CPU = GetSysCPUUsage()

	var r runtime.MemStats
	runtime.ReadMemStats(&r)

	stats.Runtime.Alloc = r.Alloc
	stats.Runtime.TotalAlloc = r.TotalAlloc
	stats.Runtime.Sys = r.Sys
	stats.Runtime.Lookups = r.Lookups
	stats.Runtime.Mallocs = r.Mallocs
	stats.Runtime.Frees = r.Frees
	stats.Runtime.HeapAlloc = r.HeapAlloc
	stats.Runtime.HeapSys = r.HeapSys
	stats.Runtime.HeapIdle = r.HeapIdle
	stats.Runtime.HeapInuse = r.HeapInuse
	stats.Runtime.HeapReleased = r.HeapReleased
	stats.Runtime.HeapObjects = r.HeapObjects
	stats.Runtime.NumGC = r.NumGC
	stats.Runtime.NumProcs = runtime.GOMAXPROCS(0)
	stats.Runtime.NumGoroutines = runtime.NumGoroutine()
	stats.Runtime.NumCgoCall = runtime.NumCgoCall()
	stats.Runtime.GCCPUFraction = r.GCCPUFraction
	stats.Runtime.GCPauseTotalNs = r.PauseTotalNs

	return stats
}

func (s *apiServer) Model() (int, string) {
	return rpc.ApiResponseJson(s.proxy.Model())
}

func (s *apiServer) StatsNoXAuth() (int, string) {
	return rpc.ApiResponseJson(s.NewStats())
}

func (s *apiServer) SlotsNoXAuth() (int, string) {
	return rpc.ApiResponseJson(s.proxy.Slots())
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
	url := c.encodeURL("/proxy")
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

func (c *ApiClient) LogLevel(level log.LogLevel) error {
	url := c.encodeURL("/api/proxy/loglevel/%s/%s", c.xauth, level)
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
