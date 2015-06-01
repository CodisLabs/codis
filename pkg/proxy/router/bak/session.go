// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/juju/errors"
	log "github.com/ngaut/logging"
	"github.com/wandoulabs/codis/pkg/proxy/parser"
)

type session struct {
	r *bufio.Reader
	w *bufio.Writer
	net.Conn

	CreateAt              time.Time
	Ops                   int64
	pipelineSeq           int64
	backQ                 chan *PipelineResponse
	lastUnsentResponseSeq int64
	closed                bool
	closeSignal           *sync.WaitGroup
}

type PipelineRequest struct {
	slotIdx int
	op      []byte
	keys    [][]byte
	seq     int64
	backQ   chan *PipelineResponse
	req     *parser.Resp
	wg      *sync.WaitGroup
}

func (pr *PipelineRequest) String() string {
	return fmt.Sprintf("op:%s, seq:%d, slot:%d", string(pr.op), pr.seq, pr.slotIdx)
}

type PipelineResponse struct {
	resp *parser.Resp
	err  error
	ctx  *PipelineRequest
}

func (s *session) String() string {
	return fmt.Sprintf("conn:%s, CreateAt:%s, Ops:%d, closed:%v", s.Conn.RemoteAddr(), s.CreateAt, s.Ops, s.closed)
}

func (s *session) writeResp(resp *PipelineResponse) error {
	buf, err := resp.resp.Bytes()
	if err != nil {
		return errors.Trace(err)
	}
	_, err = s.Write(buf)
	return errors.Trace(err)
}

func (s *session) handleResponse(resp *PipelineResponse) (flush bool, err error) {
	if resp.ctx.seq != s.lastUnsentResponseSeq {
		log.Fatal("should never happend")
	}

	s.lastUnsentResponseSeq++
	if resp.ctx.wg != nil {
		resp.ctx.wg.Done()
	}

	if resp.err != nil { //notify close client connection
		return true, resp.err
	}

	if !s.closed {
		if err := s.writeResp(resp); err != nil {
			return false, errors.Trace(err)
		}

		flush = true
	}

	return
}

func (s *session) Close() {
	s.Conn.Close()
	s.closed = true
}

func (s *session) WritingLoop() {
	s.lastUnsentResponseSeq = 1
	for {
		select {
		case resp, ok := <-s.backQ:
			if !ok {
				s.Close()
				s.closeSignal.Done()
				return
			}

			flush, err := s.handleResponse(resp)
			if err != nil {
				log.Warning(s.RemoteAddr(), resp.ctx, errors.ErrorStack(err))
				s.Close() //notify reader to exit
				continue
			}

			if flush && len(s.backQ) == 0 {
				err := s.w.Flush()
				if err != nil {
					s.Close() //notify reader to exit
					log.Warning(s.RemoteAddr(), resp.ctx, errors.ErrorStack(err))
					continue
				}
			}
		}
	}
}

//make sure all read using bufio.Reader
func (s *session) Read(p []byte) (int, error) {
	panic("not implemented")
}

//write without bufio
func (s *session) Write(p []byte) (int, error) {
	return s.w.Write(p)
}
