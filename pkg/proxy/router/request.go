// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

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

	Coalesce func() error
	Response struct {
		Resp *redis.Resp
		Err  error
	}
}
