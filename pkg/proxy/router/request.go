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
	Flush bool

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
		Flush  bool
		SlotId int
	}{
		r.Sid, r.Seq, r.OpStr,
		r.Flush, r.slot.Id,
	}
	b, _ := json.Marshal(o)
	return string(b)
}

func (r *Request) SetResponse(resp *redis.Resp, err error) error {
	r.Response.Resp, r.Response.Err = resp, err
	r.wait.Done()
	if r.slot != nil {
		r.slot.jobs.Done()
	}
	return err
}

func (r *Request) Wait() {
	r.wait.Wait()
}
