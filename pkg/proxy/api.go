package proxy

import (
	"net/http"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/router"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type Summary struct {
	Version string `json:"version"`
	Compile string `json:"compile"`

	Config *Config `json:"config"`
	Online bool    `json:"online"`
	Closed bool    `json:"closed"`

	Model *models.Proxy  `json:"model"`
	Slots []*models.Slot `json:"slots,omitempty"`
	Stats *Stats         `json:"stats,omitempty"`
}

type Stats struct {
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
}

func newApiServer(p *Proxy) http.Handler {
	m := martini.New()
	m.Use(martini.Recovery())
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

	api := &apiServer{p}

	r := martini.NewRouter()
	r.Get("/", api.Summary)
	r.Get("/api/stats/:token/:xauth", api.Stats)
	r.Put("/api/start/:token/:xauth", api.Start)
	r.Put("/api/shutdown/:token/:xauth", api.Shutdown)
	r.Put("/api/fillslot/:token/:xauth", binding.Json([]*models.Slot{}), api.FillSlot)

	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	return m
}

func (s *apiServer) verifyToken(params martini.Params) error {
	token := params["token"]
	if token == "" {
		return errors.New("Missing Token")
	}
	if token != s.proxy.GetToken() {
		return errors.New("Unmatched Token")
	}
	xauth := params["xauth"]
	if xauth == "" {
		return errors.New("Missing XAuth")
	}
	if xauth != s.proxy.GetXAuth() {
		return errors.New("Unmatched XAuth")
	}
	return nil
}

func (s *apiServer) Summary() (int, string) {
	sum := &Summary{
		Version: utils.Version,
		Compile: utils.Compile,
	}
	sum.Config = s.proxy.GetConfig()
	sum.Online = s.proxy.IsOnline()
	sum.Closed = s.proxy.IsClosed()

	sum.Slots = s.proxy.GetSlots()
	sum.Model = s.proxy.GetModel()

	sum.Stats = s.GetStats()
	return rpc.ApiResponseJson(sum)
}

func (s *apiServer) GetStats() *Stats {
	stats := &Stats{}
	stats.Ops.Total = router.OpsTotal()
	stats.Ops.Cmds = router.GetAllOpStats()
	stats.Sessions.Total = router.SessionsTotal()
	stats.Sessions.Actived = router.SessionsActived()
	return stats
}

func (s *apiServer) Stats(params martini.Params) (int, string) {
	if err := s.verifyToken(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(s.GetStats())
	}
}

func (s *apiServer) Start(params martini.Params) (int, string) {
	if err := s.verifyToken(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.proxy.Start(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) Shutdown(params martini.Params) (int, string) {
	if err := s.verifyToken(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.proxy.Close(); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) FillSlot(slots []*models.Slot, params martini.Params) (int, string) {
	if err := s.verifyToken(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.proxy.FillSlot(slots...); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

type ApiClient struct {
	addr  string
	token string
	xauth string
}

func NewApiClient(addr string) *ApiClient {
	return &ApiClient{addr: addr}
}

func (c *ApiClient) SetToken(token string, auth string) {
	c.token = token
	c.xauth = rpc.NewXAuth(auth, token)
}

func (c *ApiClient) encodeURL(format string, args ...interface{}) string {
	return rpc.EncodeURL(c.addr, format, args...)
}

func (c *ApiClient) Summary() (*Summary, error) {
	url := c.encodeURL("/")
	sum := &Summary{}
	if err := rpc.ApiGetJson(url, sum); err != nil {
		return nil, err
	}
	return sum, nil
}

func (c *ApiClient) Stats() (*Stats, error) {
	url := c.encodeURL("/api/stats/%s/%s", c.token, c.xauth)
	stats := &Stats{}
	if err := rpc.ApiGetJson(url, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func (c *ApiClient) Start() error {
	url := c.encodeURL("/api/start/%s/%s", c.token, c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) Shutdown() error {
	url := c.encodeURL("/api/shutdown/%s/%s", c.token, c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) FillSlot(slots ...*models.Slot) error {
	url := c.encodeURL("/api/fillslot/%s/%s", c.token, c.xauth)
	return rpc.ApiPutJson(url, slots, nil)
}
