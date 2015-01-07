// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package pipe

type PipeWriter struct {
	p *pipe
}

func (w *PipeWriter) Write(b []byte) (int, error) {
	return w.p.Write(b)
}

func (w *PipeWriter) Close() error {
	return w.p.wclose(nil)
}

func (w *PipeWriter) CloseWithError(err error) error {
	return w.p.wclose(err)
}
