package router

import (
	"encoding/json"
	"sync"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
)

type Request struct {
	Sid   int64
	SeqId int64
	OpStr string
	Flush bool

	Resp *redis.Resp

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
		SeqId  int64
		OpStr  string
		Flush  bool
		SlotId int
	}{
		r.Sid, r.SeqId, r.OpStr,
		r.Flush, r.slot.Id,
	}
	b, _ := json.Marshal(o)
	return string(b)
}

func (r *Request) SetResponse(resp *redis.Resp, err error) error {
	r.Response.Resp, r.Response.Err = resp, err
	if r.slot != nil {
		r.slot.jobs.Done()
	}
	if r.wait != nil {
		r.wait.Done()
	}
	return err
}

func (r *Request) Wait() {
	if r.wait != nil {
		r.wait.Wait()
	}
}
