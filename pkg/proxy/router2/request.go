package router

import (
	"fmt"
	"sync"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
)

type Request struct {
	SeqId int64
	OpStr string

	Resp *redis.Resp

	Response struct {
		Resp *redis.Resp
		Err  error
	}

	slot *Slot
	wait *sync.WaitGroup
}

func (r *Request) SetResponse(resp *redis.Resp, err error) {
	r.Response.Resp, r.Response.Err = resp, err
	r.wait.Done()
	r.slot.jobs.Done()
}

func (r *Request) String() string {
	return fmt.Sprintf("request{slot: %4d opstr: %6s seqid: %d}",
		r.slot.Id, r.OpStr, r.SeqId)
}
