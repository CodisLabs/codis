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
	"github.com/wandoulabs/codis/pkg/utils/atomic2"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/codis/pkg/utils/rpc"
)

type Topom struct {
	mu sync.RWMutex

	xauth string
	model *models.Topom
	store models.Store

	intvl atomic2.Int64

	exit struct {
		C chan struct{}
	}

	online bool
	closed bool

	ladmin net.Listener
	redisp *RedisPool

	config *Config

	mappings [models.MaxSlotNum]*models.SlotMapping

	groups  map[int]*models.Group
	proxies map[string]*models.Proxy
	clients map[string]*proxy.ApiClient

	stats struct {
		servers map[string]*ServerStats
		proxies map[string]*ProxyStats
	}
	start sync.Once
}

var (
	ErrClosedTopom = errors.New("use of closed topom")
	ErrUpdateStore = errors.New("update store failed")
)

func NewWithConfig(store models.Store, config *Config) (*Topom, error) {
	s := &Topom{config: config, store: store}
	s.xauth = rpc.NewXAuth(config.ProductName, config.ProductAuth)
	s.model = &models.Topom{
		StartTime: time.Now().String(),
	}
	s.model.ProductName = config.ProductName
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()

	s.intvl.Set(1000)

	s.redisp = NewRedisPool(config.ProductAuth, time.Minute)

	s.exit.C = make(chan struct{})

	s.groups = make(map[int]*models.Group)
	s.proxies = make(map[string]*models.Proxy)
	s.clients = make(map[string]*proxy.ApiClient)

	s.stats.servers = make(map[string]*ServerStats)
	s.stats.proxies = make(map[string]*ProxyStats)

	if err := s.setup(); err != nil {
		s.Close()
		return nil, err
	}

	log.Infof("[%p] create new topom:\n%s", s, s.model.Encode())

	go s.serveAdmin()

	return s, nil
}

func (s *Topom) setup() error {
	if !utils.IsValidName(s.config.ProductName) {
		return errors.New("invalid product name, empty or using invalid character")
	}

	if l, err := net.Listen("tcp", s.config.AdminAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.ladmin = l
	}

	if addr, err := utils.ResolveAddr("tcp", s.ladmin.Addr().String()); err != nil {
		return err
	} else {
		s.model.AdminAddr = addr
	}

	if err := s.store.Acquire(s.config.ProductName, s.model); err != nil {
		return err
	} else {
		s.online = true
	}

	for i := 0; i < len(s.mappings); i++ {
		if m, err := s.store.LoadSlotMapping(i); err != nil {
			return err
		} else {
			s.mappings[i] = m
		}
	}

	if glist, err := s.store.ListGroup(); err != nil {
		return err
	} else {
		for _, g := range glist {
			s.groups[g.Id] = g
		}
		for _, g := range glist {
			for _, addr := range g.Servers {
				s.stats.servers[addr] = nil
			}
		}
	}

	if plist, err := s.store.ListProxy(); err != nil {
		return err
	} else {
		for _, p := range plist {
			c := proxy.NewApiClient(p.AdminAddr)
			c.SetXAuth(s.config.ProductName, s.config.ProductAuth, p.Token)
			s.proxies[p.Token] = p
			s.clients[p.Token] = c
		}
		for _, p := range plist {
			s.stats.servers[p.Token] = nil
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
	s.redisp.Close()

	defer s.store.Close()

	if !s.online {
		return nil
	} else {
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.online && !s.closed
}

func (s *Topom) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *Topom) GetInterval() int {
	return int(s.intvl.Get())
}

func (s *Topom) SetInterval(ms int) {
	ms = utils.MaxInt(ms, 0)
	ms = utils.MinInt(ms, 1000)
	s.intvl.Set(int64(ms))
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

func (s *Topom) StartDaemonRoutines() {
	s.start.Do(func() {
		go func() {
			for {
				wg := s.RefreshServerStats(time.Second)
				if wg != nil {
					wg.Wait()
				} else {
					return
				}
				time.Sleep(time.Second)
			}
		}()

		go func() {
			for {
				wg := s.RefreshProxyStats(time.Second)
				if wg != nil {
					wg.Wait()
				} else {
					return
				}
				time.Sleep(time.Second)
			}
		}()

		go func() {
			for !s.IsClosed() {
				if a := s.NextAction(); a != nil {
					if err := a.Do(); err != nil {
						log.WarnErrorf(err, "[%p] action on slot-[%d] failed", s, a.SlotId)
						time.Sleep(time.Second * 3)
					}
				} else {
					time.Sleep(time.Millisecond * 200)
				}
			}
		}()
	})
}
