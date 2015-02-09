// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"bufio"
	"fmt"
	"github.com/juju/errors"
	log "github.com/ngaut/logging"
	"github.com/wandoulabs/codis/pkg/proxy/parser"
	"net"
	"time"
)

type session struct {
	r *bufio.Reader
	w *bufio.Writer
	net.Conn

	CreateAt              time.Time
	Ops                   int64
	pipelineSeq           int64
	backQ                 chan *PipelineResponse
	unsentResponses       map[int64]*PipelineResponse
	lastUnsentResponseSeq int64
}

type PipelineRequest struct {
	slotIdx int
	op      []byte
	keys    [][]byte
	seq     int64
	backQ   chan *PipelineResponse
	req     *parser.Resp
}

func (pr *PipelineRequest) String() string {
	return fmt.Sprintf("op:%s, seq:%d, slot:%d", string(pr.op), pr.seq, pr.slotIdx)
}

type PipelineResponse struct {
	resp *parser.Resp
	err  error
	ctx  *PipelineRequest
}

func (s *session) writeResp(resp *PipelineResponse) error {
	buf, err := resp.resp.Bytes()
	if err != nil {
		return errors.Trace(err)
	}
	_, err = s.Write(buf)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (s *session) handleResponse(resp *PipelineResponse) error {
	log.Debug("session handleResponse ", resp.ctx, s.lastUnsentResponseSeq)
	s.unsentResponses[resp.ctx.seq] = resp

	var flush bool

	for { //are there any more continues responses
		resp, ok := s.unsentResponses[s.lastUnsentResponseSeq]
		if !ok {
			break
		}

		log.Debugf("write %+v", resp.ctx.seq)
		if err := s.writeResp(resp); err != nil {
			return errors.Trace(err)
		}
		delete(s.unsentResponses, s.lastUnsentResponseSeq)
		s.lastUnsentResponseSeq++
		flush = true
	}

	if flush {
		s.w.Flush()
	}

	return nil
}

func (s *session) WritingLoop() {
	s.lastUnsentResponseSeq = 1
	for {
		select {
		case resp := <-s.backQ:
			err := s.handleResponse(resp)
			if err != nil {
				log.Warning(s.RemoteAddr(), errors.ErrorStack(err))
			}
		}
	}
}

//make sure all read using bufio.Reader
func (s *session) Read(p []byte) (int, error) {
	return 0, errors.New("not implemented")
}

//write without bufio
func (s *session) Write(p []byte) (int, error) {
	return s.w.Write(p)
}
