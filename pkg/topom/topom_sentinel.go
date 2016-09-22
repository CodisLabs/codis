// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/redis"
	"github.com/CodisLabs/codis/pkg/utils/sync2"
)

func (s *Topom) AddSentinel(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	if addr == "" {
		return errors.Errorf("invalid sentinel address")
	}
	p := ctx.sentinel

	for _, x := range p.Servers {
		if x == addr {
			return errors.Errorf("sentinel-[%s] already exists", addr)
		}
	}

	sentinel := redis.NewSentinel(s.config.ProductName)
	if err := sentinel.FlushConfig(addr); err != nil {
		return err
	}
	defer s.dirtySentinelCache()

	p.Servers = append(p.Servers, addr)
	p.OutOfSync = true
	return s.storeUpdateSentinel(p)
}

func (s *Topom) DelSentinel(addr string, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}

	if addr == "" {
		return errors.Errorf("invalid sentinel address")
	}
	p := ctx.sentinel

	var slice []string
	for _, x := range p.Servers {
		if x != addr {
			slice = append(slice, x)
		}
	}
	if len(slice) == len(p.Servers) {
		return errors.Errorf("sentinel-[%s] not found", addr)
	}
	defer s.dirtySentinelCache()

	p.OutOfSync = true
	if err := s.storeUpdateSentinel(p); err != nil {
		return err
	}

	sentinel := redis.NewSentinelAuth(s.config.ProductName, s.config.ProductAuth)
	if err := sentinel.Unmonitor(ctx.getGroupIds(), time.Second*5, addr); err != nil {
		log.WarnErrorf(err, "remove sentinel %s failed", addr)
		if !force {
			return errors.Errorf("remove sentinel %s failed", addr)
		}
	}

	p.Servers = slice
	return s.storeUpdateSentinel(p)
}

func (s *Topom) SwitchMasters(masters map[int]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedTopom
	}
	s.ha.masters = masters
	return nil
}

func (s *Topom) rewatchSentinels(servers []string) {
	getGroupIds := func() map[int]bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		ctx, err := s.newContext()
		if err != nil {
			return nil
		}
		return ctx.getGroupIds()
	}

	if s.ha.monitor != nil {
		s.ha.monitor.Cancel()
		s.ha.monitor = nil
	}

	if len(servers) == 0 {
		s.ha.masters = nil
	} else {
		s.ha.monitor = redis.NewSentinel(s.config.ProductName)
		go func(p *redis.Sentinel) {
			refetch := make(chan time.Duration)
			go func() {
				defer func() {
					close(refetch)
				}()
				for !p.IsCancelled() {
					refetch <- 0
					refetch <- time.Second * 10
					timeout := time.Minute * 5
					retryAt := time.Now().Add(time.Second * 30)
					if !p.Subscribe(timeout, servers...) {
						for time.Now().Before(retryAt) && !p.IsCancelled() {
							time.Sleep(time.Second)
						}
					}
				}
			}()
			go func() {
				defer func() {
					for _ = range refetch {
					}
				}()
				for d := range refetch {
					if d != 0 {
						time.Sleep(d)
					}
					timeout := time.Second * 10
					masters := p.Masters(getGroupIds(), timeout, servers...)
					if p.IsCancelled() {
						return
					}
					s.SwitchMasters(masters)
				}
			}()
		}(s.ha.monitor)
	}
	log.Warnf("rewatch sentinels = %v", servers)
}

func (s *Topom) ResyncSentinels() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return err
	}
	defer s.dirtySentinelCache()

	p := ctx.sentinel
	p.OutOfSync = true
	if err := s.storeUpdateSentinel(p); err != nil {
		return err
	}

	sentinel := redis.NewSentinelAuth(s.config.ProductName, s.config.ProductAuth)
	if err := sentinel.Monitor(ctx.getGroupMasters(), s.config.SentinelQuorum, time.Second*5, p.Servers...); err != nil {
		log.WarnErrorf(err, "resync sentinels failed")
		return err
	}
	s.rewatchSentinels(p.Servers)

	var fut sync2.Future
	for _, p := range ctx.proxy {
		fut.Add()
		go func(p *models.Proxy) {
			err := s.newProxyClient(p).SetSentinels(ctx.sentinel)
			if err != nil {
				log.ErrorErrorf(err, "proxy-[%s] resync sentinel failed", p.Token)
			}
			fut.Done(p.Token, err)
		}(p)
	}
	for t, v := range fut.Wait() {
		switch err := v.(type) {
		case error:
			if err != nil {
				return errors.Errorf("proxy-[%s] sentinel failed", t)
			}
		}
	}

	p.OutOfSync = false
	return s.storeUpdateSentinel(p)
}
