// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package etcdclient

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

const CoordinatorName = "etcd"

var ErrClosedEtcdClient = errors.New("use of closed etcd client")

type EtcdClient struct {
	sync.Mutex
	kapi client.KeysAPI

	closed  bool
	timeout time.Duration
}

func New(addr string, timeout time.Duration) (*EtcdClient, error) {
	endpoints := strings.Split(addr, ",")
	for i, s := range endpoints {
		if s != "" && !strings.HasPrefix(s, "http://") {
			endpoints[i] = "http://" + s
		}
	}
	config := client.Config{
		Endpoints: endpoints,
		Transport: client.DefaultTransport,

		HeaderTimeoutPerRequest: time.Second * 3,
	}
	c, err := client.New(config)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &EtcdClient{
		kapi: client.NewKeysAPI(c), timeout: timeout,
	}, nil
}

func (c *EtcdClient) Close() error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

func (c *EtcdClient) contextWithTimeout() (context.Context, context.CancelFunc) {
	if c.timeout == 0 {
		return context.Background(), func() {}
	} else {
		return context.WithTimeout(context.Background(), c.timeout)
	}
}

func (c *EtcdClient) Do(fn func(kapi client.KeysAPI) error) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedEtcdClient)
	}
	return fn(c.kapi)
}

func isErrNoNode(err error) bool {
	if err != nil {
		if e, ok := err.(client.Error); ok {
			return e.Code == client.ErrorCodeKeyNotFound
		}
	}
	return false
}

func isErrNodeExists(err error) bool {
	if err != nil {
		if e, ok := err.(client.Error); ok {
			return e.Code == client.ErrorCodeNodeExist
		}
	}
	return false
}

func (c *EtcdClient) Mkdir(dir string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedEtcdClient)
	}
	return c.mkdir(dir)
}

func (c *EtcdClient) mkdir(dir string) error {
	if dir == "" || dir == "/" {
		return nil
	}
	cntx, canceller := c.contextWithTimeout()
	defer canceller()
	_, err := c.kapi.Set(cntx, dir, "", &client.SetOptions{Dir: true, PrevExist: client.PrevNoExist})
	if err != nil {
		if isErrNodeExists(err) {
			return nil
		}
		return errors.Trace(err)
	}
	return nil
}

func (c *EtcdClient) Create(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedEtcdClient)
	}
	cntx, canceller := c.contextWithTimeout()
	defer canceller()
	log.Debugf("etcd create node %s", path)
	_, err := c.kapi.Set(cntx, path, string(data), &client.SetOptions{PrevExist: client.PrevNoExist})
	if err != nil {
		log.Debugf("etcd create node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("etcd create node OK")
	return nil
}

func (c *EtcdClient) Update(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedEtcdClient)
	}
	cntx, canceller := c.contextWithTimeout()
	defer canceller()
	log.Debugf("etcd update node %s", path)
	_, err := c.kapi.Set(cntx, path, string(data), &client.SetOptions{PrevExist: client.PrevIgnore})
	if err != nil {
		log.Debugf("etcd update node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("etcd update node OK")
	return nil
}

func (c *EtcdClient) Delete(path string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedEtcdClient)
	}
	cntx, canceller := c.contextWithTimeout()
	defer canceller()
	log.Debugf("etcd delete node %s", path)
	_, err := c.kapi.Delete(cntx, path, nil)
	if err != nil && !isErrNoNode(err) {
		log.Debugf("etcd delete node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("etcd delete node OK")
	return nil
}

func (c *EtcdClient) Read(path string) ([]byte, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedEtcdClient)
	}
	cntx, canceller := c.contextWithTimeout()
	defer canceller()
	log.Debugf("etcd read node %s", path)
	r, err := c.kapi.Get(cntx, path, nil)
	if err != nil && !isErrNoNode(err) {
		return nil, errors.Trace(err)
	} else if r == nil || r.Node.Dir {
		return nil, nil
	} else {
		return []byte(r.Node.Value), nil
	}
}

func (c *EtcdClient) List(path string) ([]string, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedEtcdClient)
	}
	cntx, canceller := c.contextWithTimeout()
	defer canceller()
	log.Debugf("etcd list node %s", path)
	r, err := c.kapi.Get(cntx, path, nil)
	if err != nil && !isErrNoNode(err) {
		return nil, errors.Trace(err)
	} else if r == nil || !r.Node.Dir {
		return nil, nil
	} else {
		var files []string
		for _, node := range r.Node.Nodes {
			files = append(files, node.Key)
		}
		return files, nil
	}
}
