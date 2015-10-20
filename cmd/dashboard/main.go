// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docopt/docopt-go"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/topom"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

const banner = `
  _____  ____    ____/ /  (_)  _____
 / ___/ / __ \  / __  /  / /  / ___/
/ /__  / /_/ / / /_/ /  / /  (__  )
\___/  \____/  \__,_/  /_/  /____/

`

func main() {
	const usage = `
Usage:
	codis-dashboard [--ncpu=N] [--config=CONF] [--log=FILE] [--log-level=LEVEL]

Options:
	--ncpu=N                    set runtime.GOMAXPROCS to N, default is runtime.NumCPU().
	-c CONF, --config=CONF      specify the config file.
	-l FILE, --log=FILE         specify the daliy rotated log file.
	--log-level=LEVEL           specify the log-level, can be INFO,WARN,DEBUG,ERROR, default is INFO.
`

	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}

	if s, ok := d["--log"].(string); ok && s != "" {
		w, err := log.NewRollingFile(s, log.DailyRolling)
		if err != nil {
			log.PanicErrorf(err, "open log file %s failed", s)
		} else {
			log.StdLog = log.New(w, "")
		}
	}
	log.SetLevel(log.LEVEL_INFO)

	fmt.Println(banner)

	ncpu := runtime.NumCPU()
	if s, ok := d["--ncpu"].(string); ok && s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			log.PanicErrorf(err, "parse --ncpu failed, invalid ncpu = '%s'", s)
		}
		ncpu = n
	}
	runtime.GOMAXPROCS(ncpu)
	log.Infof("set ncpu = %d", ncpu)

	if s, ok := d["--log-level"].(string); ok && s != "" {
		var level = strings.ToUpper(s)
		switch s {
		case "ERROR":
			log.SetLevel(log.LEVEL_ERROR)
		case "DEBUG":
			log.SetLevel(log.LEVEL_DEBUG)
		case "WARN", "WARNING":
			log.SetLevel(log.LEVEL_WARN)
		case "INFO":
			log.SetLevel(log.LEVEL_INFO)
		default:
			log.Panicf("parse --log-level failed, invalid level = '%s'", level)
		}
	}

	config := topom.NewDefaultConfig()
	if s, ok := d["--config"].(string); ok && s != "" {
		if err := config.LoadFromFile(s); err != nil {
			log.PanicErrorf(err, "load config failed, file = '%s'", s)
		}
	}

	s, err := topom.New(newMemStore(), config)
	if err != nil {
		log.PanicErrorf(err, "create topom with config file failed\n%s\n", config)
	}
	defer s.Close()

	log.Infof("create topom with config\n%s\n", config)

	for !s.IsClosed() {
		time.Sleep(time.Second)
	}

	log.Infof("[%p] topom exiting ...", s)
}

type memStore struct {
	mu sync.Mutex

	data map[string][]byte
}

var (
	ErrNodeExists    = errors.New("node already exists")
	ErrNodeNotExists = errors.New("node does not exist")
)

func newMemStore() *memStore {
	return &memStore{data: make(map[string][]byte)}
}

func (s *memStore) Acquire(name string, topom *models.Topom) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data["meta"]; ok {
		return errors.Trace(ErrNodeExists)
	}

	s.data["meta"] = topom.Encode()
	return nil
}

func (s *memStore) Release() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data["meta"]; !ok {
		return errors.Trace(ErrNodeNotExists)
	}

	delete(s.data, "meta")
	return nil
}

func (s *memStore) LoadSlotMapping(slotId int) (*models.SlotMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("slot-%04d", slotId)
	var m = &models.SlotMapping{}

	if b, ok := s.data[k]; ok {
		if err := json.Unmarshal(b, m); err != nil {
			return nil, errors.Trace(err)
		}
	} else {
		m.Id = slotId
	}
	return m, nil
}

func (s *memStore) SaveSlotMapping(slotId int, slot *models.SlotMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("slot-%04d", slotId)

	s.data[k] = slot.Encode()
	return nil
}

func (s *memStore) ListProxy() ([]*models.Proxy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var plist []*models.Proxy

	for k, b := range s.data {
		if strings.HasPrefix(k, "proxy-") {
			var p = &models.Proxy{}
			if err := json.Unmarshal(b, p); err != nil {
				return nil, errors.Trace(err)
			}
			plist = append(plist, p)
		}
	}
	return plist, nil
}

func (s *memStore) CreateProxy(proxyId int, proxy *models.Proxy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("proxy-%d", proxyId)

	if _, ok := s.data[k]; ok {
		return errors.Trace(ErrNodeExists)
	}

	s.data[k] = proxy.Encode()
	return nil
}

func (s *memStore) RemoveProxy(proxyId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("proxy-%d", proxyId)

	if _, ok := s.data[k]; !ok {
		return errors.Trace(ErrNodeNotExists)
	}

	delete(s.data, k)
	return nil
}

func (s *memStore) ListGroup() ([]*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var glist []*models.Group

	for k, b := range s.data {
		if strings.HasPrefix(k, "group-") {
			var g = &models.Group{}
			if err := json.Unmarshal(b, g); err != nil {
				return nil, errors.Trace(err)
			}
			glist = append(glist, g)
		}
	}
	return glist, nil
}

func (s *memStore) CreateGroup(groupId int, group *models.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("group-%d", groupId)

	if _, ok := s.data[k]; ok {
		return errors.Trace(ErrNodeExists)
	}

	s.data[k] = group.Encode()
	return nil
}

func (s *memStore) RemoveGroup(groupId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("group-%d", groupId)

	if _, ok := s.data[k]; !ok {
		return errors.Trace(ErrNodeNotExists)
	}

	delete(s.data, k)
	return nil
}

func (s *memStore) UpdateGroup(groupId int, group *models.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var k = fmt.Sprintf("group-%d", groupId)

	if _, ok := s.data[k]; !ok {
		return errors.Trace(ErrNodeNotExists)
	}

	s.data[k] = group.Encode()
	return nil
}

func (s *memStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return nil
}
