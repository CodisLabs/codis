package redis

import (
	"fmt"
	"net"
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

	product, auth string
}

func NewSentinel(product string) *Sentinel {
	return NewSentinelWithAuth(product, "")
}

func NewSentinelWithAuth(product, auth string) *Sentinel {
	s := &Sentinel{product: product, auth: auth}
	s.Context, s.Cancel = context.WithCancel(context.Background())
	return s
}

func (s *Sentinel) IsCancelled() bool {
	select {
	case <-s.Context.Done():
		return true
	default:
		return false
	}
}

func (s *Sentinel) AfterSeconds(n int) {
	if n == 0 {
		return
	}
	select {
	case <-s.Context.Done():
	case <-time.After(time.Second * time.Duration(n)):
	}
}

func (s *Sentinel) MasterName(gid int) string {
	return fmt.Sprintf("%s-%d", s.product, gid)
}

func (s *Sentinel) newSentinelClient(sentinel string, timeout time.Duration) (*Client, error) {
	return NewClient(sentinel, "", timeout)
}

func (s *Sentinel) SubscribeOne(ctx context.Context, sentinel string) (bool, error) {
	c, err := s.newSentinelClient(sentinel, time.Minute*30)
	if err != nil {
		return false, err
	}
	defer c.Close()

	var ech = make(chan error, 1)
	go func() (err error) {
		defer func() {
			ech <- err
		}()
		if err := c.Flush("SUBSCRIBE", "+switch-master"); err != nil {
			return err
		}
		for {
			reply, err := redigo.Values(c.Receive())
			if err != nil {
				return errors.Trace(err)
			}
			if msg, err := redigo.String(reply[0], nil); err != nil {
				return errors.Trace(err)
			} else if msg != "message" {
				continue
			}
			if evt, err := redigo.String(reply[1], nil); err != nil {
				return errors.Trace(err)
			} else if evt != "+switch-master" {
				continue
			}
			if len(reply) != 3 {
				return errors.Errorf("invalid response = %v", reply)
			}
			name, err := redigo.String(reply[2], nil)
			if err != nil {
				return errors.Trace(err)
			}
			if strings.HasPrefix(name, s.product) {
				return nil
			}
		}
	}()

	select {
	case <-ctx.Done():
		return false, nil
	case err := <-ech:
		if err != nil {
			return false, err
		}
		return true, nil
	}
}

func (s *Sentinel) SubscribeMulti(ctx context.Context, sentinels []string) bool {
	if len(sentinels) == 0 {
		return false
	}
	nctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var results = make(chan bool, len(sentinels))
	for i := range sentinels {
		go func(sentinel string) {
			notified, err := s.SubscribeOne(nctx, sentinel)
			if err != nil {
				log.WarnErrorf(err, "sentinel %s subscribe failed", sentinel)
			}
			if notified {
				log.Warnf("sentinel %s event +switch-master", sentinel)
			}
			results <- notified
		}(sentinels[i])
	}

	var majority = 1 + len(sentinels)/2

	for i := 0; i < majority; i++ {
		select {
		case <-ctx.Done():
			return false
		case notified := <-results:
			if notified {
				return true
			}
		}
	}
	return false
}

func (s *Sentinel) getServerRole(addr string) (string, error) {
	c, err := NewClient(addr, s.auth, time.Second*5)
	if err != nil {
		return "", err
	}
	defer c.Close()
	return c.Role()
}

func (s *Sentinel) MastersOne(ctx context.Context, sentinel string, groupIds map[int]bool) (map[int]string, error) {
	c, err := s.newSentinelClient(sentinel, time.Second*10)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	var gmp = make(map[int]string)
	var ech = make(chan error, 1)
	go func() (err error) {
		defer func() {
			ech <- err
		}()
		for gid := range groupIds {
			reply, err := c.Do("SENTINEL", "get-master-addr-by-name", s.MasterName(gid))
			if err != nil {
				return errors.Trace(err)
			}
			if reply == nil {
				continue
			}
			r, err := redigo.Strings(reply, nil)
			if err != nil {
				return errors.Trace(err)
			}
			if len(r) != 2 {
				return errors.Errorf("invalid response = %v", r)
			}
			var addr = fmt.Sprintf("%s:%s", r[0], r[1])
			if role, err := s.getServerRole(addr); err != nil {
				log.WarnErrorf(err, "sentinel get role of %s failed", addr)
			} else if role == "MASTER" {
				gmp[gid] = addr
			}
		}
		return nil
	}()

	select {
	case <-ctx.Done():
		return nil, nil
	case err := <-ech:
		if err != nil {
			return nil, err
		}
		return gmp, nil
	}
}

func (s *Sentinel) MastersMulti(ctx context.Context, sentinels []string, groupIds map[int]bool) map[int]string {
	if len(sentinels) == 0 || len(groupIds) == 0 {
		return map[int]string{}
	}
	nctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var results = make(chan map[int]string, len(sentinels))
	for i := range sentinels {
		go func(sentinel string) {
			m, err := s.MastersOne(nctx, sentinel, groupIds)
			if err != nil {
				log.WarnErrorf(err, "sentinel %s masters failed", sentinel)
			}
			results <- m
		}(sentinels[i])
	}

	var masters = make(map[int]string)
	var counter = make(map[int]int)
	for i := 0; i < len(sentinels); i++ {
		select {
		case <-ctx.Done():
			return nil
		case m := <-results:
			if m != nil {
				for gid, addr := range m {
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
			}
		}
	}
	return masters
}

func (s *Sentinel) MonitorOne(sentinel string, masters map[int]string, quorum int, overwrite bool) error {
	c, err := s.newSentinelClient(sentinel, time.Second*10)
	if err != nil {
		return err
	}
	defer c.Close()
	for gid, master := range masters {
		host, port, err := net.SplitHostPort(master)
		if err != nil {
			return errors.Trace(err)
		}
		name := s.MasterName(gid)
		if overwrite {
			_, err := c.Do("SENTINEL", "remove", name)
			if err != nil {
				return err
			}
		}
		if _, err := redigo.String(c.Do("SENTINEL", "monitor", name, host, port, quorum)); err != nil {
			return errors.Trace(err)
		}
		if s.auth == "" {
			continue
		}
		if _, err := redigo.String(c.Do("SENTINEL", "set", name, "auth-pass", s.auth)); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (s *Sentinel) Monitor(sentinels []string, masters map[int]string, quorum int, overwrite bool) error {
	if len(sentinels) == 0 {
		return nil
	}

	var results = make(chan error, len(sentinels))
	for i := range sentinels {
		go func(sentinel string) {
			err := s.MonitorOne(sentinel, masters, quorum, overwrite)
			if err != nil {
				log.WarnErrorf(err, "sentinel %s monitor failed", sentinel)
			}
			results <- err
		}(sentinels[i])
	}

	for i := 0; i < len(sentinels); i++ {
		if err := <-results; err != nil {
			return err
		}
	}
	return nil
}

func (s *Sentinel) RemoveMonitor(sentinel string, groups ...int) error {
	c, err := s.newSentinelClient(sentinel, time.Second*10)
	if err != nil {
		return err
	}
	defer c.Close()
	for gid := range groups {
		_, err := c.Do("SENTINEL", "remove", s.MasterName(gid))
		if err != nil {
			return err
		}
	}
	return nil
}
