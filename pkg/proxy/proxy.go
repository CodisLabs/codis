// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/math2"
	"github.com/CodisLabs/codis/pkg/utils/rpc"
	"github.com/CodisLabs/codis/pkg/utils/unsafe2"
)

type Proxy struct {
	mu sync.Mutex

	token string
	xauth string
	model *models.Proxy

	exit struct {
		C chan struct{}
	}
	online bool
	closed bool

	config *Config
	router *Router
	ignore []byte

	lproxy net.Listener
	ladmin net.Listener
	xjodis *Jodis
}

var ErrClosedProxy = errors.New("use of closed proxy")

func New(config *Config) (*Proxy, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}
	if err := models.ValidateProduct(config.ProductName); err != nil {
		return nil, errors.Trace(err)
	}

	s := &Proxy{}
	s.config = config
	s.exit.C = make(chan struct{})
	s.router = NewRouter(config)
	s.ignore = make([]byte, config.ProxyHeapPlaceholder.Int())

	s.model = &models.Proxy{
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
	s.model.Hostname, _ = os.Hostname()

	if err := s.setup(config); err != nil {
		s.Close()
		return nil, err
	}

	log.Warnf("[%p] create new proxy:\n%s", s, s.model.Encode())

	unsafe2.SetMaxOffheapBytes(config.ProxyMaxOffheapBytes.Int())

	go s.serveAdmin()
	go s.serveProxy()

	s.startMetricsJson()
	s.startMetricsInfluxdb()

	return s, nil
}

func (s *Proxy) setup(config *Config) error {
	proto := config.ProtoType
	if l, err := net.Listen(proto, config.ProxyAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.lproxy = l

		x, err := utils.ResolveAddr(proto, l.Addr().String(), config.HostProxy)
		if err != nil {
			return err
		}
		s.model.ProtoType = proto
		s.model.ProxyAddr = x
	}

	proto = "tcp"
	if l, err := net.Listen(proto, config.AdminAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.ladmin = l

		x, err := utils.ResolveAddr(proto, l.Addr().String(), config.HostAdmin)
		if err != nil {
			return err
		}
		s.model.AdminAddr = x
	}

	s.model.Token = rpc.NewToken(
		config.ProductName,
		s.lproxy.Addr().String(),
		s.ladmin.Addr().String(),
	)
	s.xauth = rpc.NewXAuth(
		config.ProductName,
		config.ProductAuth,
		s.model.Token,
	)

	if config.JodisName != "" {
		c, err := models.NewClient(config.JodisName, config.JodisAddr, config.JodisTimeout.Get())
		if err != nil {
			return err
		}
		s.xjodis = NewJodis(c, s.model, config.JodisCompatible)
	}

	return nil
}

func (s *Proxy) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	if s.online {
		return nil
	}
	s.online = true
	s.router.Start()
	return nil
}

func (s *Proxy) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.exit.C)

	if s.xjodis != nil {
		s.xjodis.Close()
	}
	if s.ladmin != nil {
		s.ladmin.Close()
	}
	if s.lproxy != nil {
		s.lproxy.Close()
	}
	if s.router != nil {
		s.router.Close()
	}
	return nil
}

func (s *Proxy) XAuth() string {
	return s.xauth
}

func (s *Proxy) Model() *models.Proxy {
	return s.model
}

func (s *Proxy) Config() *Config {
	return s.config
}

func (s *Proxy) IsOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online && !s.closed
}

func (s *Proxy) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Proxy) Slots() []*models.Slot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.router.GetSlots()
}

func (s *Proxy) FillSlot(m *models.Slot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	return s.router.FillSlot(m)
}

func (s *Proxy) FillSlots(slots []*models.Slot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	for _, m := range slots {
		if err := s.router.FillSlot(m); err != nil {
			return err
		}
	}
	return nil
}

func (s *Proxy) SetSentinels(servers []*models.SentinelServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	var addrs []string
	for _, s := range servers {
		addrs = append(addrs, s.Addr)
	}
	return s.router.SetSentinels(addrs)
}

func (s *Proxy) serveAdmin() {
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Warnf("[%p] admin start service on %s", s, s.ladmin.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) {
		h := http.NewServeMux()
		h.Handle("/", newApiServer(s))
		hs := &http.Server{Handler: h}
		eh <- hs.Serve(l)
	}(s.ladmin)

	select {
	case <-s.exit.C:
		log.Warnf("[%p] admin shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] admin exit on error", s)
	}
}

func (s *Proxy) serveProxy() {
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Warnf("[%p] proxy start service on %s", s, s.lproxy.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) (err error) {
		defer func() {
			eh <- err
		}()
		for {
			c, err := s.acceptConn(l)
			if err != nil {
				return err
			}
			s.newSession(c, s.config).Start(s.router, s.config)
		}
	}(s.lproxy)

	if d := s.config.BackendPingPeriod; d != 0 {
		go s.keepAlive(d.Get())
	}
	if s.xjodis != nil {
		s.xjodis.Start()
	}

	select {
	case <-s.exit.C:
		log.Warnf("[%p] proxy shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] proxy exit on error", s)
	}
}

