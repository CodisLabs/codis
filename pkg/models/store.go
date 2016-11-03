// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

func init() {
	if filepath.Separator != '/' {
		log.Panicf("bad Separator = '%c', must be '/'", filepath.Separator)
	}
}

const JodisDir = "/jodis"

func JodisPath(product string, token string) string {
	return filepath.Join(JodisDir, product, fmt.Sprintf("proxy-%s", token))
}

const CodisDir = "/codis3"

func ProductDir(product string) string {
	return filepath.Join(CodisDir, product)
}

func LockPath(product string) string {
	return filepath.Join(CodisDir, product, "topom")
}

func SlotPath(product string, sid int) string {
	return filepath.Join(CodisDir, product, "slots", fmt.Sprintf("slot-%04d", sid))
}

func GroupDir(product string) string {
	return filepath.Join(CodisDir, product, "group")
}

func ProxyDir(product string) string {
	return filepath.Join(CodisDir, product, "proxy")
}

func GroupPath(product string, gid int) string {
	return filepath.Join(CodisDir, product, "group", fmt.Sprintf("group-%04d", gid))
}

func ProxyPath(product string, token string) string {
	return filepath.Join(CodisDir, product, "proxy", fmt.Sprintf("proxy-%s", token))
}

func SentinelPath(product string) string {
	return filepath.Join(CodisDir, product, "sentinel")
}

func LoadTopom(client Client, product string, must bool) (*Topom, error) {
	b, err := client.Read(LockPath(product), must)
	if err != nil || b == nil {
		return nil, err
	}
	t := &Topom{}
	if err := jsonDecode(t, b); err != nil {
		return nil, err
	}
	return t, nil
}

type Store struct {
	client  Client
	product string
}

func NewStore(client Client, product string) *Store {
	return &Store{client, product}
}

func (s *Store) Close() error {
	return s.client.Close()
}

func (s *Store) Client() Client {
	return s.client
}

func (s *Store) LockPath() string {
	return LockPath(s.product)
}

func (s *Store) SlotPath(sid int) string {
	return SlotPath(s.product, sid)
}

func (s *Store) GroupDir() string {
	return GroupDir(s.product)
}

func (s *Store) ProxyDir() string {
	return ProxyDir(s.product)
}

func (s *Store) GroupPath(gid int) string {
	return GroupPath(s.product, gid)
}

func (s *Store) ProxyPath(token string) string {
	return ProxyPath(s.product, token)
}

func (s *Store) SentinelPath() string {
	return SentinelPath(s.product)
}

func (s *Store) Acquire(topom *Topom) error {
	return s.client.Create(s.LockPath(), topom.Encode())
}

func (s *Store) Release() error {
	return s.client.Delete(s.LockPath())
}

func (s *Store) LoadTopom(must bool) (*Topom, error) {
	return LoadTopom(s.client, s.product, must)
}

func (s *Store) SlotMappings() ([]*SlotMapping, error) {
	slots := make([]*SlotMapping, MaxSlotNum)
	for i := range slots {
		m, err := s.LoadSlotMapping(i, false)
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

func (s *Store) LoadSlotMapping(sid int, must bool) (*SlotMapping, error) {
	b, err := s.client.Read(s.SlotPath(sid), must)
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
	paths, err := s.client.List(s.GroupDir(), false)
	if err != nil {
		return nil, err
	}
	group := make(map[int]*Group)
	for _, path := range paths {
		b, err := s.client.Read(path, true)
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

func (s *Store) LoadGroup(gid int, must bool) (*Group, error) {
	b, err := s.client.Read(s.GroupPath(gid), must)
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
	paths, err := s.client.List(s.ProxyDir(), false)
	if err != nil {
		return nil, err
	}
	proxy := make(map[string]*Proxy)
	for _, path := range paths {
		b, err := s.client.Read(path, true)
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

func (s *Store) LoadProxy(token string, must bool) (*Proxy, error) {
	b, err := s.client.Read(s.ProxyPath(token), must)
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

func (s *Store) LoadSentinel(must bool) (*Sentinel, error) {
	b, err := s.client.Read(s.SentinelPath(), must)
	if err != nil || b == nil {
		return nil, err
	}
	p := &Sentinel{}
	if err := jsonDecode(p, b); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) UpdateSentinel(p *Sentinel) error {
	return s.client.Update(s.SentinelPath(), p.Encode())
}

func ValidateProduct(name string) error {
	if regexp.MustCompile(`^\w[\w\.\-]*$`).MatchString(name) {
		return nil
	}
	return errors.Errorf("bad product name = %s", name)
}
