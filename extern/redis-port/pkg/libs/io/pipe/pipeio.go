// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package pipe

import "io"

type Closer interface {
	io.Closer
	CloseWithError(err error) error
}

type Reader interface {
	io.Reader
	Closer
	Buffered() (int, error)
}

type reader struct {
	p *pipe
}

func (r *reader) Read(b []byte) (int, error) {
	return r.p.Read(b)
}

func (r *reader) Close() error {
	return r.p.RClose(nil)
}

func (r *reader) CloseWithError(err error) error {
	return r.p.RClose(err)
}

func (r *reader) Buffered() (int, error) {
	return r.p.Buffered()
}

type Writer interface {
	io.Writer
	Closer
	Available() (int, error)
}

type writer struct {
	p *pipe
}

func (w *writer) Write(b []byte) (int, error) {
	return w.p.Write(b)
}

func (w *writer) Close() error {
	return w.p.WClose(nil)
}

func (w *writer) CloseWithError(err error) error {
	return w.p.WClose(err)
}

func (w *writer) Available() (int, error) {
	return w.p.Available()
}
