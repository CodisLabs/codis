// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package etcdclient

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

var ErrClosedClient = errors.New("use of closed etcd client")

var (
	ErrNotDir  = errors.New("etcd: not a dir")
	ErrNotFile = errors.New("etcd: not a file")
)

type Client struct {
	sync.Mutex
	kapi client.KeysAPI

	closed  bool
	timeout time.Duration

	cancel  context.CancelFunc
	context context.Context
}

func New(addrlist string, auth string, timeout time.Duration) (*Client, error) {
	endpoints := strings.Split(addrlist, ",")
	for i, s := range endpoints {
		if s != "" && !strings.HasPrefix(s, "http://") {
			endpoints[i] = "http://" + s
		}
	}
	if timeout <= 0 {
		timeout = time.Second * 5
	}

	config := client.Config{
		Endpoints: endpoints, Transport: client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second * 5,
	}

	if auth != "" {
		split := strings.SplitN(auth, ":", 2)
		if len(split) != 2 || split[0] == "" {
			return nil, errors.Errorf("invalid auth")
		}
		config.Username = split[0]
		config.Password = split[1]
	}

	c, err := client.New(config)
	if err != nil {
		return nil, errors.Trace(err)
	}

	client := &Client{
		kapi: client.NewKeysAPI(c), timeout: timeout,
	}
	client.context, client.cancel = context.WithCancel(context.Background())
	return client, nil
}

func (c *Client) Close() error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.cancel()
	return nil
}

func (c *Client) newContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.context, c.timeout)
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

func (c *Client) Mkdir(path string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}
	log.Debugf("etcd mkdir node %s", path)
	cntx, cancel := c.newContext()
	defer cancel()
	_, err := c.kapi.Set(cntx, path, "", &client.SetOptions{Dir: true, PrevExist: client.PrevNoExist})
	if err != nil && !isErrNodeExists(err) {
		log.Debugf("etcd mkdir node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("etcd mkdir OK")
	return nil
}

func (c *Client) Create(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}
	cntx, cancel := c.newContext()
	defer cancel()
	log.Debugf("etcd create node %s", path)
	_, err := c.kapi.Set(cntx, path, string(data), &client.SetOptions{PrevExist: client.PrevNoExist})
	if err != nil {
		log.Debugf("etcd create node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("etcd create OK")
	return nil
}

func (c *Client) Update(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}
	cntx, cancel := c.newContext()
	defer cancel()
	log.Debugf("etcd update node %s", path)
	_, err := c.kapi.Set(cntx, path, string(data), &client.SetOptions{PrevExist: client.PrevIgnore})
	if err != nil {
		log.Debugf("etcd update node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("etcd update OK")
	return nil
}

func (c *Client) Delete(path string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}
	cntx, cancel := c.newContext()
	defer cancel()
	log.Debugf("etcd delete node %s", path)
	_, err := c.kapi.Delete(cntx, path, nil)
	if err != nil && !isErrNoNode(err) {
		log.Debugf("etcd delete node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("etcd delete OK")
	return nil
}

func (c *Client) Read(path string, must bool) ([]byte, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedClient)
	}
	cntx, cancel := c.newContext()
	defer cancel()
	r, err := c.kapi.Get(cntx, path, &client.GetOptions{Quorum: true})
	switch {
	case err != nil:
		if isErrNoNode(err) && !must {
			return nil, nil
		}
		log.Debugf("etcd read node %s failed: %s", path, err)
		return nil, errors.Trace(err)
	case !r.Node.Dir:
		return []byte(r.Node.Value), nil
	default:
		log.Debugf("etcd read node %s failed: not a file", path)
		return nil, errors.Trace(ErrNotFile)
	}
}

func (c *Client) List(path string, must bool) ([]string, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedClient)
	}
	cntx, cancel := c.newContext()
	defer cancel()
	r, err := c.kapi.Get(cntx, path, &client.GetOptions{Quorum: true})
	switch {
	case err != nil:
		if isErrNoNode(err) && !must {
			return nil, nil
		}
		log.Debugf("etcd list node %s failed: %s", path, err)
		return nil, errors.Trace(err)
	case !r.Node.Dir:
		log.Debugf("etcd list node %s failed: not a dir", path)
		return nil, errors.Trace(ErrNotDir)
	default:
		var paths []string
		for _, node := range r.Node.Nodes {
			paths = append(paths, node.Key)
		}
		return paths, nil
	}
}