func (s *Proxy) keepAlive(d time.Duration) {
	var ticker = time.NewTicker(math2.MaxDuration(d, time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-s.exit.C:
			return
		case <-ticker.C:
			s.router.KeepAlive()
		}
	}
}

func (s *Proxy) newSession(sock net.Conn, config *Config) *Session {
	c := redis.NewConn(sock,
		config.SessionRecvBufsize.Int(),
		config.SessionSendBufsize.Int(),
	)
	c.ReaderTimeout = config.SessionRecvTimeout.Get()
	c.WriterTimeout = config.SessionSendTimeout.Get()
	c.SetKeepAlivePeriod(config.SessionKeepAlivePeriod.Get())
	return NewSession(c, config.ProductAuth)
}

func (s *Proxy) acceptConn(l net.Listener) (net.Conn, error) {
	var delay int
	for {
		c, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.WarnErrorf(err, "[%p] proxy accept new connection failed", s)
				delay = math2.MinMaxInt(delay*2, 10, 500)
				time.Sleep(time.Duration(delay) * time.Millisecond)
				continue
			}
		}
		return c, err
	}
}

type Overview struct {
	Version string         `json:"version"`
	Compile string         `json:"compile"`
	Config  *Config        `json:"config,omitempty"`
	Model   *models.Proxy  `json:"model,omitempty"`
	Stats   *Stats         `json:"stats,omitempty"`
	Slots   []*models.Slot `json:"slots,omitempty"`
}

type Stats struct {
	Online bool `json:"online"`
	Closed bool `json:"closed"`

	Ops struct {
		Total int64      `json:"total"`
		Fails int64      `json:"fails"`
		Qps   int64      `json:"qps"`
		Cmd   []*OpStats `json:"cmd,omitempty"`
	} `json:"ops"`

	Sessions struct {
		Total int64 `json:"total"`
		Alive int64 `json:"alive"`
	} `json:"sessions"`

	Rusage struct {
		Mem int64   `json:"mem"`
		CPU float64 `json:"cpu"`
	} `json:"rusage"`

	Runtime struct {
		General struct {
			Alloc   uint64 `json:"alloc"`
			Sys     uint64 `json:"sys"`
			Lookups uint64 `json:"lookups"`
			Mallocs uint64 `json:"mallocs"`
			Frees   uint64 `json:"frees"`
		} `json:"general"`

		Heap struct {
			Alloc   uint64 `json:"alloc"`
			Sys     uint64 `json:"sys"`
			Idle    uint64 `json:"idle"`
			Inuse   uint64 `json:"inuse"`
			Objects uint64 `json:"objects"`
		} `json:"heap"`

		GC struct {
			Num          uint32  `json:"num"`
			CPUFraction  float64 `json:"cpu_fraction"`
			TotalPauseMs uint64  `json:"total_pausems"`
		} `json:"gc"`

		NumProcs      int   `json:"num_procs"`
		NumGoroutines int   `json:"num_goroutines"`
		NumCgoCall    int64 `json:"num_cgo_call"`
		MemOffheap    int   `json:"mem_offheap"`
	} `json:"runtime"`
}

func (s *Proxy) Overview(simple bool) *Overview {
	o := &Overview{
		Version: utils.Version,
		Compile: utils.Compile,
		Config:  s.Config(),
		Model:   s.Model(),
		Stats:   s.Stats(simple),
	}
	if !simple {
		o.Slots = s.Slots()
	}
	return o
}

func (s *Proxy) Stats(simple bool) *Stats {
	stats := &Stats{}
	stats.Online = s.IsOnline()
	stats.Closed = s.IsClosed()

	stats.Ops.Total = OpTotal()
	stats.Ops.Fails = OpFails()
	stats.Ops.Qps = OpQps()
	if !simple {
		stats.Ops.Cmd = GetOpStatsAll()
	}

	stats.Sessions.Total = SessionsTotal()
	stats.Sessions.Alive = SessionsAlive()

	stats.Rusage.Mem = GetSysMemTotal()
	stats.Rusage.CPU = GetSysCPUUsage()

	var r runtime.MemStats
	runtime.ReadMemStats(&r)

	stats.Runtime.General.Alloc = r.Alloc
	stats.Runtime.General.Sys = r.Sys
	stats.Runtime.General.Lookups = r.Lookups
	stats.Runtime.General.Mallocs = r.Mallocs
	stats.Runtime.General.Frees = r.Frees
	stats.Runtime.Heap.Alloc = r.HeapAlloc
	stats.Runtime.Heap.Sys = r.HeapSys
	stats.Runtime.Heap.Idle = r.HeapIdle
	stats.Runtime.Heap.Inuse = r.HeapInuse
	stats.Runtime.Heap.Objects = r.HeapObjects
	stats.Runtime.GC.Num = r.NumGC
	stats.Runtime.GC.CPUFraction = r.GCCPUFraction
	stats.Runtime.GC.TotalPauseMs = r.PauseTotalNs / uint64(time.Millisecond)
	stats.Runtime.NumProcs = runtime.GOMAXPROCS(0)
	stats.Runtime.NumGoroutines = runtime.NumGoroutine()
	stats.Runtime.NumCgoCall = runtime.NumCgoCall()
	stats.Runtime.MemOffheap = unsafe2.OffheapBytes()

	return stats
}
