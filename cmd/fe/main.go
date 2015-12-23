package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

func main() {
	m := martini.New()
	m.Use(martini.Recovery())
	m.Use(render.Renderer())
	m.Use(func(w http.ResponseWriter, req *http.Request, c martini.Context) {
		fmt.Println(req.URL.Path)
		c.Next()
	})

	abspath, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}

	m.Use(martini.Static(filepath.Join(abspath, "assets"), martini.StaticOptions{SkipLogging: true}))
	/*
		m.Use(render.Renderer(render.Options{
			Directory:  filepath.Join(abspath, "assets/template"),
			Extensions: []string{".tmpl", ".html"},
			Charset:    "UTF-8",
			IndentJSON: true,
		}))
	*/

	r := martini.NewRouter()
	r.Get("/list", func() (int, string) {
		var m []interface{}
		m = append(m, map[string]interface{}{
			"name":      "codis-test",
			"dashboard": "100.64.8.119:18080",
		})
		for i := 0; i < 100; i++ {
			m = append(m, map[string]interface{}{
				"name":      fmt.Sprintf("codis-test%d", i),
				"dashboard": fmt.Sprintf("100.64.8.119:%d", 48080+i),
			})
		}
		return rpc.ApiResponseJson(m)
	})

	var proxy = httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: "100.64.8.119:18080"})
	r.Any("/**", func(w http.ResponseWriter, req *http.Request) {
		forward := req.URL.Query().Get("forward")
		if forward == "codis-test" {
			fmt.Println("forward :", req.URL)
			proxy.ServeHTTP(w, req)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
	})

	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)

	l, err := net.Listen("tcp", "0.0.0.0:8080")
	if err != nil {
		panic(err)
	}
	h := http.NewServeMux()
	h.Handle("/", m)
	hs := &http.Server{Handler: h}
	hs.Serve(l)
}
