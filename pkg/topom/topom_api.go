package topom

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type Stats struct {
	Online bool `json:"online"`
	Closed bool `json:"closed"`

	GroupList []*models.Group `json:"group_list"`
	ProxyList []*models.Proxy `json:"proxy_list"`

	Slots []*models.SlotMapping `json:"slots"`
	Stats struct {
		Servers map[string]*ServerStats `json:"servers"`
		Proxies map[string]*ProxyStats  `json:"proxies"`
	} `json:"stats"`

	Action struct {
		Interval int  `json:"interval"`
		Disabled bool `json:"disabled"`
	} `json:"action"`
}

type apiServer struct {
	topom *Topom
	sync.RWMutex
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
		if req.Method != "GET" && strings.HasPrefix(path, "/api") {
			log.Infof("[%p] API from %s call %s", t, addr, path)
		}
		c.Next()
	})

	api := &apiServer{topom: t}

	r := martini.NewRouter()
	r.Get("/", func(r render.Render) {
		r.Redirect("/overview")
	})

	r.Get("/overview", api.Overview)
	r.Get("/api/model", api.Model)
	r.Get("/api/xping/:xauth", api.XPing)
	r.Get("/api/stats/:xauth", api.Stats)

	r.Put("/api/proxy/create/:xauth/:xaddr", api.CreateProxy)
	r.Put("/api/proxy/reinit/:xauth/:token", api.ReinitProxy)
	r.Put("/api/proxy/remove/:xauth/:token/:force", api.RemoveProxy)

	r.Put("/api/group/create/:xauth/:gid", api.CreateGroup)
	r.Put("/api/group/remove/:xauth/:gid", api.RemoveGroup)

	r.Put("/api/group/add/:xauth/:gid/:xaddr", api.GroupAddServer)
	r.Put("/api/group/del/:xauth/:gid/:xaddr", api.GroupDelServer)

	r.Put("/api/group/promote/:xauth/:gid/:xaddr", api.GroupPromoteServer)
	r.Put("/api/group/promote-commit/:xauth/:gid", api.GroupPromoteCommit)

	r.Put("/api/action/create/:xauth/:sid/:gid", api.SlotCreateAction)
	r.Put("/api/action/remove/:xauth/:sid", api.SlotRemoveAction)

	r.Put("/api/shutdown/:xauth", api.Shutdown)

	r.Put("/api/set/action/interval/:xauth/:value", api.SetActionInterval)
	r.Put("/api/set/action/disabled/:xauth/:value", api.SetActionDisabled)

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

func (s *apiServer) Overview() (int, string) {
	s.RLock()
	defer s.RUnlock()
	overview := &struct {
		Version string        `json:"version"`
		Compile string        `json:"compile"`
		Config  *Config       `json:"config"`
		Model   *models.Topom `json:"model"`
		Stats   *Stats        `json:"stats"`
	}{
		utils.Version,
		utils.Compile,
		s.topom.GetConfig(),
		s.topom.GetModel(),
		s.newStats(),
	}
	return rpc.ApiResponseJson(overview)
}

func (s *apiServer) newStats() *Stats {
	stats := &Stats{}

	stats.Online = s.topom.IsOnline()
	stats.Closed = s.topom.IsClosed()

	stats.GroupList = s.topom.ListGroup()
	stats.ProxyList = s.topom.ListProxy()

	stats.Action.Interval = s.topom.GetActionInterval()
	stats.Action.Disabled = s.topom.GetActionDisabled()

	stats.Slots = s.topom.GetSlotMappings()
	stats.Stats.Servers = make(map[string]*ServerStats)
	for _, g := range stats.GroupList {
		for _, addr := range g.Servers {
			stats.Stats.Servers[addr] = s.topom.GetServerStats(addr)
		}
	}

	stats.Stats.Proxies = make(map[string]*ProxyStats)
	for _, p := range stats.ProxyList {
		stats.Stats.Proxies[p.Token] = s.topom.GetProxyStats(p.Token)
	}
	return stats
}

