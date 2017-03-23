# gzip [![wercker status](https://app.wercker.com/status/186d65e4d8160cf274ffc5835e6d9795 "wercker status")](https://app.wercker.com/project/bykey/186d65e4d8160cf274ffc5835e6d9795)
Gzip middleware for Martini.

[API Reference](http://godoc.org/github.com/martini-contrib/gzip)

## Usage

~~~ go
import (
  "github.com/go-martini/martini"
  "github.com/martini-contrib/gzip"
)

func main() {
  m := martini.Classic()
  // gzip every request
  m.Use(gzip.All())
  m.Run()
}

~~~

Make sure to include the Gzip middleware above other middleware that alter the response body (like the render middleware).

## Changing compression level

You can set compression level using gzip.Options:

~~~ go
import (
  "github.com/go-martini/martini"
  "github.com/martini-contrib/gzip"
)

func main() {
  m := martini.Classic()
  // gzip every request with maximum compression level
  m.Use(gzip.All(gzip.Options{
    CompressionLevel: gzip.BestCompression,
  }))
  m.Run()
}
~~~

The compression level can be DefaultCompression or any integer value between BestSpeed and BestCompression inclusive.

## Authors
* [Jeremy Saenz](http://github.com/codegangsta)
* [Shane Logsdon](http://github.com/slogsdon)
