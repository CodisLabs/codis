package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
	"github.com/wandoulabs/codis/pkg/utils/sync2/atomic2"
)

var roundTripper http.RoundTripper

func init() {
	var dials atomic2.Int64
	tr := &http.Transport{}
	tr.Dial = func(network, addr string) (net.Conn, error) {
		c, err := net.DialTimeout(network, addr, time.Second*10)
		if err == nil {
			log.Debugf("rpc: dial new connection to [%d] %s - %s",
				dials.Incr()-1, network, addr)
		}
		return c, err
	}
	go func() {
		for {
			time.Sleep(time.Minute)
			tr.CloseIdleConnections()
		}
	}()
	roundTripper = tr
}

func main() {
	const usage = `
Usage:
	codis-fe [--ncpu=N] [--codis-map=CONF] [--log=FILE] [--log-level=LEVEL] [--listen=ADDR]
	codis-fe  --version

Options:
	--ncpu=N                    set runtime.GOMAXPROCS to N, default is runtime.NumCPU().
	-c CONF, --codis-map=CONF   set the config file, default is codis.json.
	-l FILE, --log=FILE         set path/name of daliy rotated log file.
	--log-level=LEVEL           set the log-level, should be INFO,WARN,DEBUG or ERROR, default is INFO.
	--listen=ADDR               set the listen address, default is 0.0.0.0:8080
`
	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}

	if d["--version"].(bool) {
		fmt.Println("version:", utils.Version)
		fmt.Println("compile:", utils.Compile)
		return
	}

	if s, ok := utils.Argument(d, "--log"); ok {
		w, err := log.NewRollingFile(s, log.DailyRolling)
		if err != nil {
			log.PanicErrorf(err, "open log file %s failed", s)
		} else {
			log.StdLog = log.New(w, "")
		}
	}
	log.SetLevel(log.LevelInfo)

	if s, ok := utils.Argument(d, "--log-level"); ok {
		if !log.SetLevelString(s) {
			log.Panicf("option --log-level = %s", s)
		}
	}

	if n, ok := utils.ArgumentInteger(d, "--ncpu"); ok {
		runtime.GOMAXPROCS(n)
	} else {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	log.Warnf("set ncpu = %d", runtime.GOMAXPROCS(0))

	var listen = "0.0.0.0:8080"
	if s, ok := utils.Argument(d, "--listen"); ok {
		listen = s
		log.Warnf("option --listen = %s", s)
	}

	var config = "codis.json"
	if s, ok := utils.Argument(d, "--codis-map"); ok {
		listen = s
		log.Warnf("option --codis-map = %s", s)
	}

	loader := &ConfigLoader{path: config}
	router := &ReverseProxy{}

	go func() {
		for {
			m, err := loader.Reload()
			if err != nil {
				log.WarnErrorf(err, "reload %s failed", config)
				time.Sleep(time.Second * 5)
			} else {
				if m != nil {
					log.Infof("reload %s = %v", config, m)
					router.Update(m)
				}
				time.Sleep(time.Second)
			}
		}
	}()

	m := martini.New()
	m.Use(martini.Recovery())
	m.Use(render.Renderer())

	binpath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.PanicErrorf(err, "get path of binary failed")
	}
	m.Use(martini.Static(filepath.Join(binpath, "assets"), martini.StaticOptions{SkipLogging: true}))

	r := martini.NewRouter()
	r.Get("/list", func() (int, string) {
		return rpc.ApiResponseJson(router.Names())
	})

	r.Any("/**", func(w http.ResponseWriter, req *http.Request) {
		name := req.URL.Query().Get("forward")
		if p := router.GetProxy(name); p != nil {
			p.ServeHTTP(w, req)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
	})

	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)

	l, err := net.Listen("tcp", listen)
	if err != nil {
		log.PanicErrorf(err, "listen %s failed", listen)
	}
	defer l.Close()

	h := http.NewServeMux()
	h.Handle("/", m)
	hs := &http.Server{Handler: h}
	if err := hs.Serve(l); err != nil {
		log.PanicErrorf(err, "serve %s failed", listen)
	}
}

type ConfigLoader struct {
	last time.Time
	path string
}

func (l *ConfigLoader) Reload() (map[string]string, error) {
	if fi, err := os.Stat(l.path); err != nil || fi.ModTime().Equal(l.last) {
		return nil, errors.Trace(err)
	} else {
		m, err := l.Load()
		if err != nil {
			return nil, err
		}
		l.last = fi.ModTime()
		return m, nil
	}
}

func (l *ConfigLoader) Load() (map[string]string, error) {
	b, err := ioutil.ReadFile(l.path)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var list []*struct {
		Name      string `json:"name"`
		Dashboard string `json:"dashboard"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, errors.Trace(err)
	}
	var m = make(map[string]string)
	for _, e := range list {
		m[e.Name] = e.Dashboard
	}
	return m, nil
}

type ReverseProxy struct {
	sync.Mutex
	routes map[string]*httputil.ReverseProxy
}

func (r *ReverseProxy) Update(routes map[string]string) {
	r.Lock()
	defer r.Unlock()
	r.routes = make(map[string]*httputil.ReverseProxy)
	for name, host := range routes {
		if name == "" || host == "" {
			continue
		}
		u := &url.URL{Scheme: "http", Host: host}
		p := httputil.NewSingleHostReverseProxy(u)
		p.Transport = roundTripper
		r.routes[name] = p
	}
}

func (r *ReverseProxy) GetProxy(name string) *httputil.ReverseProxy {
	r.Lock()
	defer r.Unlock()
	if r.routes == nil {
		return nil
	}
	return r.routes[name]
}

func (r *ReverseProxy) Names() []string {
	r.Lock()
	defer r.Unlock()
	var names []string
	if r.routes != nil {
		for name, _ := range r.routes {
			names = append(names, name)
		}
	}
	return names
}
