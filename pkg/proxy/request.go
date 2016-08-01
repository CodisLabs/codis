// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"sync"

	"github.com/CodisLabs/codis/pkg/proxy/redis"
)

type Request struct {
	OpStr string
	Multi []*redis.Resp

	Start int64
	Batch *sync.WaitGroup
	Group *sync.WaitGroup
	Dirty bool

	Coalesce func() error
	Response struct {
		Resp *redis.Resp
		Err  error
	}
}
