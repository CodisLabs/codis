// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"encoding/json"
	"sync"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
)

type Dispatcher interface {
	Dispatch(r *Request) error
}

type Request struct {
	Sid   int64
	Seq   int64
	OpStr string
	Start int64
	Flush bool

	Wait *sync.WaitGroup
	Resp *redis.Resp

	Coalesce func() error
	Response struct {
		Resp *redis.Resp
		Err  error
	}

	slot *Slot
}

func (r *Request) String() string {
	o := &struct {
		Sid   int64  `json:"sid"`
		Seq   int64  `json:"seq"`
		OpStr string `json:"opstr"`
		Start int64  `json:"start"`
	}{
		r.Sid, r.Seq, r.OpStr, r.Start,
	}
	b, _ := json.Marshal(o)
	return string(b)
}
