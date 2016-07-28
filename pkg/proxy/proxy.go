// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/models"
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

	lproxy net.Listener
	ladmin net.Listener
	xjodis *Jodis

	config *Config
	router *Router
}

var ErrClosedProxy = errors.New("use of closed proxy")

func New(config *Config) (*Proxy, error) {
	if err := models.ValidProductName(config.ProductName); err != nil {
		return nil, err
	}
	s := &Proxy{config: config}
	s.token = rpc.NewToken()
	s.xauth = rpc.NewXAuth(config.ProductName, config.ProductAuth, s.token)
	s.model = &models.Proxy{
		Token: s.token, StartTime: time.Now().String(),
	}
	s.model.ProductName = config.ProductName
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()
	if b, err := exec.Command("uname", "-a").Output(); err != nil {
		log.WarnErrorf(err, "run command uname failed")
	} else {
		s.model.Sys = strings.TrimSpace(string(b))
	}

	s.exit.C = make(chan struct{})
	s.router = NewRouter(config.ProductAuth)

	if err := s.setup(); err != nil {
		s.Close()
		return nil, err
	}

	log.Warnf("[%p] create new proxy:\n%s", s, s.model.Encode())

	switch n := config.MaxAliveSessions; n {
	case 0:
		SetMaxAliveSessions(math.MaxInt32)
	default:
		SetMaxAliveSessions(n)
	}
	unsafe2.SetMaxOffheapBytes(config.MaxOffheapMBytes * 1024 * 1024)

	go s.serveAdmin()
	go s.serveProxy()

	return s, nil
}

func (s *Proxy) setup() error {
	proto := s.config.ProtoType
	if l, err := net.Listen(proto, s.config.ProxyAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.lproxy = l

		x, err := utils.ResolveAddr(proto, l.Addr().String(), s.config.HostProxy)
		if err != nil {
			return err
		}
		s.model.ProtoType = proto
		s.model.ProxyAddr = x
	}

	if l, err := net.Listen("tcp", s.config.AdminAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.ladmin = l

		x, err := utils.ResolveAddr("tcp", l.Addr().String(), s.config.HostAdmin)
		if err != nil {
			return err
		}
		s.model.AdminAddr = x
	}

	if s.config.JodisName != "" {
		c, err := models.NewClient(s.config.JodisName, s.config.JodisAddr, s.config.JodisTimeout)
		if err != nil {
			return err
		}
		s.xjodis = NewJodis(c, s.model, s.config.JodisCompatible != 0)
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

func (s *Proxy) Token() string {
	return s.token
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

func (s *Proxy) FillSlot(idx int, addr, from string, locked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	return s.router.FillSlot(idx, addr, from, locked)
}

func (s *Proxy) FillSlots(slots []*models.Slot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	for _, slot := range slots {
		idx, locked := slot.Id, slot.Locked
		addr := slot.BackendAddr
		from := slot.MigrateFrom
		if err := s.router.FillSlot(idx, addr, from, locked); err != nil {
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
			s.newSession(c)
		}
	}(s.lproxy)

	if seconds := s.config.BackendPingPeriod; seconds > 0 {
		go s.keepAlive(seconds)
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
	x := NewSessionSize(c, s.config.ProductAuth,
		s.config.SessionMaxBufSize, s.config.SessionMaxTimeout)
	x.SetKeepAlivePeriod(s.config.SessionKeepAlivePeriod)
	x.Start(s.router, s.config.SessionMaxPipeline)
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
