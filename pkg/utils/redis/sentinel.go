// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"

	redigo "github.com/garyburd/redigo/redis"
)

type Sentinel struct {
	context.Context
	Cancel context.CancelFunc

	Product, Auth string

	LogFunc func(format string, arguments ...interface{})
	ErrFunc func(err error, format string, arguments ...interface{})
}

func NewSentinel(product, auth string) *Sentinel {
	s := &Sentinel{Product: product, Auth: auth}
	s.Context, s.Cancel = context.WithCancel(context.Background())
	return s
}

func (s *Sentinel) IsCanceled() bool {
	select {
	case <-s.Context.Done():
		return true
	default:
		return false
	}
}

func (s *Sentinel) NodeName(gid int) string {
	return fmt.Sprintf("%s-%d", s.Product, gid)
}

func (s *Sentinel) isSameProduct(name string) bool {
	if strings.LastIndexByte(name, '-') != len(s.Product) {
		return false
	}
	return strings.HasPrefix(name, s.Product)
}

func (s *Sentinel) printf(format string, arguments ...interface{}) {
	if s.LogFunc != nil {
		s.LogFunc(format, arguments...)
	}
}

func (s *Sentinel) errorf(err error, format string, arguments ...interface{}) {
	if s.ErrFunc != nil {
		s.ErrFunc(err, format, arguments...)
	}
}

func (s *Sentinel) subscribeCommand(client *Client, sentinel string, onSubscribed func(string)) error {
	var channels = []interface{}{"+switch-master"}
	if err := client.Flush("SUBSCRIBE", channels...); err != nil {
		return errors.Trace(err)
	}
	for _, sub := range channels {
		values, err := redigo.Values(client.Receive())
		if err != nil {
			return errors.Trace(err)
		} else if len(values) != 3 {
			return errors.Errorf("invalid response = %v", values)
		}
		s, err := redigo.Strings(values[:2], nil)
		if err != nil || s[0] != "subscribe" || s[1] != sub.(string) {
			return errors.Errorf("invalid response = %v", values)
		}
	}
	if onSubscribed != nil {
		onSubscribed(sentinel)
	}
	for {
		values, err := redigo.Values(client.Receive())
		if err != nil {
			return errors.Trace(err)
		} else if len(values) < 2 {
			return errors.Errorf("invalid response = %v", values)
		}
		message, err := redigo.Strings(values, nil)
		if err != nil || message[0] != "message" {
			return errors.Errorf("invalid response = %v", values)
		}
		s.printf("sentinel-[%s] subscribe event %v", sentinel, message)

		switch message[1] {
		case "+switch-master":
			if len(message) != 3 {
				return errors.Errorf("invalid response = %v", values)
			}
			if s.isSameProduct(message[2]) {
				return nil
			}
		}
	}
}

func (s *Sentinel) subscribeInstance(ctx context.Context, sentinel string, timeout time.Duration, onSubscribed func(string)) (bool, error) {
	c, err := NewClientNoAuth(sentinel, timeout)
	if err != nil {
		return false, err
	}
	defer c.Close()

	var exit = make(chan error, 1)

	go func() {
		exit <- s.subscribeCommand(c, sentinel, onSubscribed)
	}()

	select {
	case <-ctx.Done():
		return false, nil
	case err := <-exit:
		if err != nil {
			return false, err
		}
		return true, nil
	}
}

