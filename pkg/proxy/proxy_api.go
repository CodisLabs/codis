package proxy

import (
	"net/http"
	"strings"
	"sync"

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

type apiServer struct {
	proxy *Proxy
	sync.RWMutex
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
		if req.Method != "GET" && strings.HasPrefix(path, "/api") {
			log.Infof("[%p] API from %s call %s", p, addr, path)
		}
		c.Next()
	})

	api := &apiServer{proxy: p}

	r := martini.NewRouter()
	r.Get("/overview", api.Overview)

	r.Get("/api/model", api.Model)
	r.Get("/api/xping/:xauth", api.XPing)
	r.Get("/api/stats/:xauth", api.Stats)
	r.Get("/api/slots/:xauth", api.Slots)

	r.Put("/api/start/:xauth", api.Start)
	r.Put("/api/shutdown/:xauth", api.Shutdown)
	r.Put("/api/fillslots/:xauth", binding.Json([]*models.Slot{}), api.FillSlots)

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
	if xauth != s.proxy.GetXAuth() {
		return errors.New("invalid xauth")
	}
	return nil
}

func (s *apiServer) Overview() (int, string) {
	s.RLock()
	defer s.RUnlock()
	overview := &struct {
		Version string         `json:"version"`
		Compile string         `json:"compile"`
		Config  *Config        `json:"config"`
		Model   *models.Proxy  `json:"model"`
		Slots   []*models.Slot `json:"slots"`
		Stats   *Stats         `json:"stats"`
	}{
		utils.Version,
		utils.Compile,
		s.proxy.GetConfig(),
		s.proxy.GetModel(),
		s.proxy.GetSlots(),
		s.newStats(),
	}
	return rpc.ApiResponseJson(overview)
}

func (s *apiServer) newStats() *Stats {
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
	s.RLock()
	defer s.RUnlock()
	return rpc.ApiResponseJson(s.proxy.GetModel())
}

func (s *apiServer) Stats(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(s.newStats())
	}
}

func (s *apiServer) XPing(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) Slots(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(s.proxy.GetSlots())
	}
}

func (s *apiServer) Start(params martini.Params) (int, string) {
	s.Lock()
	defer s.Unlock()
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
	s.Lock()
	defer s.Unlock()
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
	s.Lock()
	defer s.Unlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	for _, slot := range slots {
		if err := s.proxy.FillSlot(slot.Id, slot.BackendAddr, slot.MigrateFrom, slot.Locked); err != nil {
			return rpc.ApiResponseError(err)
		}
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

func (c *ApiClient) Overview() (map[string]interface{}, error) {
	url := c.encodeURL("/overview")
	var v interface{}
	if err := rpc.ApiGetJson(url, &v); err != nil {
		return nil, err
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m, nil
	} else {
		return nil, errors.New("invalid json map")
	}
}

func (c *ApiClient) Model() (*models.Proxy, error) {
	url := c.encodeURL("/api/model")
	model := &models.Proxy{}
	if err := rpc.ApiGetJson(url, model); err != nil {
		return nil, err
	}
	return model, nil
}

func (c *ApiClient) XPing() error {
	url := c.encodeURL("/api/xping/%s", c.xauth)
	return rpc.ApiGetJson(url, nil)
}

func (c *ApiClient) Stats() (*Stats, error) {
	url := c.encodeURL("/api/stats/%s", c.xauth)
	stats := &Stats{}
	if err := rpc.ApiGetJson(url, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func (c *ApiClient) Slots() ([]*models.Slot, error) {
	url := c.encodeURL("/api/slots/%s", c.xauth)
	slots := []*models.Slot{}
	if err := rpc.ApiGetJson(url, &slots); err != nil {
		return nil, err
	}
	return slots, nil
}

func (c *ApiClient) Start() error {
	url := c.encodeURL("/api/start/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) Shutdown() error {
	url := c.encodeURL("/api/shutdown/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) FillSlots(slots ...*models.Slot) error {
	url := c.encodeURL("/api/fillslots/%s", c.xauth)
	return rpc.ApiPutJson(url, slots, nil)
}
