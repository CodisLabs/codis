package gzip

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/go-martini/martini"
)

const (
	HeaderAcceptEncoding  = "Accept-Encoding"
	HeaderContentEncoding = "Content-Encoding"
	HeaderContentLength   = "Content-Length"
	HeaderContentType     = "Content-Type"
	HeaderVary            = "Vary"
	BestSpeed             = gzip.BestSpeed
	BestCompression       = gzip.BestCompression
	DefaultCompression    = gzip.DefaultCompression
)

var serveGzip = func(w http.ResponseWriter, r *http.Request, c martini.Context, options Options) {
	if !strings.Contains(r.Header.Get(HeaderAcceptEncoding), "gzip") {
		return
	}

	headers := w.Header()
	headers.Set(HeaderContentEncoding, "gzip")
	headers.Set(HeaderVary, HeaderAcceptEncoding)

	gz := gzip.NewWriter(w)
	defer gz.Close()

	gzw := gzipResponseWriter{gz, w.(martini.ResponseWriter)}
	c.MapTo(gzw, (*http.ResponseWriter)(nil))

	c.Next()

	// delete content length after we know we have been written to
	gzw.Header().Del("Content-Length")
}

// Options is a struct for specifying configuration options.
type Options struct {
	// Compression level. Can be DefaultCompression or any integer value between BestSpeed and BestCompression inclusive.
	CompressionLevel int
}

// All returns a Handler that adds gzip compression to all requests
func All(options ...Options) martini.Handler {
	opt := prepareOptions(options)
	return func(w http.ResponseWriter, r *http.Request, c martini.Context) {
		serveGzip(w, r, c, opt)
	}
}

func prepareOptions(options []Options) Options {
	var opt Options
        if len(options) > 0 {
                opt = options[0]
        }
        if !isCompressionLevelValid(opt.CompressionLevel) {
                opt.CompressionLevel = DefaultCompression
        }
	return opt
}

func isCompressionLevelValid(level int) bool {
	switch {
	case level == DefaultCompression:
		return true
	case BestSpeed <= level && level <= BestCompression:
		return true
	default:
		return false
	}
}

type gzipResponseWriter struct {
	w *gzip.Writer
	martini.ResponseWriter
}

func (grw gzipResponseWriter) Write(p []byte) (int, error) {
	if len(grw.Header().Get(HeaderContentType)) == 0 {
		grw.Header().Set(HeaderContentType, http.DetectContentType(p))
	}

	return grw.w.Write(p)
}

func (grw gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := grw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("the ResponseWriter doesn't support the Hijacker interface")
	}
	return hijacker.Hijack()
}