func (s *Sentinel) Subscribe(timeout time.Duration, onMajoritySubscribed func(), sentinels ...string) bool {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	timeout += time.Second * 5
	results := make(chan bool, len(sentinels))

	var majority = 1 + len(sentinels)/2

	var count atomic2.Int64

	onSubscribed := func(sentinel string) {
		var total = int(count.Incr())
		if total == majority && onMajoritySubscribed != nil {
			onMajoritySubscribed()
		}
	}
	for i := range sentinels {
		go func(sentinel string) {
			notified, err := s.subscribeInstance(cntx, sentinel, timeout, onSubscribed)
			if err != nil {
				s.errorf(err, "sentinel-[%s] subscribe failed", sentinel)
			}
			results <- notified
		}(sentinels[i])
	}

	for alive := len(sentinels); ; alive-- {
		if alive < majority {
			if cntx.Err() == nil {
				s.printf("sentinel subscribe lost majority (%d/%d)", alive, len(sentinels))
			}
			return false
		}
		select {
		case <-cntx.Done():
			if cntx.Err() == context.Canceled {
				s.printf("sentinel subscribe canceled (%v)", cntx.Err())
			}
			return false
		case notified := <-results:
			if !notified {
				continue
			}
			s.printf("sentinel subscribe notified +switch-master")
			return true
		}
	}
}

func (s *Sentinel) existsCommand(client *Client, name string) (bool, error) {
	if reply, err := client.Do("SENTINEL", "get-master-addr-by-name", name); err != nil {
		return false, errors.Trace(err)
	} else {
		return reply != nil, nil
	}
}

func (s *Sentinel) masterCommand(client *Client, name string) (map[string]string, error) {
	if exists, err := s.existsCommand(client, name); err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}
	m, err := redigo.StringMap(client.Do("SENTINEL", "master", name))
	if err != nil {
		return nil, errors.Trace(err)
	}
	return m, nil
}

func (s *Sentinel) slavesCommand(client *Client, name string) ([]map[string]string, error) {
	if exists, err := s.existsCommand(client, name); err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}
	values, err := redigo.Values(client.Do("SENTINEL", "slaves", name))
	if err != nil {
		return nil, errors.Trace(err)
	}
	var slaves []map[string]string
	for i := range values {
		m, err := redigo.StringMap(values[i], nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
		slaves = append(slaves, m)
	}
	return slaves, nil
}

