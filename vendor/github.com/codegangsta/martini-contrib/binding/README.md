# binding

Request data binding for Martini.

[API Reference](http://godoc.org/github.com/codegangsta/martini-contrib/binding)



## Description

Package `binding` provides several middleware for transforming raw request data into populated structs, validating the input, and handling the errors. Each handler is independent and optional.

#### Bind

`binding.Bind` is a convenient wrapper over the other handlers in this package. It does the following boilerplate for you:

1. Deserializes the request data into a struct you supply
2. Performs validation with `binding.Validate`
3. Bails out with `binding.ErrorHandler` if there are any errors

Your application (the final handler) will not even see the request if there are any errors.

It reads the Content-Type of the request to know how to deserialize it, or if the Content-Type is not specified, it tries different deserializers until one returns without errors.

**Important safety tip:** Don't attempt to bind a pointer to a struct. This will cause a panic [to prevent a race condition](https://github.com/codegangsta/martini-contrib/pull/34#issuecomment-29683659) where every request would be pointing to the same struct.

#### Form

`binding.Form` deserializes form data from the request, whether in the query string or as a form-urlencoded payload, and puts the data into a struct you pass in. It then invokes the `binding.Validate` middleware to perform validation. No error handling is performed, but you can get the errors in your handler by receiving a `binding.Errors` type.


#### Json

`binding.Json` deserializes JSON data in the payload of the request and uses `binding.Validate` to perform validation. Similar to `binding.Form`, no error handling is performed, but you can get the errors and handle them yourself.


#### Validate

`binding.Validate` receives a populated struct and checks it for errors, first by enforcing the `binding:"required"` value on struct field tags, then by executing the `Validate()` method on the struct, if it is a `binding.Validator`. (See usage below for an example.)

*Note:* Marking a field as "required" means that you do not allow the zero value for that type (i.e. if you want to allow 0 in an int field, do not make it required).


#### ErrorHandler

`binding.ErrorHandler` is a small middleware that simply writes a `400` code to the response and also a JSON payload describing the errors, *if* any errors have been mapped to the context. It does nothing if there are no errors.



## Usage

This is a contrived example to show a few different ways to use the `binding` package.

```go
package main

import (
   "net/http"
   
   "github.com/codegangsta/martini"
   "github.com/codegangsta/martini-contrib/binding"
 )

type BlogPost struct {
	Title   string    `form:"title" json:"title" binding:"required"`
	Content string    `form:"content" json:"content"`
	Views   int       `form:"views" json:"views"`
	unexported string `form:"-"`  // skip binding of unexported fields
}

// This method implements binding.Validator and is executed by the binding.Validate middleware
func (bp BlogPost) Validate(errors *binding.Errors, req *http.Request) {
	if req.Header.Get("X-Custom-Thing") == "" {
		errors.Overall["x-custom-thing"] = "The X-Custom-Thing header is required"
	}
	if len(bp.Title) < 4 {
		errors.Fields["title"] = "Too short; minimum 4 characters"
	} else if len(bp.Title) > 120 {
		errors.Fields["title"] = "Too long; maximum 120 characters"
	}
	if bp.Views < 0 {
		errors.Fields["views"] = "Views must be at least 0"
	}
}

func main() {
	m := martini.Classic()

	m.Post("/blog", binding.Bind(BlogPost{}), func(blogpost BlogPost) string {
		// This function won't execute if there were errors
		return blogpost.Title
	})

	m.Get("/blog", binding.Form(BlogPost{}), binding.ErrorHandler, func(blogpost BlogPost) string {
		// This function won't execute if there were errors
		return blogpost.Title
	})

	m.Get("/blog", binding.Form(BlogPost{}), func(blogpost BlogPost, err binding.Errors, resp http.ResponseWriter) string {
		// This function WILL execute if there are errors because binding.Form doesn't handle errors
		if err.Count() > 0 {
			resp.WriteHeader(http.StatusBadRequest)
		}
		return blogpost.Title
	})

	m.Post("/blog", binding.Json(BlogPost{}), myOwnErrorHandler, func(blogpost BlogPost) string {
		// By this point, I assume that my own middleware took care of any errors
		return blogpost.Title
	})

	m.Run()
}
```

## Authors
* [Matthew Holt](https://github.com/mholt)
* [Michael Whatcott](https://github.com/mdwhatcott)
* [Jeremy Saenz](https://github.com/codegangsta)
