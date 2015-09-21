package proxy

import (
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/router"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type Proxy struct {
	mu sync.Mutex

	token string
	xauth string
	model *models.Proxy

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
	s := &Proxy{config: config}
	s.token = rpc.NewToken()
	s.xauth = rpc.NewXAuth(config.ProductName, config.ProductAuth, s.token)

	s.router = router.NewWithAuth(config.ProductAuth)
	s.init.C = make(chan struct{})
	s.exit.C = make(chan struct{})

	s.model = &models.Proxy{
		Token: s.token, StartTime: time.Now().String(),
	}
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()

	if err := s.setup(); err != nil {
		s.Close()
		return nil, err
	}

	log.Infof("[%p] create new proxy", s)

	go s.serveAdmin()
	go s.serveProxy()

	return s, nil
}

func (s *Proxy) setup() error {
	if l, err := net.Listen(s.config.ProtoType, s.config.ProxyAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.lproxy = l
	}

	if addr, err := utils.ResolveAddr(s.config.ProtoType, s.config.ProxyAddr); err != nil {
		return err
	} else {
		s.model.ProtoType = s.config.ProtoType
		s.model.ProxyAddr = addr
	}

	if l, err := net.Listen("tcp", s.config.AdminAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.ladmin = l
	}

	if addr, err := utils.ResolveAddr("tcp", s.config.AdminAddr); err != nil {
		return err
	} else {
		s.model.AdminAddr = addr
	}

	if !utils.IsValidName(s.config.ProductName) {
		return errors.New("invalid product name, empty or using invalid character")
	}
	return nil
}

func (s *Proxy) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.Trace(ErrClosedProxy)
	}
	if s.online {
		return nil
	}
	s.online = true
	close(s.init.C)
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

	s.ladmin.Close()
	s.lproxy.Close()
	s.router.Close()
	return nil
}

func (s *Proxy) GetToken() string {
	return s.token
}

func (s *Proxy) GetXAuth() string {
	return s.xauth
}

func (s *Proxy) GetModel() *models.Proxy {
	return s.model
}

func (s *Proxy) GetConfig() *Config {
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

func (s *Proxy) GetSlots() []*models.Slot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.router.GetSlots()
}

func (s *Proxy) FillSlot(slots ...*models.Slot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.Trace(ErrClosedProxy)
	}
	for _, slot := range slots {
		if err := s.router.FillSlot(slot.Id, slot.BackendAddr, slot.MigrateFrom, slot.Locked); err != nil {
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

	log.Infof("[%p] admin start service on %s", s, s.ladmin.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) {
		h := http.NewServeMux()
		h.Handle("/", newApiServer(s))
		hs := &http.Server{Handler: h}
		eh <- hs.Serve(l)
	}(s.ladmin)

	select {
	case <-s.exit.C:
		log.Infof("[%p] admin shutdown", s)
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

	log.Infof("[%p] proxy start service on %s", s, s.lproxy.Addr())

	ch := make(chan net.Conn, 4096)
	go func() {
		for c := range ch {
			s.newSession(c)
		}
	}()

	eh := make(chan error, 1)
	go func(l net.Listener) {
		defer close(ch)
		for {
			c, err := s.acceptConn(l)
			if err != nil {
				eh <- err
				return
			} else {
				ch <- c
			}
		}
	}(s.lproxy)

	go s.keepAlive()

	select {
	case <-s.exit.C:
		log.Infof("[%p] proxy shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] proxy exit on error", s)
	}
}

func (s *Proxy) keepAlive() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var tick int
	for {
		select {
		case <-s.exit.C:
			return
		case <-ticker.C:
			if maxTick := s.config.BackendPingPeriod; maxTick != 0 {
				if tick++; tick >= maxTick {
					tick = 0
					s.router.KeepAlive()
				}
			}
		}
	}
}

func (s *Proxy) newSession(c net.Conn) {
	x := router.NewSessionSize(c, s.config.ProductAuth,
		s.config.SessionMaxBufSize, s.config.SessionMaxTimeout)
	x.SetKeepAlivePeriod(s.config.SessionKeepAlivePeriod)
	go x.Serve(s.router, s.config.SessionMaxPipeline)
}

func (s *Proxy) acceptConn(l net.Listener) (net.Conn, error) {
	var delay time.Duration
	for {
		c, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if delay == 0 {
					delay = time.Millisecond * 10
				} else {
					delay = delay * 2
					if maxDelay := time.Second; delay > maxDelay {
						delay = maxDelay
					}
				}
				log.WarnErrorf(err, "[%p] proxy accept new connection failed", s)
				time.Sleep(delay)
				continue
			}
		}
		return c, err
	}
}
