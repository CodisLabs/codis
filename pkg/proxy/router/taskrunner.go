package router

import (
	"container/list"
	"sync"
	"time"

	"github.com/juju/errors"
	log "github.com/ngaut/logging"
	"github.com/wandoulabs/codis/pkg/proxy/parser"
	"github.com/wandoulabs/codis/pkg/proxy/redisconn"
)

type taskRunner struct {
	evtbus     chan interface{}
	in         chan interface{} //*PipelineRequest
	out        chan interface{}
	redisAddr  string
	tasks      *list.List
	c          *redisconn.Conn
	netTimeout int //second
	closed     bool
	wgClose    *sync.WaitGroup
	latest     time.Time //latest request time stamp
}

func (tr *taskRunner) readloop() {
	for {
		resp, err := parser.Parse(tr.c.BufioReader())
		if err != nil {
			tr.out <- err
			return
		}

		tr.out <- resp
	}
}

func (tr *taskRunner) dowrite(r *PipelineRequest, flush bool) error {
	b, err := r.req.Bytes()
	if err != nil {
		return errors.Trace(err)
	}

	_, err = tr.c.Write(b)
	if err != nil {
		return errors.Trace(err)
	}

	if flush {
		return errors.Trace(tr.c.Flush())
	}

	return nil
}

func (tr *taskRunner) handleTask(r *PipelineRequest, flush bool) error {
	if r == nil && flush { //just flush
		return tr.c.Flush()
	}

	tr.tasks.PushBack(r)
	tr.latest = time.Now()

	return errors.Trace(tr.dowrite(r, flush))
}

func (tr *taskRunner) cleanupQueueTasks() {
	for {
		select {
		case t := <-tr.in:
			tr.processTask(t)
		default:
			return
		}
	}
}

func (tr *taskRunner) cleanupOutgoingTasks(err error) {
	for e := tr.tasks.Front(); e != nil; {
		req := e.Value.(*PipelineRequest)
		log.Info("clean up", req)
		req.backQ <- &PipelineResponse{ctx: req, resp: nil, err: err}
		next := e.Next()
		tr.tasks.Remove(e)
		e = next
	}
}

func (tr *taskRunner) tryRecover(err error) error {
	log.Warning("try recover from ", err)
	tr.cleanupOutgoingTasks(err)
	//try to recover
	c, err := redisconn.NewConnection(tr.redisAddr, tr.netTimeout)
	if err != nil {
		tr.cleanupQueueTasks() //do not block dispatcher
		log.Warning(err)
		time.Sleep(1 * time.Second)
		return err
	}

	tr.c = c
	go tr.readloop()

	return nil
}

func (tr *taskRunner) getOutgoingResponse() {
	for {
		if tr.tasks.Len() == 0 {
			return
		}

		select {
		case resp := <-tr.out:
			err := tr.handleResponse(resp)
			if err != nil {
				tr.cleanupOutgoingTasks(err)
				return
			}
		case <-time.After(2 * time.Second):
			tr.cleanupOutgoingTasks(errors.New("try read response timeout"))
			return
		}
	}
}

func (tr *taskRunner) processTask(t interface{}) {
	var err error
	switch t.(type) {
	case *PipelineRequest:
		r := t.(*PipelineRequest)
		var flush bool
		if len(tr.in) == 0 { //force flush
			flush = true
		}

		err = tr.handleTask(r, flush)
	case *sync.WaitGroup: //close taskrunner
		err = tr.handleTask(nil, true) //flush
		//get all response for out going request
		tr.getOutgoingResponse()
		tr.closed = true
		tr.wgClose = t.(*sync.WaitGroup)
	}

	if err != nil {
		log.Warning(err)
		tr.c.Close()
	}
}

func (tr *taskRunner) handleResponse(e interface{}) error {
	switch e.(type) {
	case error:
		return e.(error)
	case *parser.Resp:
		resp := e.(*parser.Resp)
		e := tr.tasks.Front()
		req := e.Value.(*PipelineRequest)
		req.backQ <- &PipelineResponse{ctx: req, resp: resp, err: nil}
		tr.tasks.Remove(e)
		return nil
	}

	return nil
}

func (tr *taskRunner) writeloop() {
	var err error
	tick := time.Tick(2 * time.Second)
	for {
		if tr.closed && tr.tasks.Len() == 0 {
			log.Warning("exit taskrunner", tr.redisAddr)
			tr.wgClose.Done()
			tr.c.Close()
			return
		}

		if err != nil { //clean up
			err = tr.tryRecover(err)
			if err != nil {
				continue
			}
		}

		select {
		case t := <-tr.in:
			tr.processTask(t)
		case resp := <-tr.out:
			err = tr.handleResponse(resp)
		case <-tick:
			if tr.tasks.Len() > 0 && int(time.Since(tr.latest).Seconds()) > tr.netTimeout {
				tr.c.Close()
			}
		}
	}
}

func NewTaskRunner(addr string, netTimeout int) (*taskRunner, error) {
	tr := &taskRunner{
		in:         make(chan interface{}, 1000),
		out:        make(chan interface{}, 1000),
		redisAddr:  addr,
		tasks:      list.New(),
		netTimeout: netTimeout,
	}

	c, err := redisconn.NewConnection(addr, netTimeout)
	if err != nil {
		return nil, err
	}

	tr.c = c

	go tr.writeloop()
	go tr.readloop()

	return tr, nil
}
