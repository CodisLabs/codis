// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

var ErrClosedJodis = errors.New("use of closed jodis")

type Jodis struct {
	mu sync.Mutex

	path string
	data []byte

	client models.Client
	online bool
	closed bool

	watching bool
}

func NewJodis(c models.Client, p *models.Proxy) *Jodis {
	var m = map[string]string{
		"addr":  p.ProxyAddr,
		"admin": p.AdminAddr,
		"start": p.StartTime,
		"token": p.Token,
		"state": "online",
	}
	b, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	return &Jodis{path: p.JodisPath, data: b, client: c}
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

func (j *Jodis) Start() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.online {
		return
	}
	j.online = true

	go func() {
		var delay = &DelayExp2{
			Min: 1, Max: 30,
			Unit: time.Second,
		}
		for !j.IsClosed() {
			w, err := j.Rewatch()
			if err != nil {
				log.WarnErrorf(err, "jodis watch node %s failed", j.path)
				delay.SleepWithCancel(j.IsClosed)
			} else {
				<-w
				delay.Reset()
			}
		}
	}()
}
