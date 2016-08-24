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
	return NewSentinelAuth(product, "")
}

func NewSentinelAuth(product, auth string) *Sentinel {
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

func (s *Sentinel) WatchNode(gid int) string {
	return fmt.Sprintf("%s-%d", s.product, gid)
}

func (s *Sentinel) hasSameProduct(node string) bool {
	if strings.LastIndexByte(node, '-') != len(s.product) {
		return false
	}
	return strings.HasPrefix(node, s.product)
}

func (s *Sentinel) newSentinelClient(sentinel string, timeout time.Duration) (*Client, error) {
	return NewClient(sentinel, "", timeout)
}

func (s *Sentinel) subscribe(ctx context.Context, sentinel string, timeout time.Duration) (bool, error) {
	c, err := s.newSentinelClient(sentinel, timeout)
	if err != nil {
		return false, err
	}
	defer c.Close()

	var exit = make(chan error, 1)

	go func() (err error) {
		defer func() {
			exit <- err
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
			switch node, err := redigo.String(reply[2], nil); {
			case err != nil:
				return errors.Trace(err)
			case s.hasSameProduct(node):
				return nil
			}
		}
	}()

	select {
	case <-ctx.Done():
		return false, nil
	case err := <-exit:
		if err != nil {
			e, ok := errors.Cause(err).(*net.OpError)
			if ok && e.Timeout() {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
}

func (s *Sentinel) Subscribe(timeout time.Duration, sentinels ...string) bool {
	nctx, cancel := context.WithCancel(s.Context)
	defer cancel()

	var results = make(chan bool, len(sentinels))

	for i := range sentinels {
		go func(sentinel string) {
			notified, err := s.subscribe(nctx, sentinel, timeout)
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
		case <-s.Context.Done():
			return false
		case notified := <-results:
			if notified {
				return true
			}
		}
	}
	return false
}

func (s *Sentinel) isRoleMaster(addr string) (bool, error) {
	c, err := NewClient(addr, s.auth, time.Second*5)
	if err != nil {
		return false, err
	}
	defer c.Close()
	role, err := c.Role()
	if err != nil {
		return false, err
	}
	return role == "MASTER", nil
}

func (s *Sentinel) masters(ctx context.Context, sentinel string, timeout time.Duration, groups map[int]bool) (map[int]string, error) {
	c, err := s.newSentinelClient(sentinel, timeout)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	masters := make(map[int]string)

	var exit = make(chan error, 1)

	go func() (err error) {
		defer func() {
			exit <- err
		}()
		for gid := range groups {
			switch reply, err := c.Do("SENTINEL", "get-master-addr-by-name", s.WatchNode(gid)); {
			case err != nil:
				return err
			case reply != nil:
				r, err := redigo.Strings(reply, nil)
				if err != nil {
					return errors.Trace(err)
				}
				if len(r) != 2 {
					return errors.Errorf("invalid response = %v", r)
				}
				addr := fmt.Sprintf("%s:%s", r[0], r[1])
				switch yes, err := s.isRoleMaster(addr); {
				case err != nil:
					log.WarnErrorf(err, "sentinel get role of %s failed", addr)
				case yes:
					masters[gid] = addr
				}
			}
		}
		return nil
	}()

	select {
	case <-ctx.Done():
		return nil, nil
	case err := <-exit:
		if err != nil {
			return nil, err
		}
		return masters, nil
	}
}

func (s *Sentinel) FlushConfig(sentinel string) error {
	c, err := NewClient(sentinel, "", time.Second*5)
	if err != nil {
		return err
	}
	defer c.Close()
	if _, err := c.Do("SENTINEL", "flushconfig"); err != nil {
		return err
	}
	return nil
}

func (s *Sentinel) Masters(groups map[int]bool, timeout time.Duration, sentinels ...string) map[int]string {
	nctx, cancel := context.WithCancel(s.Context)
	defer cancel()

	var results = make(chan map[int]string, len(sentinels))

	for i := range sentinels {
		go func(sentinel string) {
			masters, err := s.masters(nctx, sentinel, timeout, groups)
			if err != nil {
				log.WarnErrorf(err, "sentinel %s masters failed", sentinel)
			}
			results <- masters
		}(sentinels[i])
	}

	masters := make(map[int]string)
	counter := make(map[int]int)

	for _ = range sentinels {
		select {
		case <-s.Context.Done():
			return masters
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

func (s *Sentinel) monitor(ctx context.Context, sentinel string, timeout time.Duration, masters map[int]string, quorum int) error {
	c, err := s.newSentinelClient(sentinel, timeout)
	if err != nil {
		return err
	}
	defer c.Close()

	var exit = make(chan error, 1)

	go func() (err error) {
		defer func() {
			exit <- err
		}()
		for gid, addr := range masters {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return errors.Trace(err)
			}
			node := s.WatchNode(gid)
			switch reply, err := c.Do("SENTINEL", "get-master-addr-by-name", node); {
			case err != nil:
				return err
			case reply != nil:
				_, err := c.Do("SENTINEL", "remove", node)
				if err != nil {
					return err
				}
			}
			switch _, err := c.Do("SENTINEL", "monitor", node, host, port, quorum); {
			case err != nil:
				return err
			case s.auth != "":
				_, err := c.Do("SENTINEL", "set", node, "auth-pass", s.auth)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-exit:
		return err
	}
}

func (s *Sentinel) Monitor(masters map[int]string, quorum int, timeout time.Duration, sentinels ...string) error {
	nctx, cancel := context.WithCancel(s.Context)
	defer cancel()

	var results = make(chan error, len(sentinels))

	for i := range sentinels {
		go func(sentinel string) {
			err := s.monitor(nctx, sentinel, timeout, masters, quorum)
			if err != nil {
				log.WarnErrorf(err, "sentinel %s monitor failed", sentinel)
			}
			results <- err
		}(sentinels[i])
	}

	for _ = range sentinels {
		select {
		case <-s.Context.Done():
			return nil
		case err := <-results:
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Sentinel) unmonitor(ctx context.Context, sentinel string, timeout time.Duration, groups map[int]bool) error {
	c, err := s.newSentinelClient(sentinel, time.Second*5)
	if err != nil {
		return err
	}
	defer c.Close()

	var exit = make(chan error, 1)

	go func() (err error) {
		defer func() {
			exit <- err
		}()
		for gid := range groups {
			node := s.WatchNode(gid)
			switch reply, err := c.Do("SENTINEL", "get-master-addr-by-name", node); {
			case err != nil:
				return err
			case reply != nil:
				_, err := c.Do("SENTINEL", "remove", node)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-exit:
		return err
	}
}

func (s *Sentinel) Unmonitor(groups map[int]bool, timeout time.Duration, sentinels ...string) error {
	nctx, cancel := context.WithCancel(s.Context)
	defer cancel()

	var results = make(chan error, len(sentinels))

	for i := range sentinels {
		go func(sentinel string) {
			err := s.unmonitor(nctx, sentinel, timeout, groups)
			if err != nil {
				log.WarnErrorf(err, "sentinel %s unmonitor failed", sentinel)
			}
			results <- err
		}(sentinels[i])
	}

	for _ = range sentinels {
		select {
		case <-s.Context.Done():
			return nil
		case err := <-results:
			if err != nil {
				return err
			}
		}
	}
	return nil
}
