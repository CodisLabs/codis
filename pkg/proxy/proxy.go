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
	"github.com/CodisLabs/codis/pkg/proxy/router"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/rpc"
)

type Proxy struct {
	mu sync.Mutex

	xauth string
	model *models.Proxy

	jodis *Jodis

	init, exit struct {
		C chan struct{}
	}
	online bool
	closed bool

	lproxy net.Listener
	ladmin net.Listener

	config *Config
	router *router.Router
}

var ErrClosedProxy = errors.New("use of closed proxy")

func New(config *Config) (*Proxy, error) {
	if !utils.IsValidProduct(config.ProductName) {
		return nil, errors.Errorf("invalid product name = %s", config.ProductName)
	}
	s := &Proxy{config: config}
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

	s.router = router.NewWithAuth(config.ProductAuth)
	s.init.C = make(chan struct{})
	s.exit.C = make(chan struct{})

	if err := s.setup(config); err != nil {
		s.Close()
		return nil, err
	}

	log.Warnf("[%p] create new proxy:\n%s", s, s.model.Encode())

	go s.serveAdmin()
	go s.serveProxy()

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

		x, err := utils.ResolveAddr("tcp", l.Addr().String(), config.HostAdmin)
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
	close(s.init.C)

	if s.jodis == nil && s.config.JodisAddr != "" {
		s.jodis = NewJodis(s.config.JodisAddr, s.config.JodisTimeout, s.config.JodisCompatible, s.model)
	}
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

	if s.jodis != nil {
		s.jodis.Close()
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

func (s *Proxy) Token() string {
	return s.model.Token
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

func (s *Proxy) FillSlot(i int, addr, from string, locked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	return s.router.FillSlot(i, addr, from, locked)
}

func (s *Proxy) FillSlots(slots []*models.Slot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	for _, slot := range slots {
		i, locked := slot.Id, slot.Locked
		addr := slot.BackendAddr
		from := slot.MigrateFrom
		if err := s.router.FillSlot(i, addr, from, locked); err != nil {
			return err
		}
	}
	return nil
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

	select {
	case <-s.exit.C:
		return
	case <-s.init.C:
	}

	log.Warnf("[%p] proxy start service on %s", s, s.lproxy.Addr())

	ch := make(chan net.Conn, 4096)

	var nn = utils.MinInt(8, utils.MaxInt(4, runtime.GOMAXPROCS(0)))
	for i := 0; i < nn; i++ {
		go func() {
			for c := range ch {
				s.newSession(c)
			}
		}()
	}

	eh := make(chan error, 1)
	go func(l net.Listener) (err error) {
		defer func() {
			eh <- err
			close(ch)
		}()
		for {
			c, err := s.acceptConn(l)
			if err != nil {
				return err
			}
			ch <- c
		}
	}(s.lproxy)

	if seconds := s.config.BackendPingPeriod; seconds > 0 {
		go s.keepAlive(seconds)
	}

	if s.jodis != nil {
		go s.jodis.Run()
	}

	select {
	case <-s.exit.C:
		log.Warnf("[%p] proxy shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] proxy exit on error", s)
	}
}

func (s *Proxy) keepAlive(seconds int) {
	var ticker = time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		for i := 0; i < seconds; i++ {
			select {
			case <-s.exit.C:
				return
			case <-ticker.C:
			}
		}
		s.router.KeepAlive()
	}
}

func (s *Proxy) newSession(c net.Conn) {
	x := router.NewSessionSize(c, s.config.ProductAuth,
		s.config.SessionMaxBufSize, s.config.SessionMaxTimeout)
	x.SetKeepAlivePeriod(s.config.SessionKeepAlivePeriod)
	x.Start(s.router, s.config.SessionMaxPipeline)
}

func (s *Proxy) acceptConn(l net.Listener) (net.Conn, error) {
	var delay time.Duration
	for {
		c, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.WarnErrorf(err, "[%p] proxy accept new connection failed", s)
				delay = delay * 2
				delay = utils.MaxDuration(delay, time.Millisecond*10)
				delay = utils.MinDuration(delay, time.Millisecond*500)
				time.Sleep(delay)
				continue
			}
		}
		return c, err
	}
}
