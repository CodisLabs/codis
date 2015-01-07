// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package pipe

type PipeCloser interface {
	Close() error
	CloseWithError(err error) error
}
