package router

import (
	"container/list"
	log "github.com/ngaut/logging"
	"github.com/wandoulabs/codis/pkg/proxy/parser"
	"github.com/wandoulabs/codis/pkg/proxy/redisconn"
	"sync"
)

type taskRunner struct {
	evtbus    chan interface{}
	in        chan interface{} //*PipelineRequest
	out       chan *parser.Resp
	redisAddr string
	tasks     *list.List
	c         *redisconn.Conn
}

func (tr *taskRunner) readloop() {
	for {
		resp, err := parser.Parse(tr.c.BufioReader())
		if err != nil {
			return
		}

		tr.out <- resp
	}
}

func (tr *taskRunner) dowrite(r *PipelineRequest, flush bool) error {
	b, err := r.req.Bytes()
	if err != nil {
		log.Warning(err)
		return err
	}

	_, err = tr.c.Write(b)
	if err != nil {
		log.Warning(err)
		return err
	}

	if flush {
		return tr.c.Flush()
	}

	return nil
}

func (tr *taskRunner) handleTask(r *PipelineRequest, flush bool) error {
	if r == nil && flush { //just flush
		return tr.c.Flush()
	}

	log.Debugf("handleTask:%v", r)
	tr.tasks.PushBack(r)

	err := tr.dowrite(r, flush)
	if err != nil {
		//todo: how to handle this error
		//notify all request, close client connection ?
		log.Error(err)
		return err
	}

	return nil
}

func (tr *taskRunner) writeloop() {
	var bufCnt int64
	var closed bool
	var err error
	var wgClose *sync.WaitGroup
	for {
		if closed && tr.tasks.Len() == 0 {
			wgClose.Done()
			return
		}

		if err != nil { //clean up
			for e := tr.tasks.Front(); e != nil; e = e.Next() {
				req := e.Value.(*PipelineRequest)
				log.Info("clean up", req)
				req.backQ <- &PipelineResponse{ctx: req, resp: nil, err: err}
				next := e.Next()
				tr.tasks.Remove(e)
				e = next
			}
		}

		select {
		case t := <-tr.in:
			switch t.(type) {
			case *PipelineRequest:
				r := t.(*PipelineRequest)
				var flush bool
				bufCnt++
				if len(tr.in) == 0 { //force flush
					flush = true
				}

				err = tr.handleTask(r, flush)
			case *sync.WaitGroup: //close
				err = tr.handleTask(nil, true) //flush
				closed = true
			}
		case resp := <-tr.out:
			e := tr.tasks.Front()
			req := e.Value.(*PipelineRequest)
			log.Debug("finish", req)
			req.backQ <- &PipelineResponse{ctx: req, resp: resp, err: nil}
			tr.tasks.Remove(e)
		}
	}
}

func NewTaskRunner(addr string) (*taskRunner, error) {
	tr := &taskRunner{
		in:        make(chan interface{}, 100),
		out:       make(chan *parser.Resp, 100),
		redisAddr: addr,
		tasks:     list.New(),
	}

	c, err := redisconn.NewConnection(addr)
	if err != nil {
		return nil, err
	}

	tr.c = c

	go tr.writeloop()
	go tr.readloop()

	return tr, nil
}
