// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/utils/sync2/atomic2"
)

type opStats struct {
	opstr string
	calls atomic2.Int64
	usecs atomic2.Int64
}

func (s *opStats) OpStats() *OpStats {
	o := &OpStats{
		OpStr: s.opstr,
		Calls: s.calls.Get(),
		Usecs: s.usecs.Get(),
	}
	if o.Calls != 0 {
		o.UsecsPercall = o.Usecs / o.Calls
	}
	return o
}

type OpStats struct {
	OpStr        string `json:"opstr"`
	Calls        int64  `json:"calls"`
	Usecs        int64  `json:"usecs"`
	UsecsPercall int64  `json:"usecs_percall"`
}

var cmdstats struct {
	total atomic2.Int64
	fails atomic2.Int64

	opmap map[string]*opStats
	rwlck sync.RWMutex

	qps atomic2.Int64
}

func init() {
	cmdstats.opmap = make(map[string]*opStats)
	go func() {
		for {
			lastn := cmdstats.total.Get()
			time.Sleep(time.Second)
			delta := cmdstats.total.Get() - lastn
			cmdstats.qps.Set(delta)
		}
	}()
}

func OpTotal() int64 {
	return cmdstats.total.Get()
}

func OpFails() int64 {
	return cmdstats.fails.Get()
}

func OpQps() int64 {
	return cmdstats.qps.Get()
}

func getOpStats(opstr string, create bool) *opStats {
	cmdstats.rwlck.RLock()
	s := cmdstats.opmap[opstr]
	cmdstats.rwlck.RUnlock()

	if s != nil || !create {
		return s
	}

	cmdstats.rwlck.Lock()
	s = cmdstats.opmap[opstr]
	if s == nil {
		s = &opStats{opstr: opstr}
		cmdstats.opmap[opstr] = s
	}
	cmdstats.rwlck.Unlock()
	return s
}

func GetOpStatsAll() []*OpStats {
	var all = make([]*OpStats, 0, 128)
	cmdstats.rwlck.RLock()
	for _, s := range cmdstats.opmap {
		all = append(all, s.OpStats())
	}
	cmdstats.rwlck.RUnlock()
	return all
}

func incrOpTotal() {
	cmdstats.total.Incr()
}

func incrOpFails() {
	cmdstats.fails.Incr()
}

func incrOpStats(opstr string, usecs int64) {
	s := getOpStats(opstr, true)
	s.calls.Incr()
	s.usecs.Add(usecs)
}

var sessions struct {
	total atomic2.Int64
	alive atomic2.Int64
}

func incrSessions() {
	sessions.total.Incr()
	sessions.alive.Incr()
}

func decrSessions() {
	sessions.alive.Decr()
}

func SessionsTotal() int64 {
	return sessions.total.Get()
}

func SessionsAlive() int64 {
	return sessions.alive.Get()
}
