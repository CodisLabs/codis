// Copyright 2015 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	log "github.com/ngaut/logging"
	"github.com/wandoulabs/go-zookeeper/zk"
	"github.com/wandoulabs/zkhelper"
	"sync"
	"time"
)

const retryMaxOnOps = 10

type ConnBuilder interface {
	// Get a conn that will retry automatically when getting error caused by connection issues.
	// If retry can not rebuild the connection, there will be a fetal error
	GetSafeConn() zkhelper.Conn

	// Get a conn that will return error caused by connection issues
	// It will try to rebuild the connection after return error.
	GetUnsafeConn() zkhelper.Conn
}

type connBuilder struct {
	connection         zkhelper.Conn
	builder            func() (zkhelper.Conn, error)
	createdOn          time.Time
	lock               sync.RWMutex
	safeConnInstance   *safeConn
	unsafeConnInstance *unsafeConn
}

func NewConnBuilder(buildFunc func() (zkhelper.Conn, error)) ConnBuilder {
	b := &connBuilder{
		builder: buildFunc,
	}
	b.safeConnInstance = &safeConn{builder: b}
	b.unsafeConnInstance = &unsafeConn{builder: b}
	b.resetConnection()
	return b
}

func (b *connBuilder) resetConnection() {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.builder == nil {
		log.Fatal("no connection builder")
	}
	if time.Now().Before(b.createdOn.Add(time.Second)) {
		return
	}
	if b.connection != nil {
		b.connection.Close()
	}
	var err error
	b.connection, err = b.builder() // this is asnyc
	if err == nil {
		b.safeConnInstance.Conn = b.connection
		b.unsafeConnInstance.Conn = b.connection
		b.createdOn = time.Now()
		return
	}
	log.Fatal("can not build new zk session, exit")
}

func (b *connBuilder) GetSafeConn() zkhelper.Conn {
	return b.safeConnInstance
}

func (b *connBuilder) GetUnsafeConn() zkhelper.Conn {
	return b.unsafeConnInstance
}

type conn struct {
	zkhelper.Conn
	builder *connBuilder
}

type safeConn conn
type unsafeConn conn

func isConnectionError(e error) bool {
	return !zkhelper.ZkErrorEqual(zk.ErrNoNode, e) && !zkhelper.ZkErrorEqual(zk.ErrNodeExists, e) &&
		!zkhelper.ZkErrorEqual(zk.ErrNodeExists, e) && !zkhelper.ZkErrorEqual(zk.ErrNotEmpty, e)
}

func (c *safeConn) Get(path string) (data []byte, stat zk.Stat, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		data, stat, err = c.Conn.Get(path)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) GetW(path string) (data []byte, stat zk.Stat, watch <-chan zk.Event, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		data, stat, watch, err = c.Conn.GetW(path)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) Children(path string) (children []string, stat zk.Stat, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		children, stat, err = c.Conn.Children(path)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) ChildrenW(path string) (children []string, stat zk.Stat, watch <-chan zk.Event, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		children, stat, watch, err = c.Conn.ChildrenW(path)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) Exists(path string) (exist bool, stat zk.Stat, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		exist, stat, err = c.Conn.Exists(path)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) ExistsW(path string) (exist bool, stat zk.Stat, watch <-chan zk.Event, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		exist, stat, watch, err = c.Conn.ExistsW(path)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) Create(path string, value []byte, flags int32, aclv []zk.ACL) (pathCreated string, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		pathCreated, err = c.Conn.Create(path, value, flags, aclv)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) Set(path string, value []byte, version int32) (stat zk.Stat, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		stat, err = c.Conn.Set(path, value, version)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) Delete(path string, version int32) (err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		err = c.Conn.Delete(path, version)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) Close() {
	log.Fatal("do not close zk connection by yourself")
}

func (c *safeConn) GetACL(path string) (acl []zk.ACL, stat zk.Stat, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		acl, stat, err = c.Conn.GetACL(path)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) SetACL(path string, aclv []zk.ACL, version int32) (stat zk.Stat, err error) {
	for i := 0; i <= retryMaxOnOps; i++ {
		c.builder.lock.RLock()
		stat, err = c.Conn.SetACL(path, aclv, version)
		c.builder.lock.RUnlock()
		if err == nil || !isConnectionError(err) {
			return
		}
		c.builder.resetConnection()
	}
	log.Warning(err)
	log.Fatal("zk error after retries")
	return
}

func (c *safeConn) Seq2Str(seq int64) string {
	return c.Conn.Seq2Str(seq)
}

func (c *unsafeConn) Get(path string) (data []byte, stat zk.Stat, err error) {
	c.builder.lock.RLock()
	data, stat, err = c.Conn.Get(path)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) GetW(path string) (data []byte, stat zk.Stat, watch <-chan zk.Event, err error) {
	c.builder.lock.RLock()
	data, stat, watch, err = c.Conn.GetW(path)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) Children(path string) (children []string, stat zk.Stat, err error) {
	c.builder.lock.RLock()
	children, stat, err = c.Conn.Children(path)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) ChildrenW(path string) (children []string, stat zk.Stat, watch <-chan zk.Event, err error) {
	c.builder.lock.RLock()
	children, stat, watch, err = c.Conn.ChildrenW(path)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) Exists(path string) (exist bool, stat zk.Stat, err error) {
	c.builder.lock.RLock()
	exist, stat, err = c.Conn.Exists(path)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) ExistsW(path string) (exist bool, stat zk.Stat, watch <-chan zk.Event, err error) {
	c.builder.lock.RLock()
	exist, stat, watch, err = c.Conn.ExistsW(path)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) Create(path string, value []byte, flags int32, aclv []zk.ACL) (pathCreated string, err error) {
	c.builder.lock.RLock()
	pathCreated, err = c.Conn.Create(path, value, flags, aclv)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) Set(path string, value []byte, version int32) (stat zk.Stat, err error) {
	c.builder.lock.RLock()
	stat, err = c.Conn.Set(path, value, version)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) Delete(path string, version int32) (err error) {
	c.builder.lock.RLock()
	err = c.Conn.Delete(path, version)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) Close() {
	log.Fatal("do not close zk connection by yourself")
}

func (c *unsafeConn) GetACL(path string) (acl []zk.ACL, stat zk.Stat, err error) {
	c.builder.lock.RLock()
	acl, stat, err = c.Conn.GetACL(path)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) SetACL(path string, aclv []zk.ACL, version int32) (stat zk.Stat, err error) {
	c.builder.lock.RLock()
	stat, err = c.Conn.SetACL(path, aclv, version)
	c.builder.lock.RUnlock()
	if err != nil && isConnectionError(err) {
		go c.builder.resetConnection()
	}
	return
}

func (c *unsafeConn) Seq2Str(seq int64) string {
	return c.Conn.Seq2Str(seq)
}
