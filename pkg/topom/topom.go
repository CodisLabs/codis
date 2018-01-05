// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"container/list"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/math2"
	"github.com/CodisLabs/codis/pkg/utils/redis"
	"github.com/CodisLabs/codis/pkg/utils/rpc"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
)

type Topom struct {
	mu sync.Mutex

	xauth string
	model *models.Topom
	store *models.Store
	cache struct {
		hooks list.List
		slots []*models.SlotMapping
		group map[int]*models.Group
		proxy map[string]*models.Proxy

		sentinel *models.Sentinel
		cHashring    *models.Consistent
	}

	exit struct {
		C chan struct{}
	}

	config *Config
	online bool
	closed bool

	ladmin net.Listener

	action struct {
		redisp *redis.Pool

		interval atomic2.Int64
		disabled atomic2.Bool

		progress struct {
			status atomic.Value
		}
		executor atomic2.Int64
	}

	stats struct {
		redisp *redis.Pool

		servers map[string]*RedisStats
		proxies map[string]*ProxyStats
	}

	ha struct {
		redisp *redis.Pool

		monitor *redis.Sentinel
		masters map[int]string
	}
}

var ErrClosedTopom = errors.New("use of closed topom")

func New(client models.Client, config *Config) (*Topom, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}
	if err := models.ValidateProduct(config.ProductName); err != nil {
		return nil, errors.Trace(err)
	}
	s := &Topom{}
	s.config = config
	s.exit.C = make(chan struct{})
	s.action.redisp = redis.NewPool(config.ProductAuth, config.MigrationTimeout.Duration())
	s.action.progress.status.Store("")

	s.ha.redisp = redis.NewPool("", time.Second*5)

	s.model = &models.Topom{
		StartTime: time.Now().String(),
	}
	s.model.ProductName = config.ProductName
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()
	if b, err := exec.Command("uname", "-a").Output(); err != nil {
		log.WarnErrorf(err, "run command uname failed")
	} else {
		s.model.Sys = strings.TrimSpace(string(b))
	}
	s.store = models.NewStore(client, config.ProductName)

	s.stats.redisp = redis.NewPool(config.ProductAuth, time.Second*5)
	s.stats.servers = make(map[string]*RedisStats)
	s.stats.proxies = make(map[string]*ProxyStats)

	if err := s.setup(config); err != nil {
		s.Close()
		return nil, err
	}

	log.Warnf("create new topom:\n%s", s.model.Encode())

	go s.serveAdmin()

	return s, nil
}

func (s *Topom) setup(config *Config) error {
	if l, err := net.Listen("tcp", config.AdminAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.ladmin = l

		x, err := utils.ReplaceUnspecifiedIP("tcp", l.Addr().String(), s.config.HostAdmin)
		if err != nil {
			return err
		}
		s.model.AdminAddr = x
	}

	s.model.Token = rpc.NewToken(
		config.ProductName,
		s.ladmin.Addr().String(),
	)
	s.xauth = rpc.NewXAuth(config.ProductName)

	return nil
}

func (s *Topom) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.exit.C)

	if s.ladmin != nil {
		s.ladmin.Close()
	}
	for _, p := range []*redis.Pool{
		s.action.redisp, s.stats.redisp, s.ha.redisp,
	} {
		if p != nil {
			p.Close()
		}
	}

	defer s.store.Close()

	if s.online {
		if err := s.store.Release(); err != nil {
			log.ErrorErrorf(err, "store: release lock of %s failed", s.config.ProductName)
			return errors.Errorf("store: release lock of %s failed", s.config.ProductName)
		}
	}
	return nil
}

func (s *Topom) Start(routines bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}
	if s.online {
		return nil
	} else {
		if err := s.store.Acquire(s.model); err != nil {
			log.ErrorErrorf(err, "store: acquire lock of %s failed", s.config.ProductName)
			return errors.Errorf("store: acquire lock of %s failed", s.config.ProductName)
		}
		s.online = true
	}

	if !routines {
		return nil
	}
	ctx, err := s.newContext()
	if err != nil {
		return err
	}
	s.rewatchSentinels(ctx.sentinel.Servers)

	go func() {
		for !s.IsClosed() {
			if s.IsOnline() {
				w, _ := s.RefreshRedisStats(time.Second)
				if w != nil {
					w.Wait()
				}
			}
			time.Sleep(time.Second)
		}
	}()

	go func() {
		for !s.IsClosed() {
			if s.IsOnline() {
				w, _ := s.RefreshProxyStats(time.Second)
				if w != nil {
					w.Wait()
				}
			}
			time.Sleep(time.Second)
		}
	}()

	go func() {
		for !s.IsClosed() {
			if s.IsOnline() {
				if err := s.ProcessSlotAction(); err != nil {
					log.WarnErrorf(err, "process slot action failed")
					time.Sleep(time.Second * 5)
				}
			}
			time.Sleep(time.Second)
		}
	}()

	go func() {
		for !s.IsClosed() {
			if s.IsOnline() {
				if err := s.ProcessSyncAction(); err != nil {
					log.WarnErrorf(err, "process sync action failed")
					time.Sleep(time.Second * 5)
				}
			}
			time.Sleep(time.Second)
		}
	}()

	return nil
}

func (s *Topom) XAuth() string {
	return s.xauth
}

func (s *Topom) Model() *models.Topom {
	return s.model
}

var ErrNotOnline = errors.New("topom is not online")