func (c *Client) CreateEphemeral(path string, data []byte) (<-chan struct{}, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedClient)
	}
	cntx, cancel := c.newContext()
	defer cancel()
	log.Debugf("etcd create-ephemeral node %s", path)
	_, err := c.kapi.Set(cntx, path, string(data), &client.SetOptions{PrevExist: client.PrevNoExist, TTL: c.timeout})
	if err != nil {
		log.Debugf("etcd create-ephemeral node %s failed: %s", path, err)
		return nil, errors.Trace(err)
	}
	log.Debugf("etcd create-ephemeral OK")
	return runRefreshEphemeral(c, path), nil
}

func (c *Client) CreateEphemeralInOrder(path string, data []byte) (<-chan struct{}, string, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, "", errors.Trace(ErrClosedClient)
	}
	cntx, cancel := c.newContext()
	defer cancel()
	log.Debugf("etcd create-ephemeral-inorder node %s", path)
	r, err := c.kapi.CreateInOrder(cntx, path, string(data), &client.CreateInOrderOptions{TTL: c.timeout})
	if err != nil {
		log.Debugf("etcd create-ephemeral-inorder node %s failed: %s", path, err)
		return nil, "", errors.Trace(err)
	}
	node := r.Node.Key
	log.Debugf("etcd create-ephemeral-inorder OK, node = %s", node)
	return runRefreshEphemeral(c, node), node, nil
}

func runRefreshEphemeral(c *Client, path string) <-chan struct{} {
	signal := make(chan struct{})
	go func() {
		defer close(signal)
		for {
			if err := c.RefreshEphemeral(path); err != nil {
				return
			} else {
				time.Sleep(c.timeout / 2)
			}
		}
	}()
	return signal
}

func (c *Client) RefreshEphemeral(path string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}
	cntx, cancel := c.newContext()
	defer cancel()
	log.Debugf("etcd refresh-ephemeral node %s", path)
	_, err := c.kapi.Set(cntx, path, "", &client.SetOptions{PrevExist: client.PrevExist, Refresh: true, TTL: c.timeout})
	if err != nil {
		log.Debugf("etcd refresh-ephemeral node %s failed: %s", path, err)
		return errors.Trace(err)
	}
	log.Debugf("etcd refresh-ephemeral OK")
	return nil
}

func (c *Client) WatchInOrder(path string) (<-chan struct{}, []string, error) {
	if err := c.Mkdir(path); err != nil {
		return nil, nil, err
	}
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, nil, errors.Trace(ErrClosedClient)
	}
	log.Debugf("etcd watch-inorder node %s", path)
	cntx, cancel := c.newContext()
	defer cancel()
	r, err := c.kapi.Get(cntx, path, &client.GetOptions{Quorum: true, Sort: true})
	switch {
	case err != nil:
		log.Debugf("etcd watch-inorder node %s failed: %s", path, err)
		return nil, nil, errors.Trace(err)
	case !r.Node.Dir:
		log.Debugf("etcd watch-inorder node %s failed: not a dir", path)
		return nil, nil, errors.Trace(ErrNotDir)
	}
	var index = r.Index
	var paths []string
	for _, node := range r.Node.Nodes {
		paths = append(paths, node.Key)
	}
	signal := make(chan struct{})
	go func() {
		defer close(signal)
		watch := c.kapi.Watcher(path, &client.WatcherOptions{AfterIndex: index})
		for {
			r, err := watch.Next(c.context)
			switch {
			case err != nil:
				log.Debugf("etch watch-inorder node %s failed: %s", path, err)
				return
			case r.Action != "get":
				log.Debugf("etcd watch-inorder node %s update", path)
				return
			}
		}
	}()
	log.Debugf("etcd watch-inorder OK")
	return signal, paths, nil
}
