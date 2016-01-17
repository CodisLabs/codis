// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"fmt"
	"path/filepath"
)

type Client interface {
	Create(path string, data []byte) error
	Update(path string, data []byte) error
	Delete(path string) error

	Read(path string) ([]byte, error)
	List(path string) ([]string, error)

	Close() error
}

type Store struct {
	client Client
	prefix string
}

func NewStore(client Client, name string) *Store {
	return &Store{
		client: client,
		prefix: filepath.Join("/codis3", name),
	}
}

func (s *Store) Close() error {
	return s.client.Close()
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
	return s.client.Create(s.LockPath(), topom.Encode())
}

func (s *Store) Release() error {
	return s.client.Delete(s.LockPath())
}

func (s *Store) SlotMappings() ([]*SlotMapping, error) {
	slots := make([]*SlotMapping, MaxSlotNum)
	for i := 0; i < len(slots); i++ {
		m, err := s.LoadSlotMapping(i)
		if err != nil {
			return nil, err
		}
		if m != nil {
			slots[i] = m
		} else {
			slots[i] = &SlotMapping{Id: i}
		}
	}
	return slots, nil
}

func (s *Store) LoadSlotMapping(sid int) (*SlotMapping, error) {
	b, err := s.client.Read(s.SlotPath(sid))
	if err != nil || b == nil {
		return nil, err
	}
	m := &SlotMapping{}
	if err := jsonDecode(m, b); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) UpdateSlotMapping(m *SlotMapping) error {
	return s.client.Update(s.SlotPath(m.Id), m.Encode())
}

func (s *Store) ListGroup() (map[int]*Group, error) {
	files, err := s.client.List(s.GroupBase())
	if err != nil {
		return nil, err
	}
	group := make(map[int]*Group)
	for _, path := range files {
		b, err := s.client.Read(path)
		if err != nil {
			return nil, err
		}
		g := &Group{}
		if err := jsonDecode(g, b); err != nil {
			return nil, err
		}
		group[g.Id] = g
	}
	return group, nil
}

func (s *Store) LoadGroup(gid int) (*Group, error) {
	b, err := s.client.Read(s.GroupPath(gid))
	if err != nil || b == nil {
		return nil, err
	}
	g := &Group{}
	if err := jsonDecode(g, b); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Store) UpdateGroup(g *Group) error {
	return s.client.Update(s.GroupPath(g.Id), g.Encode())
}

func (s *Store) DeleteGroup(gid int) error {
	return s.client.Delete(s.GroupPath(gid))
}

func (s *Store) ListProxy() (map[string]*Proxy, error) {
	files, err := s.client.List(s.ProxyBase())
	if err != nil {
		return nil, err
	}
	proxy := make(map[string]*Proxy)
	for _, path := range files {
		b, err := s.client.Read(path)
		if err != nil {
			return nil, err
		}
		p := &Proxy{}
		if err := jsonDecode(p, b); err != nil {
			return nil, err
		}
		proxy[p.Token] = p
	}
	return proxy, nil
}

func (s *Store) LoadProxy(token string) (*Proxy, error) {
	b, err := s.client.Read(s.ProxyPath(token))
	if err != nil || b == nil {
		return nil, err
	}
	p := &Proxy{}
	if err := jsonDecode(p, b); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) UpdateProxy(p *Proxy) error {
	return s.client.Update(s.ProxyPath(p.Token), p.Encode())
}

func (s *Store) DeleteProxy(token string) error {
	return s.client.Delete(s.ProxyPath(token))
}
