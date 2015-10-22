package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/models/store/zk"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

var ErrClosedJodis = errors.New("use of closed jodis")

type Jodis struct {
	mu sync.Mutex

	path string
	data []byte
	addr []string

	client *zkstore.ZkClient
	closed bool

	watching bool
}

func NewJodis(addr []string, s *models.Proxy) *Jodis {
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
	p := filepath.Join("/zk/codis", fmt.Sprintf("db_%s", s.ProductName), "proxy", s.Token)
	return &Jodis{path: p, data: b, addr: addr}
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
			log.Infof("jodis remove node %s", j.path)
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
		timeout := time.Second * 10
		client, err := zkstore.NewClient(j.addr, timeout, zkstore.DefaultLogfunc)
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
		log.Infof("jodis create node %s", j.path)
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
			delay = utils.MinInt(delay, 20)
			for i := 0; i < delay && !j.IsClosed(); i++ {
				time.Sleep(time.Second)
			}
		} else {
			delay = 0
			<-w
		}
	}
}
