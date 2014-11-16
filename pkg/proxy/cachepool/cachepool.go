// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package cachepool

import (
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy/redispool"

	"github.com/juju/errors"
)

type LivePool struct {
	pool *redispool.ConnectionPool
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
		pool: redispool.NewConnectionPool("redis conn pool", 50, 120*time.Second),
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
