package redeo

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type clientSlice []*Client

func (p clientSlice) Len() int           { return len(p) }
func (p clientSlice) Less(i, j int) bool { return p[i].id < p[j].id }
func (p clientSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

var clientInc = uint64(0)

// A client is the origin of a request
type Client struct {
	Ctx interface{}

	id   uint64
	conn net.Conn

	firstAccess time.Time
	lastAccess  time.Time
	lastCommand string

	quit  bool
	mutex sync.Mutex
}

// NewClient creates a new client info container
func NewClient(conn net.Conn) *Client {
	now := time.Now()
	return &Client{
		id:          atomic.AddUint64(&clientInc, 1),
		conn:        conn,
		firstAccess: now,
		lastAccess:  now,
	}
}

// ID return the unique client id
func (i *Client) ID() uint64 { return i.id }

// RemoteAddr return the remote client address
func (i *Client) RemoteAddr() net.Addr { return i.conn.RemoteAddr() }

// Close will disconnect as soon as all pending replies have been written
// to the client
func (i *Client) Close() { i.quit = true }

// String generates an info string
func (i *Client) String() string {
	i.mutex.Lock()
	cmd := i.lastCommand
	atime := i.lastAccess
	i.mutex.Unlock()

	now := time.Now()
	age := now.Sub(i.firstAccess) / time.Second
	idle := now.Sub(atime) / time.Second

	return fmt.Sprintf("id=%d addr=%s age=%d idle=%d cmd=%s", i.id, i.RemoteAddr(), age, idle, cmd)
}

// ------------------------------------------------------------------------

// Instantly closes the underlying socket connection
func (i *Client) close() error { return i.conn.Close() }

// Tracks user command
func (i *Client) trackCommand(cmd string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.lastAccess = time.Now()
	i.lastCommand = cmd
}
