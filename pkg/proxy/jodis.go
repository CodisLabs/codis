// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/models/zk"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

var ErrClosedJodis = errors.New("use of closed jodis")

type Jodis struct {
	mu sync.Mutex

	addr string
	path string
	data []byte

	client  *zkclient.ZkClient
	closed  bool
	timeout time.Duration

	watching bool
}

func NewJodis(addr string, seconds int, compatible bool, s *models.Proxy) *Jodis {
	var m = map[string]string{
		"addr":     s.ProxyAddr,
		"start_at": s.StartTime,
		"token":    s.Token,
		"state":    "online",
	}
	b, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	var path string
	if compatible {
		path = filepath.Join("/zk/codis", fmt.Sprintf("db_%s", s.ProductName), "proxy", s.Token)
	} else {
		path = filepath.Join("/jodis", s.ProductName, fmt.Sprintf("proxy-%s", s.Token))
	}
	return &Jodis{path: path, data: b, addr: addr, timeout: time.Second * time.Duration(seconds)}
}

func (j *Jodis) Path() string {
	return j.path
}

func (j *Jodis) Data() string {
	return string(j.data)
}

func (j *Jodis) IsClosed() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.closed
}

func (j *Jodis) IsWatching() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.watching && !j.closed
}

func (j *Jodis) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true

	if j.client == nil {
		return nil
	}

	if j.watching {
		if err := j.client.Delete(j.path); err != nil {
			log.WarnErrorf(err, "jodis remove node %s failed", j.path)
		} else {
			log.Warnf("jodis remove node %s", j.path)
		}
	}
	return j.client.Close()
}

func (j *Jodis) Rewatch() (<-chan struct{}, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil, ErrClosedJodis
	}

	if j.client == nil {
		client, err := zkclient.New(j.addr, j.timeout)
		if err != nil {
			return nil, err
		}
		j.client = client
	}

	w, err := j.client.CreateEphemeral(j.path, j.data)
	if err != nil {
		log.WarnErrorf(err, "jodis create node %s failed", j.path)
		j.watching = false
	} else {
		log.Warnf("jodis create node %s", j.path)
		j.watching = true
	}
	return w, err
}

func (j *Jodis) Run() {
	var delay int
	for !j.IsClosed() {
		w, err := j.Rewatch()
		if err != nil {
			log.WarnErrorf(err, "jodis watch node %s failed", j.path)
			delay = delay * 2
			delay = utils.MaxInt(delay, 2)
			delay = utils.MinInt(delay, 30)
			for i := 0; i < delay && !j.IsClosed(); i++ {
				time.Sleep(time.Second)
			}
		} else {
			delay = 0
			<-w
		}
	}
}
