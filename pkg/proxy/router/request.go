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
	Owner *Session
	OpSeq int64
	OpStr string
	Start int64

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
		OpSeq int64  `json:"opseq"`
		OpStr string `json:"opstr"`
		Start int64  `json:"start"`
	}{
		0, r.OpSeq, r.OpStr, r.Start,
	}
	if r.Owner != nil {
		o.Sid = r.Owner.Sid
	}
	b, _ := json.Marshal(o)
	return string(b)
}
