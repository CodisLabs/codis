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

	Resp *redis.Resp

	Callback func() error
	Response struct {
		Resp *redis.Resp
		Err  error
	}

	slot *Slot
	wait *sync.WaitGroup
}

func (r *Request) String() string {
	o := &struct {
		Sid    int64
		Seq    int64
		OpStr  string
		SlotId int
	}{
		r.Sid, r.Seq, r.OpStr, -1,
	}
	if r.slot != nil {
		o.SlotId = r.slot.Id
	}
	b, _ := json.Marshal(o)
	return string(b)
}
