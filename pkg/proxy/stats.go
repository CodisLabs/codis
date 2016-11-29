// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
)

type opStats struct {
	opstr string
	calls atomic2.Int64
	nsecs atomic2.Int64
}

func (s *opStats) OpStats() *OpStats {
	o := &OpStats{
		OpStr: s.opstr,
		Calls: s.calls.Get(),
		Usecs: s.nsecs.Get() / 1e3,
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
	sync.RWMutex

	total atomic2.Int64
	fails atomic2.Int64
	opmap map[string]*opStats

	qps atomic2.Int64
}

func init() {
	cmdstats.opmap = make(map[string]*opStats, 128)
	go func() {
		for {
			start := time.Now()
			total := cmdstats.total.Get()
			time.Sleep(time.Second)
			delta := cmdstats.total.Get() - total
			normalized := math.Max(0, float64(delta)) * float64(time.Second) / float64(time.Since(start))
			cmdstats.qps.Set(int64(normalized + 0.5))
		}
	}()
}

func OpTotal() int64 {
	return cmdstats.total.Get()
}

func OpFails() int64 {
	return cmdstats.fails.Get()
}

func OpQPS() int64 {
	return cmdstats.qps.Get()
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
	sessions.total.Set(sessions.alive.Get())
}

func incrOpTotal(n int64) {
	cmdstats.total.Add(n)
}

func incrOpFails() {
	cmdstats.fails.Incr()
}

func incrOpStats(opstr string, calls int64, nsecs int64) {
	s := getOpStats(opstr, true)
	s.calls.Add(calls)
	s.nsecs.Add(nsecs)
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
	return sessions.total.Get()
}

func SessionsAlive() int64 {
	return sessions.alive.Get()
}

var sysUsage struct {
	mem int64
	cpu float64
}

func init() {
	updateSysUsage := func() error {
		mem, err := utils.MemTotal()
		if err != nil {
			return err
		}
		cpu, err := utils.CPUUsage(time.Second)
		if err != nil {
			return err
		}
		sysUsage.mem = mem
		sysUsage.cpu = cpu
		return nil
	}
	go func() {
		for {
			if err := updateSysUsage(); err != nil {
				sysUsage.mem = 0
				sysUsage.cpu = 0
				time.Sleep(time.Second)
			}
		}
	}()
}

func GetSysMemTotal() int64 {
	return sysUsage.mem
}

func GetSysCPUUsage() float64 {
	return sysUsage.cpu
}
