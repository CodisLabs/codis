// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package etcdstore

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

var (
	ErrClosedEtcdStore = errors.New("use of closed etcdstore")
	ErrAcquireAgain    = errors.New("acquire again")
	ErrReleaseAgain    = errors.New("release again")
	ErrNoProtection    = errors.New("operation without lock protection")
)

type EtcdStore struct {
	sync.Mutex

	client *EtcdClient
	prefix string

	locked bool
	closed bool
}

func NewStore(addr string, name string) (*EtcdStore, error) {
	client, err := NewClient(addr, time.Minute)
	if err != nil {
		return nil, err
	}
	return &EtcdStore{
		client: client,
		prefix: filepath.Join("/etcd/codis2", name),
	}, nil
}

func (s *EtcdStore) Close() error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	s.client.Close()
	return nil
}

func (s *EtcdStore) LockPath() string {
	return filepath.Join(s.prefix, "topom")
}

func (s *EtcdStore) SlotPath(slotId int) string {
	return filepath.Join(s.prefix, "slots", fmt.Sprintf("slot-%04d", slotId))
}

func (s *EtcdStore) ProxyBase() string {
	return filepath.Join(s.prefix, "proxy")
}

func (s *EtcdStore) ProxyPath(proxyId int) string {
	return filepath.Join(s.prefix, "proxy", fmt.Sprintf("proxy-%04d", proxyId))
}

func (s *EtcdStore) GroupBase() string {
	return filepath.Join(s.prefix, "group")
}

func (s *EtcdStore) GroupPath(groupId int) string {
	return filepath.Join(s.prefix, "group", fmt.Sprintf("group-%04d", groupId))
}

func (s *EtcdStore) Acquire(topom *models.Topom) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return errors.Trace(ErrClosedEtcdStore)
	}
	if s.locked {
		return errors.Trace(ErrAcquireAgain)
	}

	if err := s.client.Create(s.LockPath(), topom.Encode()); err != nil {
		return err
	}
	s.locked = true
	return nil
}

func (s *EtcdStore) Release(force bool) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedEtcdStore
	}
	if !s.locked && !force {
		return ErrReleaseAgain
	}

	if err := s.client.Delete(s.LockPath()); err != nil {
		return err
	}
	s.locked = false
	return nil
}

func (s *EtcdStore) LoadSlotMapping(slotId int) (*models.SlotMapping, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedEtcdStore
	}
	if !s.locked {
		return nil, ErrNoProtection
	}

	b, err := s.client.LoadData(s.SlotPath(slotId))
	if err != nil {
		return nil, err
	}
	if b != nil {
		slot := &models.SlotMapping{}
		if err := slot.Decode(b); err != nil {
			return nil, err
		}
		return slot, nil
	}
	return nil, nil
}

func (s *EtcdStore) SaveSlotMapping(slotId int, slot *models.SlotMapping) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedEtcdStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Update(s.SlotPath(slotId), slot.Encode())
}

func (s *EtcdStore) ListProxy() ([]*models.Proxy, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedEtcdStore
	}
	if !s.locked {
		return nil, ErrNoProtection
	}

	files, err := s.client.ListFile(s.ProxyBase())
	if err != nil {
		return nil, err
	}

	var plist []*models.Proxy
	for _, file := range files {
		b, err := s.client.LoadData(file)
		if err != nil {
			return nil, err
		}
		p := &models.Proxy{}
		if err := p.Decode(b); err != nil {
			return nil, err
		}
		plist = append(plist, p)
	}
	return plist, nil
}

func (s *EtcdStore) CreateProxy(proxyId int, proxy *models.Proxy) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedEtcdStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Create(s.ProxyPath(proxyId), proxy.Encode())
}

func (s *EtcdStore) RemoveProxy(proxyId int) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedEtcdStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Delete(s.ProxyPath(proxyId))
}

func (s *EtcdStore) ListGroup() ([]*models.Group, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedEtcdStore
	}
	if !s.locked {
		return nil, ErrNoProtection
	}

	files, err := s.client.ListFile(s.GroupBase())
	if err != nil {
		return nil, err
	}

	var glist []*models.Group
	for _, file := range files {
		b, err := s.client.LoadData(file)
		if err != nil {
			return nil, err
		}
		g := &models.Group{}
		if err := g.Decode(b); err != nil {
			return nil, err
		}
		glist = append(glist, g)
	}
	return glist, nil
}

func (s *EtcdStore) CreateGroup(groupId int, group *models.Group) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedEtcdStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Create(s.GroupPath(groupId), group.Encode())
}

func (s *EtcdStore) UpdateGroup(groupId int, group *models.Group) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedEtcdStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Update(s.GroupPath(groupId), group.Encode())
}

func (s *EtcdStore) RemoveGroup(groupId int) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedEtcdStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Delete(s.GroupPath(groupId))
}
