// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package ioutils

import (
	"io"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

type simpleReader struct {
	r   io.Reader
	err error
}

func (r *simpleReader) Read(b []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.r.Read(b)
	if err != nil {
		r.err = errors.Trace(err)
	}
	return n, r.err
}

func SimpleReader(r io.Reader) io.Reader {
	return &simpleReader{r: r}
}

type simpleWriter struct {
	w   io.Writer
	err error
}

func (w *simpleWriter) Write(b []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.w.Write(b)
	if err != nil {
		w.err = errors.Trace(err)
	}
	return n, w.err
}

func SimpleWriter(w io.Writer) io.Writer {
	return &simpleWriter{w: w}
}

type simpleReadCloser struct {
	r io.Reader
	c io.Closer
}

func (r *simpleReadCloser) Read(b []byte) (int, error) {
	return r.r.Read(b)
}

func (r *simpleReadCloser) Close() error {
	return r.c.Close()
}

func SimpleReadCloser(r io.ReadCloser) io.ReadCloser {
	return &simpleReadCloser{r: SimpleReader(r), c: r}
}

type simpleWriteCloser struct {
	w io.Writer
	c io.Closer
}

func (w *simpleWriteCloser) Write(b []byte) (int, error) {
	return w.w.Write(b)
}

func (w *simpleWriteCloser) Close() error {
	return w.c.Close()
}

func SimpleWriteCloser(w io.WriteCloser) io.WriteCloser {
	return &simpleWriteCloser{w: SimpleWriter(w), c: w}
}
