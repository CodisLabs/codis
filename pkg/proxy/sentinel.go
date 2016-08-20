package proxy

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"

	redigo "github.com/garyburd/redigo/redis"
)

type Sentinel struct {
	context.Context
	Cancel context.CancelFunc

	router *Router
	config *Config
}

func NewSentinel(router *Router) *Sentinel {
	s := &Sentinel{
		router: router,
		config: router.config,
	}
	s.Context, s.Cancel = context.WithCancel(context.Background())
	return s
}

func (s *Sentinel) MasterName(gid int) string {
	return fmt.Sprintf("%s-%d", s.config.ProductName, gid)
}

func (s *Sentinel) dial(address string, timeout time.Duration) (redigo.Conn, error) {
	c, err := redigo.Dial("tcp", address, []redigo.DialOption{
		redigo.DialConnectTimeout(time.Second * 5),
		redigo.DialReadTimeout(timeout), redigo.DialWriteTimeout(time.Second * 5),
	}...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return c, nil
}

func (s *Sentinel) subscribe(ctx context.Context, address string) bool {
	c, err := s.dial(address, time.Minute*10)
	if err != nil {
		log.WarnErrorf(err, "sentinel %s dial failed", address)
		return false
	}
	defer c.Close()

	var eh = make(chan error, 1)
	go func() (err error) {
		defer func() {
			eh <- err
		}()
		if err := c.Send("SUBSCRIBE", "+switch-master"); err != nil {
			return errors.Trace(err)
		}
		if err := c.Flush(); err != nil {
			return errors.Trace(err)
		}
		for {
			switch r, err := redigo.Strings(c.Receive()); {
			case err != nil:
				return errors.Trace(err)
			case len(r) != 3:
				return errors.Errorf("invalid response = %v", r)
			case strings.HasPrefix(r[2], s.config.ProductName):
				return nil
			}
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-eh:
		if err == nil {
			return true
		}
		log.WarnErrorf(err, "sentinel %s subscribe failed", address)
	}
	return false
}

func (s *Sentinel) Subscribe(servers []string) bool {
	nctx, cancel := context.WithCancel(s.Context)
	defer cancel()
	var results = make(chan bool, len(servers))
	for i := range servers {
		go func() {
			results <- s.subscribe(nctx, servers[i])
		}()
	}
	for _ = range servers {
		select {
		case <-s.Context.Done():
			return false
		case changed := <-results:
			if changed {
				return true
			}
		}
	}
	return false
}

func (s *Sentinel) masters(ctx context.Context, address string, groupIds map[int]bool) map[int]string {
	c, err := s.dial(address, time.Second*10)
	if err != nil {
		log.WarnErrorf(err, "sentinel %s dial failed", address)
		return nil
	}
	defer c.Close()

	var mp = make(map[int]string)
	var eh = make(chan error, 1)
	go func() (err error) {
		defer func() {
			eh <- err
		}()
		for gid := range groupIds {
			switch r, err := redigo.Strings(c.Do("SENTINEL", "get-master-addr-by-name", s.MasterName(gid))); {
			case err != nil:
				return errors.Trace(err)
			case len(r) == 2:
				mp[gid] = fmt.Sprintf("%s:%s", r[0], r[1])
			case len(r) != 0:
				return errors.Errorf("invalid response = %v", r)
			}
		}
		return nil
	}()

	select {
	case <-ctx.Done():
	case err := <-eh:
		if err == nil {
			return mp
		}
		log.WarnErrorf(err, "sentinel %s get masters failed", address)
	}
	return nil
}

func (s *Sentinel) Masters(servers []string, groupIds map[int]bool) map[int]string {
	nctx, cancel := context.WithCancel(s.Context)
	defer cancel()
	var results = make(chan map[int]string, len(servers))
	for i := range servers {
		go func() {
			results <- s.masters(nctx, servers[i], groupIds)
		}()
	}
	var success int
	var masters = make(map[int]string)
	var counter = make(map[int]int)
	for _ = range servers {
		select {
		case <-s.Context.Done():
			return nil
		case mp := <-results:
			if mp != nil {
				for gid, addr := range mp {
					if masters[gid] == addr {
						counter[gid]++
						continue
					}
					switch counter[gid] {
					case 0:
						masters[gid] = addr
						counter[gid]++
					case 1:
						delete(masters, gid)
						fallthrough
					default:
						counter[gid]--
					}
				}
				success += 1
			}
		}
	}
	var limits = len(servers) / 2
	if success > limits {
		return masters
	}
	return nil
}

func (s *Sentinel) Monitor(servers []string) {
	for {
		time.Sleep(time.Millisecond * 250)
		select {
		case <-s.Context.Done():
			return
		default:
		}
		masters := s.Masters(servers, s.router.GetGroupIds())
		if masters != nil {
			s.router.SwitchMasters(masters)
			s.Subscribe(servers)
			continue
		}
		select {
		case <-s.Context.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}
