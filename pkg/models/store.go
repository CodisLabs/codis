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
	ErrNoLockHolder = errors.New("without lock holder")
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

func (s *Store) SlotPath(sid int) string {
	return filepath.Join(s.prefix, "slots", fmt.Sprintf("slot-%04d", sid))
}

func (s *Store) GroupBase() string {
	return filepath.Join(s.prefix, "group")
}

func (s *Store) GroupPath(gid int) string {
	return filepath.Join(s.prefix, "group", fmt.Sprintf("group-%04d", gid))
}

func (s *Store) ProxyBase() string {
	return filepath.Join(s.prefix, "proxy")
}

func (s *Store) ProxyPath(token string) string {
	return filepath.Join(s.prefix, "proxy", fmt.Sprintf("proxy-%s", token))
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

func (s *Store) SlotMappings() ([]*SlotMapping, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedStore
	}
	if !s.locked {
		return nil, ErrNoLockHolder
	}

	slots := make([]*SlotMapping, MaxSlotNum)
	for i := 0; i < len(slots); i++ {
		b, err := s.client.Read(s.SlotPath(i))
		if err != nil {
			return nil, err
		}
		if b != nil {
			m := &SlotMapping{}
			if err := m.Decode(b); err != nil {
				return nil, err
			}
			slots[i] = m
		} else {
			slots[i] = &SlotMapping{Id: i}
		}
	}
	return slots, nil
}

func (s *Store) UpdateSlotMapping(m *SlotMapping) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Update(s.SlotPath(m.Id), m.Encode())
}

func (s *Store) ListGroup() (map[int]*Group, error) {
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
	group := make(map[int]*Group)
	for _, file := range files {
		b, err := s.client.Read(file)
		if err != nil {
			return nil, err
		}
		g := &Group{}
		if err := g.Decode(b); err != nil {
			return nil, err
		}
		group[g.Id] = g
	}
	return group, nil
}

func (s *Store) UpdateGroup(g *Group) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Update(s.GroupPath(g.Id), g.Encode())
}

func (s *Store) DeleteGroup(gid int) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Delete(s.GroupPath(gid))
}

func (s *Store) ListProxy() (map[string]*Proxy, error) {
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
	proxy := make(map[string]*Proxy)
	for _, file := range files {
		b, err := s.client.Read(file)
		if err != nil {
			return nil, err
		}
		p := &Proxy{}
		if err := p.Decode(b); err != nil {
			return nil, err
		}
		proxy[p.Token] = p
	}
	return proxy, nil
}

func (s *Store) UpdateProxy(p *Proxy) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Update(s.ProxyPath(p.Token), p.Encode())
}

func (s *Store) DeleteProxy(token string) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedStore
	}
	if !s.locked {
		return ErrNoLockHolder
	}

	return s.client.Delete(s.ProxyPath(token))
}
