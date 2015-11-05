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

	exit struct {
		C chan struct{}
	}

	online bool
	closed bool

	ladmin net.Listener
	redisp *RedisPool

	config *Config

	mappings [models.MaxSlotNum]*models.SlotMapping

	groups map[int]*models.Group
	mlocks map[int]*atomic2.Int64

	proxies map[string]*models.Proxy
	clients map[string]*proxy.ApiClient

	stats struct {
		servers map[string]*ServerStats
		proxies map[string]*ProxyStats
	}
	start sync.Once

	action struct {
		interval atomic2.Int64
		disabled atomic2.Bool

		progress struct {
			remain atomic2.Int64
			failed atomic2.Bool
		}

		notify chan bool
	}
}

var ErrClosedTopom = errors.New("use of closed topom")

func New(store models.Store, config *Config) (*Topom, error) {
	s := &Topom{config: config, store: store}
	s.xauth = rpc.NewXAuth(config.ProductName, config.ProductAuth)
	s.model = &models.Topom{
		StartTime: time.Now().String(),
	}
	s.model.ProductName = config.ProductName
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()

	s.action.interval.Set(1000)
	s.action.notify = make(chan bool, 1)

	s.redisp = NewRedisPool(config.ProductAuth, time.Second*10)

	s.exit.C = make(chan struct{})

	s.groups = make(map[int]*models.Group)
	s.mlocks = make(map[int]*atomic2.Int64)

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

	if err := s.store.Acquire(s.model); err != nil {
		log.ErrorErrorf(err, "[%p] acquire lock for %s failed", s, s.config.ProductName)
		return errors.Errorf("store: acquire lock for %s failed", s.config.ProductName)
	} else {
		s.online = true
	}

	for i := 0; i < len(s.mappings); i++ {
		if m, err := s.store.LoadSlotMapping(i); err != nil {
			log.ErrorErrorf(err, "[%p] load slot-[%d] failed", s, i)
			return errors.Errorf("store: load slot-[%d] failed", i)
		} else {
			if m == nil {
				m = &models.SlotMapping{Id: i}
			}
			s.mappings[i] = m
		}
	}

	if glist, err := s.store.ListGroup(); err != nil {
		log.ErrorErrorf(err, "[%p] list group failed", s)
		return errors.Errorf("store: list group failed")
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
		log.ErrorErrorf(err, "[%p] list proxy failed", s)
		return errors.Errorf("store: list proxy failed")
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
	}
	if err := s.store.Release(false); err != nil {
		log.ErrorErrorf(err, "[%p] release lock for %s failed", s, s.config.ProductName)
		return errors.Errorf("store: release lock for %s failed", s.config.ProductName)
	} else {
		return nil
	}
}

func (s *Topom) GetXAuth() string {
	return s.xauth
}

func (s *Topom) GetModel() *models.Topom {
	return s.model
}

func (s *Topom) GetStats() *Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := &Stats{}
	stats.Online = s.online
	stats.Closed = s.closed

	stats.Slots = s.getSlotMappings()

	stats.Group.Models = s.getGroupModels()
	stats.Group.Stats = make(map[string]*ServerStats)
	for k, v := range s.stats.servers {
		stats.Group.Stats[k] = v
	}

	stats.Proxy.Models = s.getProxyModels()
	stats.Proxy.Stats = make(map[string]*ProxyStats)
	for k, v := range s.stats.proxies {
		stats.Proxy.Stats[k] = v
	}

	stats.Action.Interval = s.action.interval.Get()
	stats.Action.Disabled = s.action.disabled.Get()
	stats.Action.Progress.Remain = s.action.progress.remain.Get()
	stats.Action.Progress.Failed = s.action.progress.failed.Get()
	return stats
}

type Stats struct {
	Online bool `json:"online"`
	Closed bool `json:"closed"`

	Slots []*models.SlotMapping `json:"slots"`
	Group struct {
		Models []*models.Group         `json:"models"`
		Stats  map[string]*ServerStats `json:"stats"`
	} `json:"group"`
	Proxy struct {
		Models []*models.Proxy        `json:"models"`
		Stats  map[string]*ProxyStats `json:"stats"`
	} `json:"proxy"`

	Action struct {
		Interval int64 `json:"interval"`
		Disabled bool  `json:"disabled"`

		Progress struct {
			Remain int64 `json:"remain"`
			Failed bool  `json:"failed"`
		} `json:"migration"`
	} `json:"action"`
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

func (s *Topom) GetActionInterval() int {
	return int(s.action.interval.Get())
}

func (s *Topom) SetActionInterval(ms int) {
	ms = utils.MaxInt(ms, 0)
	ms = utils.MinInt(ms, 1000)
	s.action.interval.Set(int64(ms))
	log.Infof("[%p] set action interval = %d", s, ms)
}

func (s *Topom) GetActionDisabled() bool {
	return s.action.disabled.Get()
}

func (s *Topom) SetActionDisabled(value bool) {
	s.action.disabled.Set(value)
	log.Infof("[%p] set action disabled = %t", s, value)
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
			for !s.IsClosed() {
				if wg := s.RefreshServerStats(time.Second); wg != nil {
					wg.Wait()
				}
				time.Sleep(time.Second)
			}
		}()

		go func() {
			for !s.IsClosed() {
				if wg := s.RefreshProxyStats(time.Second); wg != nil {
					wg.Wait()
				}
				time.Sleep(time.Second)
			}
		}()

		go func() {
			var ticker = time.NewTicker(time.Second)
			defer ticker.Stop()
			for !s.IsClosed() {
				var slotId int = -1
				if !s.GetActionDisabled() {
					slotId = s.NextActionSlotId()
				}
				if slotId < 0 {
					select {
					case <-s.exit.C:
						return
					case <-ticker.C:
					case <-s.action.notify:
					}
				} else if err := s.ProcessAction(slotId); err != nil {
					s.action.progress.failed.Set(true)
					log.WarnErrorf(err, "[%p] action on slot-[%d] failed", s, slotId)
					time.Sleep(time.Second * 3)
				} else {
					s.action.progress.remain.Set(0)
					s.action.progress.failed.Set(false)
					log.Infof("[%p] action on slot-[%d] completed", s, slotId)
				}
			}
		}()
	})
}
