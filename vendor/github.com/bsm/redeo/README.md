# Redeo [![Build Status](https://travis-ci.org/bsm/redeo.png?branch=master)](https://travis-ci.org/bsm/redeo)

High-performance framework for building redis-protocol compatible TCP
servers/services. Optimised for speed!

### Example

```go
package main

import (
  "github.com/bsm/redeo"
  "log"
)

func main() {
  srv := redeo.NewServer(&redeo.Config{Addr: "localhost:9736"})
  srv.HandleFunc("ping", func(out *redeo.Responder, _ *redeo.Request) error {
    out.WriteInlineString("PONG")
    return nil
  })

  log.Printf("Listening on tcp://%s", srv.Addr())
  log.Fatal(srv.ListenAndServe())
}
```

### Licence

```
Copyright (c) 2014 Black Square Media

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```
