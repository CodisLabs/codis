package topom

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-martini/martini"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

/*
type Stats struct {
	Online bool `json:"online"`
	Closed bool `json:"closed"`

	Intvl      int                     `json:"action_intvl"`
	Stats      map[string]*proxy.Stats `json:"stats,omitempty"`
	ServerSize map[string]float64      `json:"server_size,omitempty"`
}
*/

type apiServer struct {
	topom *Topom
	sync.RWMutex
}

func newApiServer(t *Topom) http.Handler {
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
			log.Infof("[%p] API from %s call %s", t, addr, path)
		}
		c.Next()
	})

	api := &apiServer{topom: t}

	r := martini.NewRouter()

	r.Get("/", api.Summary)
	r.Get("/api/model", api.Model)
	r.Get("/api/slots/:xauth", api.Slots)
	r.Get("/api/proxy/list/:xauth", api.ListProxy)
	r.Get("/api/group/list/:xauth", api.ListGroup)
	r.Get("/api/servers/:xauth/:msecs", api.ListServers)

	r.Put("/api/proxy/create/:xauth/:xaddr", api.CreateProxy)
	r.Put("/api/proxy/reinit/:xauth/:token", api.ReinitProxy)
	r.Put("/api/proxy/remove/:xauth/:token", api.RemoveProxy)
	r.Put("/api/proxy/force-remove/:xauth/:token", api.ForceRemoveProxy)

	r.Put("/api/group/create/:xauth/:gid", api.CreateGroup)
	r.Put("/api/group/remove/:xauth/:gid", api.RemoveGroup)

	r.Put("/api/group/add/:xauth/:gid/:xaddr", api.GroupAddServer)
	r.Put("/api/group/del/:xauth/:gid/:xaddr", api.GroupDelServer)

	r.Put("/api/group/promote/:xauth/:gid/:xaddr", api.GroupPromoteServer)
	r.Put("/api/group/promote-commit/:xauth/:gid", api.GroupPromoteCommit)

	r.Put("/api/action/create/:xauth/:sid/:gid", api.SlotCreateAction)
	r.Put("/api/action/remove/:xauth/:sid", api.SlotRemoveAction)

	r.Put("/api/shutdown/:xauth", api.Shutdown)
	r.Put("/api/set/interval/:xauth/:intvl", api.SetInterval)

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

func (s *apiServer) Summary() (int, string) {
	s.RLock()
	defer s.RUnlock()
	sum := &struct {
		Version string                `json:"version"`
		Compile string                `json:"compile"`
		Config  *Config               `json:"config,omitempty"`
		Model   *models.Topom         `json:"model,omitempty"`
		Slots   []*models.SlotMapping `json:"slots,omitempty"`
		Groups  []*models.Group       `json:"groups,omitempty"`
		Proxies []*models.Proxy       `json:"proxies,omitempty"`
	}{
		utils.Version,
		utils.Compile,
		s.topom.GetConfig(),
		s.topom.GetModel(),
		s.topom.GetSlotMappings(),
		s.topom.ListGroup(),
		s.topom.ListProxy(),
	}
	return rpc.ApiResponseJson(sum)
}

func (s *apiServer) Model() (int, string) {
	s.RLock()
	defer s.RUnlock()
	return rpc.ApiResponseJson(s.topom.GetModel())
}

func (s *apiServer) Slots(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(s.topom.GetSlotMappings())
	}
}

func (s *apiServer) ListProxy(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(s.topom.ListProxy())
	}
}

func (s *apiServer) ListGroup(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson(s.topom.ListGroup())
	}
}

type ServerInfo struct {
	GroupId int    `json:"gid"`
	Address string `json:"address"`

	Infomap map[string]string `json:"infomap,omitempty"`
	Error   error             `json:"error,omitempty"`
}

func (s *apiServer) runFetchServerInfo(addr string, timeout time.Duration) *ServerInfo {
	var ch = make(chan *ServerInfo, 1)
	go func() (m map[string]string, err error) {
		defer func() {
			ch <- &ServerInfo{Infomap: m, Error: err}
		}()
		c, err := s.topom.redisp.GetClient(addr)
		if err != nil {
			return nil, err
		}
		defer s.topom.redisp.PutClient(c)
		return c.GetInfo()
	}()

	select {
	case info := <-ch:
		return info
	case <-time.After(timeout):
		return &ServerInfo{}
	}
}

func (s *apiServer) runListServers(params martini.Params) (chan *ServerInfo, error) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return nil, err
	}
	msecs, err := s.parseInteger(params, "msecs")
	if err != nil {
		return nil, err
	}
	msecs = utils.MaxInt(msecs, 1)
	msecs = utils.MinInt(msecs, 1000)

	timeout := time.Millisecond * time.Duration(msecs)

	var ch = make(chan *ServerInfo)
	var wg sync.WaitGroup
	for _, g := range s.topom.ListGroup() {
		for _, addr := range g.Servers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				info := s.runFetchServerInfo(addr, timeout)
				info.Address = addr
				info.GroupId = g.Id
				ch <- info
			}()
		}
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch, nil
}

func (s *apiServer) ListServers(params martini.Params) (int, string) {
	ch, err := s.runListServers(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	} else {
		var infos []*ServerInfo
		for info := range ch {
			infos = append(infos, info)
		}
		return rpc.ApiResponseJson(infos)
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
	if err := s.topom.RemoveProxy(token, false); err != nil {
		return rpc.ApiResponseError(err)
	} else {
		return rpc.ApiResponseJson("OK")
	}
}

func (s *apiServer) ForceRemoveProxy(params martini.Params) (int, string) {
	s.Lock()
	defer s.Unlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	token, err := s.parseToken(params)
	if err != nil {
		return rpc.ApiResponseError(err)
	}
	if err := s.topom.RemoveProxy(token, true); err != nil {
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

func (s *apiServer) SetInterval(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	intvl, err := s.parseInteger(params, "intvl")
	if err != nil {
		return rpc.ApiResponseError(err)
	} else {
		s.topom.SetInterval(intvl)
		return rpc.ApiResponseJson("OK")
	}
}

/*
func (s *apiServer) Summary() (int, string) {
	s.RLock()
	defer s.RUnlock()
	sum := &struct {
		Version   string                `json:"version"`
		Compile   string                `json:"compile"`
		Config    *Config               `json:"config,omitempty"`
		Model     *models.Topom         `json:"model,omitempty"`
		Slots     []*models.SlotMapping `json:"slots,omitempty"`
		GroupList []*models.Group       `json:"group_list,omitempty"`
		ProxyList []*models.Proxy       `json:"proxy_list,omitempty"`
	}{
		utils.Version,
		utils.Compile,
		s.topom.GetConfig(),
		s.topom.GetModel(),
		s.topom.GetSlotMappings(),
		s.topom.ListGroup(),
		s.topom.ListProxy(),
	}
	return rpc.ApiResponseJson(sum)
}

func (s *apiServer) Stats(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	panic("todo")
}

func (s *apiServer) SetActionIntvl(params martini.Params) (int, string) {
	s.RLock()
	defer s.RUnlock()
	if err := s.verifyXAuth(params); err != nil {
		return rpc.ApiResponseError(err)
	}
	text := params["intvl"]
	if text == "" {
		return rpc.ApiResponseError(errors.New("missing intvl"))
	}
	if v, err := strconv.Atoi(text); err != nil {
		return rpc.ApiResponseError(errors.New("invalid intvl"))
	} else {
		s.topom.SetInterval(v)
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
*/
