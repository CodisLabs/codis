// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package cachepool

import (
	"sync"
	"time"

	"container/list"
	"github.com/juju/errors"
	"github.com/wandoulabs/codis/pkg/proxy/redispool"
)

type SimpleConnectionPool struct {
	createTs time.Time
	sync.Mutex
	fact  redispool.CreateConnectionFunc
	conns *list.List
}

func NewSimpleConnectionPool() *SimpleConnectionPool {
	return &SimpleConnectionPool{
		createTs: time.Now(),
	}
}

func (s *SimpleConnectionPool) Put(conn redispool.PoolConnection) {
	if conn != nil {
		s.Lock()
		s.conns.PushBack(conn)
		s.Unlock()
	}

}

func (s *SimpleConnectionPool) Get() (redispool.PoolConnection, error) {
	s.Lock()
	if s.conns.Len() == 0 {
		c, err := s.fact(s)
		s.Unlock()
		return c, err
	}

	e := s.conns.Front()
	s.conns.Remove(e)

	s.Unlock()
	return e.Value.(redispool.PoolConnection), nil
}

func (s *SimpleConnectionPool) Open(fact redispool.CreateConnectionFunc) {
	s.Lock()
	defer s.Unlock()
	s.fact = fact
	s.conns = list.New()
}

func (s *SimpleConnectionPool) Close() {
	s.Lock()
	defer s.Unlock()
	for e := s.conns.Front(); e != nil; e = e.Next() {
		e.Value.(redispool.PoolConnection).Close()
	}
}

type LivePool struct {
	pool redispool.IPool
}

type CachePool struct {
	mu    sync.RWMutex
	pools map[string]*LivePool
}

func NewCachePool() *CachePool {
	return &CachePool{
		pools: make(map[string]*LivePool),
	}
}

func (cp *CachePool) GetConn(key string) (redispool.PoolConnection, error) {
	cp.mu.RLock()

	pool, ok := cp.pools[key]
	if !ok {
		cp.mu.RUnlock()
		return nil, errors.Errorf("pool %s not exist", key)
	}

	cp.mu.RUnlock()
	c, err := pool.pool.Get()

	return c, err
}

func (cp *CachePool) ReleaseConn(pc redispool.PoolConnection) {
	pc.Recycle()
}

func (cp *CachePool) AddPool(key string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	pool, ok := cp.pools[key]
	if ok {
		return nil
	}
	pool = &LivePool{
		//pool: redispool.NewConnectionPool("redis conn pool", 50, 120*time.Second),
		pool: NewSimpleConnectionPool(),
	}

	pool.pool.Open(redispool.ConnectionCreator(key))

	cp.pools[key] = pool

	return nil
}

func (cp *CachePool) RemovePool(key string) error {
	cp.mu.Lock()

	pool, ok := cp.pools[key]
	if !ok {
		cp.mu.Unlock()
		return errors.Errorf("pool %s not exist", key)
	}
	delete(cp.pools, key)
	cp.mu.Unlock()

	go pool.pool.Close()
	return nil
}