func (s *Sentinel) mastersCommand(client *Client) ([]map[string]string, error) {
	values, err := redigo.Values(client.Do("SENTINEL", "masters"))
	if err != nil {
		return nil, errors.Trace(err)
	}
	var masters []map[string]string
	for i := range values {
		m, err := redigo.StringMap(values[i], nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if s.isSameProduct(m["name"]) {
			masters = append(masters, m)
		}
	}
	return masters, nil
}

func (s *Sentinel) mastersInstance(ctx context.Context, sentinel string, groups map[int]bool, timeout time.Duration) (map[int]*SentinelMaster, error) {
	c, err := NewClientNoAuth(sentinel, timeout)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	masters := make(map[int]*SentinelMaster)

	var exit = make(chan error, 1)

	go func() (err error) {
		defer func() {
			exit <- err
		}()
		for gid := range groups {
			var name = s.NodeName(gid)
			master, err := s.masterCommand(c, name)
			if err != nil {
				return err
			} else if master == nil {
				continue
			}
			epoch, err := strconv.ParseInt(master["config-epoch"], 10, 64)
			if err != nil {
				s.printf("sentinel-[%s] masters parse config-epoch failed, %s", sentinel, err)
				continue
			}
			ip, port := master["ip"], master["port"]
			if ip == "" || port == "" {
				s.printf("sentinel-[%s] masters parse ip:port failed, '%s:%s'", sentinel, ip, port)
				continue
			}
			masters[gid] = &SentinelMaster{
				Addr: net.JoinHostPort(ip, port),
				Info: master, Epoch: epoch,
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

type SentinelMaster struct {
	Addr  string
	Info  map[string]string
	Epoch int64
}

func (s *Sentinel) Masters(groups map[int]bool, timeout time.Duration, sentinels ...string) (map[int]string, error) {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	timeout += time.Second * 5
	results := make(chan map[int]*SentinelMaster, len(sentinels))

	var majority = 1 + len(sentinels)/2

	for i := range sentinels {
		go func(sentinel string) {
			masters, err := s.mastersInstance(cntx, sentinel, groups, timeout)
			if err != nil {
				s.errorf(err, "sentinel-[%s] masters failed", sentinel)
			}
			results <- masters
		}(sentinels[i])
	}

	masters := make(map[int]string)
	current := make(map[int]*SentinelMaster)

	var voted int
	for alive := len(sentinels); ; alive-- {
		if alive == 0 {
			if cntx.Err() == context.Canceled {
				s.printf("sentinel masters canceled (%v)", cntx.Err())
				return nil, errors.Trace(cntx.Err())
			} else if voted != len(sentinels) || len(masters) != len(groups) {
				s.printf("sentinel masters voted = (%d/%d) masters = (%d/%d) (%v)", voted, len(sentinels), len(masters), len(groups), cntx.Err())
			}
			if voted < majority {
				return nil, errors.Errorf("lost majority (%d/%d)", voted, len(sentinels))
			}
			return masters, nil
		}
		select {
		case <-cntx.Done():
			if cntx.Err() == context.Canceled {
				s.printf("sentinel masters canceled (%v)", cntx.Err())
				return nil, errors.Trace(cntx.Err())
			} else {
				s.printf("sentinel masters voted = (%d/%d) masters = (%d/%d) (%v)", voted, len(sentinels), len(masters), len(groups), cntx.Err())
			}
			if voted < majority {
				return nil, errors.Errorf("lost majority (%d/%d)", voted, len(sentinels))
			}
			return masters, nil
		case m := <-results:
			if m == nil {
				continue
			}
			for gid, master := range m {
				if current[gid] == nil || current[gid].Epoch < master.Epoch {
					current[gid] = master
					masters[gid] = master.Addr
				}
			}
			voted += 1
		}
	}
}

type MonitorConfig struct {
	Quorum          int
	ParallelSyncs   int
	DownAfter       time.Duration
	FailoverTimeout time.Duration

	NotificationScript   string
	ClientReconfigScript string
}

func (s *Sentinel) monitorCommand(client *Client, sentniel string, groups map[int]*net.TCPAddr, config *MonitorConfig) error {
	for gid, tcpAddr := range groups {
		var name = s.NodeName(gid)
		if exists, err := s.existsCommand(client, name); err != nil {
			return err
		} else if exists {
			_, err := client.Do("SENTINEL", "remove", name)
			if err != nil {
				return errors.Trace(err)
			}
		}
		var host = tcpAddr.IP.String()
		var port = tcpAddr.Port
		_, err := client.Do("SENTINEL", "monitor", name, host, port, config.Quorum)
		if err != nil {
			return errors.Trace(err)
		} else {
			var arguments = []interface{}{"set", name}
			if config.ParallelSyncs != 0 {
				arguments = append(arguments, "parallel-syncs", config.ParallelSyncs)
			}
			if config.DownAfter != 0 {
				arguments = append(arguments, "down-after-milliseconds", int(config.DownAfter/time.Millisecond))
			}
			if config.FailoverTimeout != 0 {
				arguments = append(arguments, "failover-timeout", int(config.FailoverTimeout/time.Millisecond))
			}
			if s.Auth != "" {
				arguments = append(arguments, "auth-pass", s.Auth)
			}
			if config.NotificationScript != "" {
				arguments = append(arguments, "notification-script", config.NotificationScript)
			}
			if config.ClientReconfigScript != "" {
				arguments = append(arguments, "client-reconfig-script", config.ClientReconfigScript)
			}
			_, err := client.Do("SENTINEL", arguments...)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

func (s *Sentinel) monitorInstance(ctx context.Context, sentinel string, groups map[int]*net.TCPAddr, config *MonitorConfig, timeout time.Duration) error {
	c, err := NewClientNoAuth(sentinel, timeout)
	if err != nil {
		return err
	}
	defer c.Close()

	var exit = make(chan error, 1)

	go func() {
		exit <- s.monitorCommand(c, sentinel, groups, config)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-exit:
		return err
	}
}

func (s *Sentinel) Monitor(groups map[int]string, config *MonitorConfig, timeout time.Duration, sentinels ...string) error {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	resolve := make(map[int]*net.TCPAddr)

	var exit = make(chan error, 1)

	go func() (err error) {
		defer func() {
			exit <- err
		}()
		for gid, addr := range groups {
			if err := cntx.Err(); err != nil {
				return errors.Trace(err)
			}
			tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
			if err != nil {
				s.printf("sentinel monitor resolve tcp address of %s failed, %s", addr, err)
				return errors.Trace(err)
			}
			resolve[gid] = tcpAddr
		}
		return nil
	}()

	select {
	case <-cntx.Done():
		if cntx.Err() == context.Canceled {
			s.printf("sentinel monitor canceled (%v)", cntx.Err())
		} else {
			s.printf("sentinel montior resolve tcp address (%v)", cntx.Err())
		}
		return errors.Trace(cntx.Err())
	case err := <-exit:
		if err != nil {
			return err
		}
	}

	timeout += time.Second * 5
	results := make(chan error, len(sentinels))

	for i := range sentinels {
		go func(sentinel string) {
			err := s.monitorInstance(cntx, sentinel, resolve, config, timeout)
			if err != nil {
				s.errorf(err, "sentinel-[%s] monitor failed", sentinel)
			}
			results <- err
		}(sentinels[i])
	}

	var last error
	for _ = range sentinels {
		select {
		case <-cntx.Done():
			if last != nil {
				return last
			}
			return errors.Trace(cntx.Err())
		case err := <-results:
			if err != nil {
				last = err
			}
		}
	}
	return last
}

func (s *Sentinel) unmonitorCommand(client *Client, sentinel string, groups map[int]bool) error {
	for gid := range groups {
		var name = s.NodeName(gid)
		if exists, err := s.existsCommand(client, name); err != nil {
			return err
		} else if exists {
			_, err := client.Do("SENTINEL", "remove", name)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

func (s *Sentinel) unmonitorInstance(ctx context.Context, sentinel string, groups map[int]bool, timeout time.Duration) error {
	c, err := NewClientNoAuth(sentinel, timeout)
	if err != nil {
		return err
	}
	defer c.Close()

	var exit = make(chan error, 1)

	go func() {
		exit <- s.unmonitorCommand(c, sentinel, groups)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-exit:
		return err
	}
}

func (s *Sentinel) Unmonitor(groups map[int]bool, timeout time.Duration, sentinels ...string) error {
	cntx, cancel := context.WithTimeout(s.Context, timeout)
	defer cancel()

	timeout += time.Second * 5
	results := make(chan error, len(sentinels))

	for i := range sentinels {
		go func(sentinel string) {
			err := s.unmonitorInstance(cntx, sentinel, groups, timeout)
			if err != nil {
				s.errorf(err, "sentinel-[%s] unmonitor failed", sentinel)
			}
			results <- err
		}(sentinels[i])
	}

	var last error
	for _ = range sentinels {
		select {
		case <-cntx.Done():
			if last != nil {
				return last
			}
			return errors.Trace(cntx.Err())
		case err := <-results:
			if err != nil {
				last = err
			}
		}
	}
	return last
}

type SentinelGroup struct {
	Master map[string]string   `json:"master"`
	Slaves []map[string]string `json:"slaves,omitempty"`
}

func (s *Sentinel) MastersAndSlaves(sentinel string, timeout time.Duration) (map[string]*SentinelGroup, error) {
	c, err := NewClientNoAuth(sentinel, timeout)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	masters, err := s.mastersCommand(c)
	if err != nil {
		return nil, err
	}

	var results = make(map[string]*SentinelGroup)
	for _, master := range masters {
		var name = master["name"]
		slaves, err := s.slavesCommand(c, name)
		if err != nil {
			return nil, err
		}
		results[name] = &SentinelGroup{
			Master: master, Slaves: slaves,
		}
	}
	return results, nil
}

func (s *Sentinel) FlushConfig(sentinel string) error {
	c, err := NewClientNoAuth(sentinel, time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	if _, err := c.Do("SENTINEL", "flushconfig"); err != nil {
		return err
	}
	return nil
}
