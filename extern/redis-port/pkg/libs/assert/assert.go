// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package assert

import "github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"

func Must(b bool) {
	if b {
		return
	}
	log.Panic("assertion failed")
}

func ErrorIsNil(err error) {
	if err == nil {
		return
	}
	log.PanicError(err, "error happens, assertion failed")
}
