// Package render is a middleware for Martini that provides easy JSON serialization and HTML template rendering.
//
//  package main
//
//  import (
//    "github.com/codegangsta/martini"
//    "github.com/codegangsta/martini-contrib/render"
//  )
//
//  func main() {
//    m := martini.Classic()
//    m.Use(render.Renderer()) // reads "templates" directory by default
//
//    m.Get("/html", func(r render.Render) {
//      r.HTML(200, "mytemplate", nil)
//    })
//
//    m.Get("/json", func(r render.Render) {
//      r.JSON(200, "hello world")
//    })
//
//    m.Run()
//  }
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/codegangsta/martini"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

const (
	ContentType    = "Content-Type"
	ContentLength  = "Content-Length"
	ContentJSON    = "application/json"
	ContentHTML    = "text/html"
	defaultCharset = "UTF-8"
)

// Included helper functions for use when rendering html
var helperFuncs = template.FuncMap{
	"yield": func() (string, error) {
		return "", fmt.Errorf("yield called with no layout defined")
	},
}

// Render is a service that can be injected into a Martini handler. Render provides functions for easily writing JSON and
// HTML templates out to a http Response.
type Render interface {
	// JSON writes the given status and JSON serialized version of the given value to the http.ResponseWriter.
	JSON(status int, v interface{})
	// HTML renders a html template specified by the name and writes the result and given status to the http.ResponseWriter.
	HTML(status int, name string, v interface{}, htmlOpt ...HTMLOptions)
	// Error is a convenience function that writes an http status to the http.ResponseWriter.
	Error(status int)
	// Redirect is a convienience function that sends an HTTP redirect. If status is omitted, uses 302 (Found)
	Redirect(location string, status ...int)
	// Template returns the internal *template.Template used to render the HTML
	Template() *template.Template
}

// Delims represents a set of Left and Right delimiters for HTML template rendering
type Delims struct {
	// Left delimiter, defaults to {{
	Left string
	// Right delimiter, defaults to }}
	Right string
}

// Options is a struct for specifying configuration options for the render.Renderer middleware
type Options struct {
	// Directory to load templates. Default is "templates"
	Directory string
	// Layout template name. Will not render a layout if "". Defaults to "".
	Layout string
	// Extensions to parse template files from. Defaults to [".tmpl"]
	Extensions []string
	// Funcs is a slice of FuncMaps to apply to the template upon compilation. This is useful for helper functions. Defaults to [].
	Funcs []template.FuncMap
	// Delims sets the action delimiters to the specified strings in the Delims struct.
	Delims Delims
	// Appends the given charset to the Content-Type header. Default is "UTF-8".
	Charset string
	// Outputs human readable JSON
	IndentJSON bool
}

// HTMLOptions is a struct for overriding some rendering Options for specific HTML call
type HTMLOptions struct {
	// Layout template name. Overrides Options.Layout.
	Layout string
}

// Renderer is a Middleware that maps a render.Render service into the Martini handler chain. An single variadic render.Options
// struct can be optionally provided to configure HTML rendering. The default directory for templates is "templates" and the default
// file extension is ".tmpl".
//
// If MARTINI_ENV is set to "" or "development" then templates will be recompiled on every request. For more performance, set the
// MARTINI_ENV environment variable to "production"
func Renderer(options ...Options) martini.Handler {
	opt := prepareOptions(options)
	cs := prepareCharset(opt.Charset)
	t := compile(opt)
	return func(res http.ResponseWriter, req *http.Request, c martini.Context) {
		// recompile for easy development
		if martini.Env == martini.Dev {
			t = compile(opt)
		}
		tc, _ := t.Clone()
		c.MapTo(&renderer{res, req, tc, opt, cs}, (*Render)(nil))
	}
}

func prepareCharset(charset string) string {
	if len(charset) != 0 {
		return "; charset=" + charset
	}

	return "; charset=" + defaultCharset
}

func prepareOptions(options []Options) Options {
	var opt Options
	if len(options) > 0 {
		opt = options[0]
	}

	// Defaults
	if len(opt.Directory) == 0 {
		opt.Directory = "templates"
	}
	if len(opt.Extensions) == 0 {
		opt.Extensions = []string{".tmpl"}
	}

	return opt
}

func compile(options Options) *template.Template {
	dir := options.Directory
	t := template.New(dir)
	t.Delims(options.Delims.Left, options.Delims.Right)
	// parse an initial template in case we don't have any
	template.Must(t.Parse("Martini"))

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		r, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		ext := filepath.Ext(r)
		for _, extension := range options.Extensions {
			if ext == extension {

				buf, err := ioutil.ReadFile(path)
				if err != nil {
					panic(err)
				}

				name := (r[0 : len(r)-len(ext)])
				tmpl := t.New(filepath.ToSlash(name))

				// add our funcmaps
				for _, funcs := range options.Funcs {
					tmpl.Funcs(funcs)
				}

				// Bomb out if parse fails. We don't want any silent server starts.
				template.Must(tmpl.Funcs(helperFuncs).Parse(string(buf)))
				break
			}
		}

		return nil
	})

	return t
}

type renderer struct {
	http.ResponseWriter
	req             *http.Request
	t               *template.Template
	opt             Options
	compiledCharset string
}

func (r *renderer) JSON(status int, v interface{}) {
	var result []byte
	var err error
	if r.opt.IndentJSON {
		result, err = json.MarshalIndent(v, "", "  ")
	} else {
		result, err = json.Marshal(v)
	}
	if err != nil {
		http.Error(r, err.Error(), 500)
		return
	}

	// json rendered fine, write out the result
	r.Header().Set(ContentType, ContentJSON+r.compiledCharset)
	r.WriteHeader(status)
	r.Write(result)
}

func (r *renderer) HTML(status int, name string, binding interface{}, htmlOpt ...HTMLOptions) {
	opt := r.prepareHTMLOptions(htmlOpt)
	// assign a layout if there is one
	if len(opt.Layout) > 0 {
		r.addYield(name, binding)
		name = opt.Layout
	}

	out, err := r.execute(name, binding)
	if err != nil {
		http.Error(r, err.Error(), http.StatusInternalServerError)
		return
	}

	// template rendered fine, write out the result
	r.Header().Set(ContentType, ContentHTML+r.compiledCharset)
	r.WriteHeader(status)
	io.Copy(r, out)
}

// Error writes the given HTTP status to the current ResponseWriter
func (r *renderer) Error(status int) {
	r.WriteHeader(status)
}

func (r *renderer) Redirect(location string, status ...int) {
	code := http.StatusFound
	if len(status) == 1 {
		code = status[0]
	}

	http.Redirect(r, r.req, location, code)
}

func (r *renderer) Template() *template.Template {
	return r.t
}

func (r *renderer) execute(name string, binding interface{}) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	return buf, r.t.ExecuteTemplate(buf, name, binding)
}

func (r *renderer) addYield(name string, binding interface{}) {
	funcs := template.FuncMap{
		"yield": func() (template.HTML, error) {
			buf, err := r.execute(name, binding)
			// return safe html here since we are rendering our own template
			return template.HTML(buf.String()), err
		},
	}
	r.t.Funcs(funcs)
}

func (r *renderer) prepareHTMLOptions(htmlOpt []HTMLOptions) HTMLOptions {
	if len(htmlOpt) > 0 {
		return htmlOpt[0]
	}

	return HTMLOptions{
		Layout: r.opt.Layout,
	}
}
