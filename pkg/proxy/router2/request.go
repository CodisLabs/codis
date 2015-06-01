package router

import (
	"fmt"
	"sync"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
)

type Request struct {
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

func (r *Request) String() string {
	return fmt.Sprintf("request{slot: %4d opstr: %6s seqid: %d}",
		r.slot.Id, r.OpStr, r.SeqId)
}
