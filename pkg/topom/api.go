package topom

import (
	"net/http"
	"strings"

	"github.com/go-martini/martini"

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
}

type apiServer struct {
	topom *Topom
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

	api := &apiServer{t}
	// TODO
	_ = api

	r := martini.NewRouter()

	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	return m
}

func (s *apiServer) verifyXAuth(params martini.Params) error {
	xauth := params["xauth"]
	if xauth == "" {
		return errors.New("Missing XAuth")
	}
	if xauth != s.topom.GetXAuth() {
		return errors.New("Unmatched XAuth")
	}
	return nil
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
