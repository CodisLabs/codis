package topom

import (
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type Topom struct {
	mu sync.Mutex

	xauth string
	model *models.Topom

	exit struct {
		C chan struct{}
		sync.WaitGroup
	}
	online bool
	closed bool

	ladmin net.Listener

	config *Config

	store models.Store
	rwlck sync.RWMutex

	slots  []*models.SlotMapping
	groups map[int]*models.Group

	proxies map[string]*models.Proxy
	clients map[string]*proxy.ApiClient
}

var ErrClosedTopom = errors.New("use of closed topom")

func NewWithConfig(store models.Store, config *Config) (*Topom, error) {
	s := &Topom{store: store, config: config}
	s.xauth = rpc.NewXAuth(config.ProductName, config.ProductAuth)

	s.exit.C = make(chan struct{})

	s.model = &models.Topom{
		StartTime: time.Now().String(),
	}
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()

	if err := s.setup(); err != nil {
		s.Close()
		return nil, err
	}

	log.Infof("[%p] create new topom", s)

	s.exit.Add(1)
	go func() {
		defer s.exit.Done()
		// TODO
	}()

	go s.serveAdmin()

	return s, nil
}

func (s *Topom) setup() error {
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

	if err := s.store.Acquire(s.GetModel()); err != nil {
		return err
	}
	s.online = true

	s.slots = make([]*models.SlotMapping, models.MaxSlotNum)
	for i := 0; i < len(s.slots); i++ {
		if m, err := s.store.LoadSlotMapping(i); err != nil {
			return err
		} else {
			s.slots[i] = m
		}
	}
	if glist, err := s.store.ListGroup(); err != nil {
		return err
	} else {
		s.groups = make(map[int]*models.Group)
		for _, g := range glist {
			s.groups[g.Id] = g
		}
	}
	if plist, err := s.store.ListProxy(); err != nil {
		return err
	} else {
		s.proxies = make(map[string]*models.Proxy)
		s.clients = make(map[string]*proxy.ApiClient)
		for _, p := range plist {
			c := proxy.NewApiClient(p.AdminAddr)
			c.SetXAuth(s.config.ProductName, s.config.ProductAuth, p.Token)
			s.proxies[p.Token] = p
			s.clients[p.Token] = c
		}
	}
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

	s.ladmin.Close()

	defer s.store.Close()
	if !s.online {
		return nil
	} else {
		s.exit.Wait()
		return s.store.Release()
	}
}

func (s *Topom) GetXAuth() string {
	return s.xauth
}

func (s *Topom) GetModel() *models.Topom {
	return s.model
}

func (s *Topom) GetConfig() *Config {
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

func (s *Topom) serveAdmin() {
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
