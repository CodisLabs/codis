# render [![wercker status](https://app.wercker.com/status/fcf6b26a1b41f53540200b1949b48dec "wercker status")](https://app.wercker.com/project/bykey/fcf6b26a1b41f53540200b1949b48dec)
Martini middleware/handler for easily rendering serialized JSON, XML, and HTML template responses.

[API Reference](http://godoc.org/github.com/martini-contrib/render)

## Usage
render uses Go's [html/template](http://golang.org/pkg/html/template/) package to render html templates.

~~~ go
// main.go
package main

import (
  "github.com/go-martini/martini"
  "github.com/martini-contrib/render"
)

func main() {
  m := martini.Classic()
  // render html templates from templates directory
  m.Use(render.Renderer())

  m.Get("/", func(r render.Render) {
    r.HTML(200, "hello", "jeremy")
  })

  m.Run()
}

~~~

~~~ html
<!-- templates/hello.tmpl -->
<h2>Hello {{.}}!</h2>
~~~

### Options
`render.Renderer` comes with a variety of configuration options:

~~~ go
// ...
m.Use(render.Renderer(render.Options{
  Directory: "templates", // Specify what path to load the templates from.
  Layout: "layout", // Specify a layout template. Layouts can call {{ yield }} to render the current template.
  Extensions: []string{".tmpl", ".html"}, // Specify extensions to load for templates.
  Funcs: []template.FuncMap{AppHelpers}, // Specify helper function maps for templates to access.
  Delims: render.Delims{"{[{", "}]}"}, // Sets delimiters to the specified strings.
  Charset: "UTF-8", // Sets encoding for json and html content-types. Default is "UTF-8".
  IndentJSON: true, // Output human readable JSON
  IndentXML: true, // Output human readable XML
  HTMLContentType: "application/xhtml+xml", // Output XHTML content type instead of default "text/html"
}))
// ...
~~~

### Loading Templates
By default the `render.Renderer` middleware will attempt to load templates with a '.tmpl' extension from the "templates" directory. Templates are found by traversing the templates directory and are named by path and basename. For instance, the following directory structure:

~~~
templates/
  |
  |__ admin/
  |      |
  |      |__ index.tmpl
  |      |
  |      |__ edit.tmpl
  |
  |__ home.tmpl
~~~

Will provide the following templates:
~~~
admin/index
admin/edit
home
~~~
### Layouts
`render.Renderer` provides a `yield` function for layouts to access:
~~~ go
// ...
m.Use(render.Renderer(render.Options{
  Layout: "layout",
}))
// ...
~~~

~~~ html
<!-- templates/layout.tmpl -->
<html>
  <head>
    <title>Martini Plz</title>
  </head>
  <body>
    <!-- Render the current template here -->
    {{ yield }}
  </body>
</html>
~~~

`current` can also be called to get the current template being rendered.
~~~ html
<!-- templates/layout.tmpl -->
<html>
  <head>
    <title>Martini Plz</title>
  </head>
  <body>
    This is the {{ current }} page.
  </body>
</html>
~~~

### Character Encodings
The `render.Renderer` middleware will automatically set the proper Content-Type header based on which function you call. See below for an example of what the default settings would output (note that UTF-8 is the default):
~~~ go
// main.go
package main

import (
  "encoding/xml"

  "github.com/go-martini/martini"
  "github.com/martini-contrib/render"
)

type Greeting struct {
  XMLName xml.Name `xml:"greeting"`
  One     string   `xml:"one,attr"`
  Two     string   `xml:"two,attr"`
}

func main() {
  m := martini.Classic()
  m.Use(render.Renderer())

  // This will set the Content-Type header to "text/html; charset=UTF-8"
  m.Get("/", func(r render.Render) {
    r.HTML(200, "hello", "world")
  })

  // This will set the Content-Type header to "application/json; charset=UTF-8"
  m.Get("/api", func(r render.Render) {
    r.JSON(200, map[string]interface{}{"hello": "world"})
  })

  // This will set the Content-Type header to "text/xml; charset=UTF-8"
  m.Get("/xml", func(r render.Render) {
    r.XML(200, Greeting{One: "hello", Two: "world"})
  })

  // This will set the Content-Type header to "text/plain; charset=UTF-8"
  m.Get("/text", func(r render.Render) {
    r.Text(200, "hello, world")
  })

  m.Run()
}

~~~

In order to change the charset, you can set the `Charset` within the `render.Options` to your encoding value:
~~~ go
// main.go
package main

import (
  "encoding/xml"

  "github.com/go-martini/martini"
  "github.com/martini-contrib/render"
)

type Greeting struct {
  XMLName xml.Name `xml:"greeting"`
  One     string   `xml:"one,attr"`
  Two     string   `xml:"two,attr"`
}

func main() {
  m := martini.Classic()
  m.Use(render.Renderer(render.Options{
    Charset: "ISO-8859-1",
  }))

  // This will set the Content-Type header to "text/html; charset=ISO-8859-1"
  m.Get("/", func(r render.Render) {
    r.HTML(200, "hello", "world")
  })

  // This will set the Content-Type header to "application/json; charset=ISO-8859-1"
  m.Get("/api", func(r render.Render) {
    r.JSON(200, map[string]interface{}{"hello": "world"})
  })

  // This will set the Content-Type header to "text/xml; charset=ISO-8859-1"
  m.Get("/xml", func(r render.Render) {
    r.XML(200, Greeting{One: "hello", Two: "world"})
  })

  // This will set the Content-Type header to "text/plain; charset=ISO-8859-1"
  m.Get("/text", func(r render.Render) {
    r.Text(200, "hello, world")
  })

  m.Run()
}

~~~

## Authors
* [Jeremy Saenz](http://github.com/codegangsta)
* [Cory Jacobsen](http://github.com/unrolled)
