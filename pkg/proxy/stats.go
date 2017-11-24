// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
)

type opStats struct {
	opstr string
	calls atomic2.Int64
	nsecs atomic2.Int64
	fails atomic2.Int64
	redis struct {
		errors atomic2.Int64
	}
}

func (s *opStats) OpStats() *OpStats {
	o := &OpStats{
		OpStr: s.opstr,
		Calls: s.calls.Int64(),
		Usecs: s.nsecs.Int64() / 1e3,
		Fails: s.fails.Int64(),
	}
	if o.Calls != 0 {
		o.UsecsPercall = o.Usecs / o.Calls
	}
	o.RedisErrType = s.redis.errors.Int64()
	return o
}

type OpStats struct {
	OpStr        string `json:"opstr"`
	Calls        int64  `json:"calls"`
	Usecs        int64  `json:"usecs"`
	UsecsPercall int64  `json:"usecs_percall"`
	Fails        int64  `json:"fails"`
	RedisErrType int64  `json:"redis_errtype"`
}

var cmdstats struct {
	sync.RWMutex

	opmap map[string]*opStats
	total atomic2.Int64
	fails atomic2.Int64
	redis struct {
		errors atomic2.Int64
	}

	qps atomic2.Int64
}

func init() {
	cmdstats.opmap = make(map[string]*opStats, 128)
	go func() {
		for {
			start := time.Now()
			total := cmdstats.total.Int64()
			time.Sleep(time.Second)
			delta := cmdstats.total.Int64() - total
			normalized := math.Max(0, float64(delta)) * float64(time.Second) / float64(time.Since(start))
			cmdstats.qps.Set(int64(normalized + 0.5))
		}
	}()
}

func OpTotal() int64 {
	return cmdstats.total.Int64()
}

func OpFails() int64 {
	return cmdstats.fails.Int64()
}

func OpRedisErrors() int64 {
	return cmdstats.redis.errors.Int64()
}

func OpQPS() int64 {
	return cmdstats.qps.Int64()
}

func getOpStats(opstr string, create bool) *opStats {
	cmdstats.RLock()
	s := cmdstats.opmap[opstr]
	cmdstats.RUnlock()

	if s != nil || !create {
		return s
	}

	cmdstats.Lock()
	s = cmdstats.opmap[opstr]
	if s == nil {
		s = &opStats{opstr: opstr}
		cmdstats.opmap[opstr] = s
	}
	cmdstats.Unlock()
	return s
}

type sliceOpStats []*OpStats

func (s sliceOpStats) Len() int {
	return len(s)
}

func (s sliceOpStats) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sliceOpStats) Less(i, j int) bool {
	return s[i].OpStr < s[j].OpStr
}

func GetOpStatsAll() []*OpStats {
	var all = make([]*OpStats, 0, 128)
	cmdstats.RLock()
	for _, s := range cmdstats.opmap {
		all = append(all, s.OpStats())
	}
	cmdstats.RUnlock()
	sort.Sort(sliceOpStats(all))
	return all
}

func ResetStats() {
	cmdstats.Lock()
	cmdstats.opmap = make(map[string]*opStats, 128)
	cmdstats.Unlock()

	cmdstats.total.Set(0)
	cmdstats.fails.Set(0)
	cmdstats.redis.errors.Set(0)
	sessions.total.Set(sessions.alive.Int64())
}

func incrOpTotal(n int64) {
	cmdstats.total.Add(n)
}

func incrOpFails(n int64) {
	cmdstats.fails.Add(n)
}

func incrOpStats(e *opStats) {
	s := getOpStats(e.opstr, true)
	s.calls.Add(e.calls.Swap(0))
	s.nsecs.Add(e.nsecs.Swap(0))
	if n := e.fails.Swap(0); n != 0 {
		s.fails.Add(n)
		cmdstats.fails.Add(n)
	}
	if n := e.redis.errors.Swap(0); n != 0 {
		s.redis.errors.Add(n)
		cmdstats.redis.errors.Add(n)
	}
}

var sessions struct {
	total atomic2.Int64
	alive atomic2.Int64
}

func incrSessions() int64 {
	sessions.total.Incr()
	return sessions.alive.Incr()
}

func decrSessions() {
	sessions.alive.Decr()
}

func SessionsTotal() int64 {
	return sessions.total.Int64()
}

func SessionsAlive() int64 {
	return sessions.alive.Int64()
}

type SysUsage struct {
	Now time.Time
	CPU float64
	*utils.Usage
}

var lastSysUsage atomic.Value

func init() {
	go func() {
		for {
			cpu, usage, err := utils.CPUUsage(time.Second)
			if err != nil {
				lastSysUsage.Store(&SysUsage{
					Now: time.Now(),
				})
			} else {
				lastSysUsage.Store(&SysUsage{
					Now: time.Now(),
					CPU: cpu, Usage: usage,
				})
			}
			if err != nil {
				time.Sleep(time.Second * 5)
			}
		}
	}()
}

func GetSysUsage() *SysUsage {
	if p := lastSysUsage.Load(); p != nil {
		return p.(*SysUsage)
	}
	return nil
}
