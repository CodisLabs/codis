package router

import (
	"container/list"
	"github.com/juju/errors"
	log "github.com/ngaut/logging"
	"github.com/wandoulabs/codis/pkg/proxy/parser"
	"github.com/wandoulabs/codis/pkg/proxy/redisconn"
	"sync"
	"time"
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

	log.Debugf("handleTask:%v", r)
	tr.tasks.PushBack(r)

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

func (tr *taskRunner) tryRecover(err error) error {
	log.Warning(errors.ErrorStack(err))
	//clean up all task
	for e := tr.tasks.Front(); e != nil; {
		req := e.Value.(*PipelineRequest)
		log.Info("clean up", req)
		req.backQ <- &PipelineResponse{ctx: req, resp: nil, err: err}
		next := e.Next()
		tr.tasks.Remove(e)
		e = next
	}
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

func (tr *taskRunner) processTask(t interface{}) error {
	switch t.(type) {
	case *PipelineRequest:
		r := t.(*PipelineRequest)
		var flush bool
		if len(tr.in) == 0 { //force flush
			flush = true
		}

		return tr.handleTask(r, flush)
	case *sync.WaitGroup: //close
		err := tr.handleTask(nil, true) //flush
		tr.closed = true
		return err
	}

	return nil
}

func (tr *taskRunner) handleResponse(e interface{}) error {
	switch e.(type) {
	case error:
		return e.(error)
	case *parser.Resp:
		resp := e.(*parser.Resp)
		e := tr.tasks.Front()
		req := e.Value.(*PipelineRequest)
		log.Debug("finish", req)
		req.backQ <- &PipelineResponse{ctx: req, resp: resp, err: nil}
		tr.tasks.Remove(e)
		return nil
	}

	return nil
}

func (tr *taskRunner) writeloop() {
	var err error
	var wgClose *sync.WaitGroup
	for {
		if tr.closed && tr.tasks.Len() == 0 {
			wgClose.Done()
			return
		}

		if err != nil { //clean up
			err = tr.tryRecover(err)
			if err != nil {
				//todo: clean tasks in tr.in
				continue
			}
		}

		select {
		case t := <-tr.in:
			err = tr.processTask(t)
		case resp := <-tr.out:
			err = tr.handleResponse(resp)
		}
	}
}

func NewTaskRunner(addr string, netTimeout int) (*taskRunner, error) {
	tr := &taskRunner{
		in:         make(chan interface{}, 100),
		out:        make(chan interface{}, 100),
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
