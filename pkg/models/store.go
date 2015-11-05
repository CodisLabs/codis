package models

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/wandoulabs/codis/pkg/utils/errors"
)

type Client interface {
	Create(path string, data []byte) error
	Update(path string, data []byte) error
	Delete(path string) error

	Read(path string) ([]byte, error)
	List(path string) ([]string, error)

	Close() error
}

var (
	ErrClosedStore  = errors.New("use of closed store")
	ErrNoLockHolder = errors.New("without lock protection")
	ErrAcquireAgain = errors.New("acquire again")
	ErrReleaseAgain = errors.New("release again")
)

type Store struct {
	sync.Mutex

	client Client
	prefix string

	locked bool
	closed bool
}

func NewStore(client Client, name string) *Store {
	return &Store{
		client: client,
		prefix: filepath.Join("/codis2", name),
	}
}

func (s *Store) Close() error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	s.client.Close()
	return nil
}

func (s *Store) LockPath() string {
	return filepath.Join(s.prefix, "topom")
}

func (s *Store) SlotPath(slotId int) string {
	return filepath.Join(s.prefix, "slots", fmt.Sprintf("slot-%04d", slotId))
}

func (s *Store) ProxyBase() string {
	return filepath.Join(s.prefix, "proxy")
}

func (s *Store) ProxyPath(proxyId int) string {
	return filepath.Join(s.prefix, "proxy", fmt.Sprintf("proxy-%04d", proxyId))
}

func (s *Store) GroupBase() string {
	return filepath.Join(s.prefix, "group")
}

func (s *Store) GroupPath(groupId int) string {
	return filepath.Join(s.prefix, "group", fmt.Sprintf("group-%04d", groupId))
}

func (s *Store) Acquire(topom *Topom) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if s.locked {
		return ErrAcquireAgain
	}

	if err := s.client.Create(s.LockPath(), topom.Encode()); err != nil {
		return err
	}
	s.locked = true
	return nil
}

func (s *Store) Release(force bool) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
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

func (s *Store) LoadSlotMapping(slotId int) (*SlotMapping, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedStore
	}
	if !s.locked {
		return nil, ErrNoLockHolder
	}

	b, err := s.client.Read(s.SlotPath(slotId))
	if err != nil {
		return nil, err
	}
	if b != nil {
		slot := &SlotMapping{}
		if err := slot.Decode(b); err != nil {
			return nil, err
		}
		return slot, nil
	}
	return nil, nil
}

func (s *Store) SaveSlotMapping(slotId int, slot *SlotMapping) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Update(s.SlotPath(slotId), slot.Encode())
}

func (s *Store) ListProxy() ([]*Proxy, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedStore
	}
	if !s.locked {
		return nil, ErrNoLockHolder
	}

	files, err := s.client.List(s.ProxyBase())
	if err != nil {
		return nil, err
	}

	var plist []*Proxy
	for _, file := range files {
		b, err := s.client.Read(file)
		if err != nil {
			return nil, err
		}
		p := &Proxy{}
		if err := p.Decode(b); err != nil {
			return nil, err
		}
		plist = append(plist, p)
	}
	return plist, nil
}

func (s *Store) CreateProxy(proxyId int, proxy *Proxy) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Create(s.ProxyPath(proxyId), proxy.Encode())
}

func (s *Store) RemoveProxy(proxyId int) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Delete(s.ProxyPath(proxyId))
}

func (s *Store) ListGroup() ([]*Group, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedStore
	}
	if !s.locked {
		return nil, ErrNoLockHolder
	}

	files, err := s.client.List(s.GroupBase())
	if err != nil {
		return nil, err
	}

	var glist []*Group
	for _, file := range files {
		b, err := s.client.Read(file)
		if err != nil {
			return nil, err
		}
		g := &Group{}
		if err := g.Decode(b); err != nil {
			return nil, err
		}
		glist = append(glist, g)
	}
	return glist, nil
}

func (s *Store) CreateGroup(groupId int, group *Group) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Create(s.GroupPath(groupId), group.Encode())
}

func (s *Store) UpdateGroup(groupId int, group *Group) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Update(s.GroupPath(groupId), group.Encode())
}

func (s *Store) RemoveGroup(groupId int) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Delete(s.GroupPath(groupId))
}
