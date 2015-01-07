// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package pipe

type PipeReader struct {
	p *pipe
}

func (r *PipeReader) Read(b []byte) (int, error) {
	return r.p.Read(b)
}

func (r *PipeReader) Close() error {
	return r.p.rclose(nil)
}

func (r *PipeReader) CloseWithError(err error) error {
	return r.p.rclose(err)
}
