// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package ioutils

import (
	"io"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

func ReadFull(r io.Reader, p []byte) (int, error) {
	n := 0
	for len(p) != 0 {
		i, err := r.Read(p)
		n, p = n+i, p[i:]
		if err != nil {
			return n, errors.Trace(err)
		}
	}
	return n, nil
}

func WriteFull(w io.Writer, p []byte) (int, error) {
	n := 0
	for len(p) != 0 {
		i, err := w.Write(p)
		n, p = n+i, p[i:]
		if err != nil {
			return n, errors.Trace(err)
		}
	}
	return n, nil
}
