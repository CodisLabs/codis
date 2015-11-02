package zkstore

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

var (
	ErrClosedZkStore = errors.New("use of closed zkstore")
	ErrAcquireAgain  = errors.New("acquire again")
	ErrReleaseAgain  = errors.New("release again")
	ErrNoProtection  = errors.New("operation without lock protection")
)

type ZkStore struct {
	sync.Mutex

	client *ZkClient
	prefix string

	locked bool
	closed bool
}

func NewStore(addr string, name string) (*ZkStore, error) {
	client, err := NewClient(addr, time.Minute)
	if err != nil {
		return nil, err
	}
	return &ZkStore{
		client: client,
		prefix: filepath.Join("/zk/codis2", name),
	}, nil
}

func (s *ZkStore) Close() error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	s.client.Close()
	return nil
}

func (s *ZkStore) LockPath() string {
	return filepath.Join(s.prefix, "topom")
}

func (s *ZkStore) SlotPath(slotId int) string {
	return filepath.Join(s.prefix, "slots", fmt.Sprintf("slot-%04d", slotId))
}

func (s *ZkStore) ProxyBase() string {
	return filepath.Join(s.prefix, "proxy")
}

func (s *ZkStore) ProxyPath(proxyId int) string {
	return filepath.Join(s.prefix, "proxy", fmt.Sprintf("proxy-%04d", proxyId))
}

func (s *ZkStore) GroupBase() string {
	return filepath.Join(s.prefix, "group")
}

func (s *ZkStore) GroupPath(groupId int) string {
	return filepath.Join(s.prefix, "group", fmt.Sprintf("group-%04d", groupId))
}

func (s *ZkStore) Acquire(topom *models.Topom) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedZkStore
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

func (s *ZkStore) Release(force bool) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedZkStore
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

func (s *ZkStore) LoadSlotMapping(slotId int) (*models.SlotMapping, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedZkStore
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

func (s *ZkStore) SaveSlotMapping(slotId int, slot *models.SlotMapping) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedZkStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Update(s.SlotPath(slotId), slot.Encode())
}

func (s *ZkStore) ListProxy() ([]*models.Proxy, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedZkStore
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

func (s *ZkStore) CreateProxy(proxyId int, proxy *models.Proxy) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedZkStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Create(s.ProxyPath(proxyId), proxy.Encode())
}

func (s *ZkStore) RemoveProxy(proxyId int) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedZkStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Delete(s.ProxyPath(proxyId))
}

func (s *ZkStore) ListGroup() ([]*models.Group, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, ErrClosedZkStore
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

func (s *ZkStore) CreateGroup(groupId int, group *models.Group) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedZkStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Create(s.GroupPath(groupId), group.Encode())
}

func (s *ZkStore) UpdateGroup(groupId int, group *models.Group) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedZkStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Update(s.GroupPath(groupId), group.Encode())
}

func (s *ZkStore) RemoveGroup(groupId int) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosedZkStore
	}
	if !s.locked {
		return ErrNoProtection
	}

	return s.client.Delete(s.GroupPath(groupId))
}
