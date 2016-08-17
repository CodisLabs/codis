# cors [![wercker status](https://app.wercker.com/status/4d44f8169ae6c51d0f2d6ffe523bd72e "wercker status")](https://app.wercker.com/project/bykey/4d44f8169ae6c51d0f2d6ffe523bd72e)

Martini middleware/handler to enable CORS support.

## Usage

~~~ go
import (
  "github.com/go-martini/martini"
  "github.com/martini-contrib/cors"
)

func main() {
  m := martini.Classic()
  // CORS for https://foo.* origins, allowing:
  // - PUT and PATCH methods
  // - Origin header
  // - Credentials share
  m.Use(cors.Allow(&cors.Options{
    AllowOrigins:     []string{"https://*.foo.com"},
    AllowMethods:     []string{"PUT", "PATCH"},
    AllowHeaders:     []string{"Origin"},
    ExposeHeaders:    []string{"Content-Length"},
    AllowCredentials: true,
  }))
  m.Run()
}
~~~

You may alternatively prefer to allow CORS only for certain routes. Instead of using the CORS middleware app-wide, register it for the prefered routes. The following snippet demonstrates how to enable CORS for `/api/books` endpoint's PUT handler.

~~~ go
m := martini.Classic()
allowCORSHandler := cors.Allow(&cors.Options{
  AllowOrigins:     []string{"https://*.foo.com"},
  AllowMethods:     []string{"PUT", "PATCH"},
  AllowHeaders:     []string{"Origin"},
})

m.Put("/api/books", allowCORSHandler, func() string {
  // ...
})
~~~

## Authors

* [Burcu Dogan](http://github.com/rakyll)