func (s *Topom) newContext() (*context, error) {
	if s.closed {
		return nil, ErrClosedTopom
	}
	if s.online {
		if err := s.refillCache(); err != nil {
			return nil, err
		} else {
			ctx := &context{}
			ctx.slots = s.cache.slots
			ctx.group = s.cache.group
			ctx.proxy = s.cache.proxy
			ctx.sentinel = s.cache.sentinel
			ctx.cHashring = s.cache.cHashring
			ctx.hosts.m = make(map[string]net.IP)
			ctx.method, _ = models.ParseForwardMethod(s.config.MigrationMethod)
			return ctx, nil
		}
	} else {
		return nil, ErrNotOnline
	}
}

func (s *Topom) Stats() (*Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return nil, err
	}

	stats := &Stats{}
	stats.Closed = s.closed

	stats.Slots = ctx.slots

	stats.Group.Models = models.SortGroup(ctx.group)
	stats.Group.Stats = map[string]*RedisStats{}
	for _, g := range ctx.group {
		for _, x := range g.Servers {
			if v := s.stats.servers[x.Addr]; v != nil {
				stats.Group.Stats[x.Addr] = v
			}
		}
	}

	stats.Proxy.Models = models.SortProxy(ctx.proxy)
	stats.Proxy.Stats = s.stats.proxies

	stats.SlotAction.Interval = s.action.interval.Int64()
	stats.SlotAction.Disabled = s.action.disabled.Bool()
	stats.SlotAction.Progress.Status = s.action.progress.status.Load().(string)
	stats.SlotAction.Executor = s.action.executor.Int64()

	stats.HA.Model = ctx.sentinel
	stats.HA.Stats = map[string]*RedisStats{}
	for _, server := range ctx.sentinel.Servers {
		if v := s.stats.servers[server]; v != nil {
			stats.HA.Stats[server] = v
		}
	}
	stats.HA.Masters = make(map[string]string)
	if s.ha.masters != nil {
		for gid, addr := range s.ha.masters {
			stats.HA.Masters[strconv.Itoa(gid)] = addr
		}
	}
	return stats, nil
}

type Stats struct {
	Closed bool `json:"closed"`

	Slots []*models.SlotMapping `json:"slots"`

	Group struct {
		Models []*models.Group        `json:"models"`
		Stats  map[string]*RedisStats `json:"stats"`
	} `json:"group"`

	Proxy struct {
		Models []*models.Proxy        `json:"models"`
		Stats  map[string]*ProxyStats `json:"stats"`
	} `json:"proxy"`

	SlotAction struct {
		Interval int64 `json:"interval"`
		Disabled bool  `json:"disabled"`

		Progress struct {
			Status string `json:"status"`
		} `json:"progress"`

		Executor int64 `json:"executor"`
	} `json:"slot_action"`

	HA struct {
		Model   *models.Sentinel       `json:"model"`
		Stats   map[string]*RedisStats `json:"stats"`
		Masters map[string]string      `json:"masters"`
	} `json:"sentinels"`
}

func (s *Topom) Config() *Config {
	return s.config
}

func (s *Topom) IsOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online && !s.closed
}

func (s *Topom) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Topom) GetSlotActionInterval() int {
	return s.action.interval.AsInt()
}

func (s *Topom) SetSlotActionInterval(us int) {
	us = math2.MinMaxInt(us, 0, 1000*1000)
	s.action.interval.Set(int64(us))
	log.Warnf("set action interval = %d", us)
}

func (s *Topom) GetSlotActionDisabled() bool {
	return s.action.disabled.Bool()
}

func (s *Topom) SetSlotActionDisabled(value bool) {
	s.action.disabled.Set(value)
	log.Warnf("set action disabled = %t", value)
}

func (s *Topom) Slots() ([]*models.Slot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return nil, err
	}
	return ctx.toSlotSlice(ctx.slots, nil), nil
}

func (s *Topom) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.newContext()
	if err != nil {
		return err
	}
	defer s.dirtyCacheAll()
	return nil
}

func (s *Topom) serveAdmin() {
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Warnf("admin start service on %s", s.ladmin.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) {
		h := http.NewServeMux()
		h.Handle("/", newApiServer(s))
		hs := &http.Server{Handler: h}
		eh <- hs.Serve(l)
	}(s.ladmin)

	select {
	case <-s.exit.C:
		log.Warnf("admin shutdown")
	case err := <-eh:
		log.ErrorErrorf(err, "admin exit on error")
	}
}

type Overview struct {
	Version string        `json:"version"`
	Compile string        `json:"compile"`
	Config  *Config       `json:"config,omitempty"`
	Model   *models.Topom `json:"model,omitempty"`
	Stats   *Stats        `json:"stats,omitempty"`
	WorkingMode string	  `json:"workingMode"`
	GroupWeight  map[int]string   `json:"groupWeight"`
}

func (s *Topom) Overview() (*Overview, error) {
	if stats, err := s.Stats(); err != nil {
		return nil, err
	} else {
		return &Overview{
			//Version: utils.Version,
			//Compile: utils.Compile,
			Config: s.Config(),
			Model:  s.Model(),
			Stats:  stats,
			WorkingMode: s.config.WorkingMode,
			GroupWeight: s.getGroupWeight(),
		}, nil
	}
}

func (s *Topom) getGroupWeight() map[int]string{
	ctx, err := s.newContext()
	if err != nil {
		return nil
	}
	if ctx.cHashring == nil{
		ctx.cHashring = models.NewConsistent()
	}
	return ctx.cHashring.NodeStatus
}