func (s *apiServer) Model() (int, string) {
	s.RLock()
	defer s.RUnlock()
	return rpc.ApiResponseJson(s.topom.GetModel())
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

func (s *apiServer) Stats(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(s.newStats())
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

func (s *apiServer) parseBoolean(params martini.Params, entry string) (bool, error) {
	text := params[entry]
	if text == "" {
		return false, fmt.Errorf("missing %s", entry)
	}
	v, err := strconv.ParseBool(text)
	if err != nil {
		return false, fmt.Errorf("invalid %s", entry)
	}
	return v, nil
}

func (s *apiServer) CreateProxy(params martini.Params) (int, string) {
	s.Lock()
	defer s.Unlock()
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
	s.RLock()
	defer s.RUnlock()
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
	s.Lock()
	defer s.Unlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	token, err := s.parseToken(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	force, err := s.parseBoolean(params, "force")
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.RemoveProxy(token, force); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) CreateGroup(params martini.Params) (int, string) {
	s.Lock()
	defer s.Unlock()
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
	s.Lock()
	defer s.Unlock()
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
	s.Lock()
	defer s.Unlock()
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
	s.Lock()
	defer s.Unlock()
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

func (s *apiServer) GroupPromoteServer(params martini.Params) (int, string) {
	s.Lock()
	defer s.Unlock()
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
	s.Lock()
	defer s.Unlock()
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

func (s *apiServer) SlotCreateAction(params martini.Params) (int, string) {
	s.Lock()
	defer s.Unlock()
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

func (s *apiServer) SlotRemoveAction(params martini.Params) (int, string) {
	s.Lock()
	defer s.Unlock()
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
	s.Lock()
	defer s.Unlock()
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
	s.RLock()
	defer s.RUnlock()
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
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	value, err := s.parseBoolean(params, "value")
	if err != nil {
		return rpc.ApiResponseError(err)
	} else {
		s.topom.SetActionDisabled(value)
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

func (c *ApiClient) Model() (*models.Topom, error) {
	url := c.encodeURL("/api/model")
	model := &models.Topom{}
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

func (c *ApiClient) CreateProxy(addr string) error {
	url := c.encodeURL("/api/proxy/create/%s/%s", c.xauth, c.encodeXAddr(addr))
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) ReinitProxy(token string) error {
	url := c.encodeURL("/api/proxy/reinit/%s/%s", c.xauth, token)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) RemoveProxy(token string, force bool) error {
	url := c.encodeURL("/api/proxy/remove/%s/%s/%t", c.xauth, token, force)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) CreateGroup(groupId int) error {
	url := c.encodeURL("/api/group/create/%s/%d", c.xauth, groupId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) RemoveGroup(groupId int) error {
	url := c.encodeURL("/api/group/remove/%s/%d", c.xauth, groupId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupAddServer(groupId int, addr string) error {
	url := c.encodeURL("/api/group/add/%s/%d/%s", c.xauth, groupId, c.encodeXAddr(addr))
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupDelServer(groupId int, addr string) error {
	url := c.encodeURL("/api/group/del/%s/%d/%s", c.xauth, groupId, c.encodeXAddr(addr))
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupPromoteServer(groupId int, addr string) error {
	url := c.encodeURL("/api/group/promote/%s/%d/%s", c.xauth, groupId, c.encodeXAddr(addr))
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) GroupPromoteCommit(groupId int) error {
	url := c.encodeURL("/api/group/promote-commit/%s/%d", c.xauth, groupId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotCreateAction(slotId int, groupId int) error {
	url := c.encodeURL("/api/action/create/%s/%d/%d", c.xauth, slotId, groupId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SlotRemoveAction(slotId int) error {
	url := c.encodeURL("/api/action/remove/%s/%d", c.xauth, slotId)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) Shutdown() error {
	url := c.encodeURL("/api/shutdown/%s", c.xauth)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SetActionInterval(value int) error {
	url := c.encodeURL("/api/set/action/interval/%s/%d", c.xauth, value)
	return rpc.ApiPutJson(url, nil, nil)
}

func (c *ApiClient) SetActionDisabled(value bool) error {
	url := c.encodeURL("/api/set/action/disabled/%s/%t", c.xauth, value)
	return rpc.ApiPutJson(url, nil, nil)
}
