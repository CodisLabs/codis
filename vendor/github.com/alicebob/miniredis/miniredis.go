// Package miniredis is a pure Go Redis test server, for use in Go unittests.
// There are no dependencies on system binaries, and every server you start
// will be empty.
//
// Start a server with `s, err := miniredis.Run()`.
// Stop it with `defer s.Close()`.
//
// Point your Redis client to `s.Addr()` or `s.Host(), s.Port()`.
//
// Set keys directly via s.Set(...) and similar commands, or use a Redis client.
//
// For direct use you can select a Redis database with either `s.Select(12);
// s.Get("foo")` or `s.DB(12).Get("foo")`.
//
package miniredis

import (
	"fmt"
	"net"
	"sync"

	"github.com/bsm/redeo"
)

type hashKey map[string]string
type listKey []string
type setKey map[string]struct{}

// RedisDB holds a single (numbered) Redis database.
type RedisDB struct {
	master        *sync.Mutex          // pointer to the lock in Miniredis
	id            int                  // db id
	keys          map[string]string    // Master map of keys with their type
	stringKeys    map[string]string    // GET/SET &c. keys
	hashKeys      map[string]hashKey   // MGET/MSET &c. keys
	listKeys      map[string]listKey   // LPUSH &c. keys
	setKeys       map[string]setKey    // SADD &c. keys
	sortedsetKeys map[string]sortedSet // ZADD &c. keys
	expire        map[string]int       // EXPIRE values
	keyVersion    map[string]uint      // used to watch values
}

// Miniredis is a Redis server implementation.
type Miniredis struct {
	sync.Mutex
	srv        *redeo.Server
	listenAddr string
	password   string
	closed     chan struct{}
	listen     net.Listener
	dbs        map[int]*RedisDB
	selectedDB int // DB id used in the direct Get(), Set() &c.
}

type txCmd func(*redeo.Responder, *connCtx)

// database id + key combo
type dbKey struct {
	db  int
	key string
}

// connCtx has all state for a single connection.
type connCtx struct {
	selectedDB       int            // selected DB
	authenticated    bool           // auth enabled and a valid AUTH seen
	transaction      []txCmd        // transaction callbacks. Or nil.
	dirtyTransaction bool           // any error during QUEUEing.
	watch            map[dbKey]uint // WATCHed keys.
}

// NewMiniRedis makes a new, non-started, Miniredis object.
func NewMiniRedis() *Miniredis {
	return &Miniredis{
		closed: make(chan struct{}),
		dbs:    map[int]*RedisDB{},
	}
}

func newRedisDB(id int, l *sync.Mutex) RedisDB {
	return RedisDB{
		id:            id,
		master:        l,
		keys:          map[string]string{},
		stringKeys:    map[string]string{},
		hashKeys:      map[string]hashKey{},
		listKeys:      map[string]listKey{},
		setKeys:       map[string]setKey{},
		sortedsetKeys: map[string]sortedSet{},
		expire:        map[string]int{},
		keyVersion:    map[string]uint{},
	}
}

// Run creates and Start()s a Miniredis.
func Run() (*Miniredis, error) {
	m := NewMiniRedis()
	return m, m.Start()
}

// Start starts a server. It listens on a random port on localhost. See also
// Addr().
func (m *Miniredis) Start() error {
	m.Lock()
	defer m.Unlock()

	l, err := listen("127.0.0.1:0")
	if err != nil {
		return err
	}
	m.listen = l
	m.listenAddr = l.Addr().String()
	m.srv = redeo.NewServer(&redeo.Config{Addr: m.listenAddr})

	commandsConnection(m, m.srv)
	commandsGeneric(m, m.srv)
	commandsServer(m, m.srv)
	commandsString(m, m.srv)
	commandsHash(m, m.srv)
	commandsList(m, m.srv)
	commandsSet(m, m.srv)
	commandsSortedSet(m, m.srv)
	commandsTransaction(m, m.srv)

	go func() {
		m.srv.Serve(m.listen)
		m.closed <- struct{}{}
	}()
	return nil
}

// Restart restarts a Close()d server on the same port. Values will be
// preserved.
func (m *Miniredis) Restart() error {
	m.Lock()
	defer m.Unlock()

	l, err := listen(m.listenAddr)
	if err != nil {
		return err
	}
	m.listen = l

	go func() {
		m.srv.Serve(m.listen)
		m.closed <- struct{}{}
	}()

	return nil
}

func listen(addr string) (net.Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		if l, err = net.Listen("tcp6", addr); err != nil {
			return nil, fmt.Errorf("failed to listen on a port: %v", err)
		}
	}
	return l, nil
}

// Close shuts down a Miniredis.
func (m *Miniredis) Close() {
	m.Lock()
	defer m.Unlock()
	if m.listen == nil {
		return
	}
	if m.listen.Close() != nil {
		return
	}
	m.srv.Close()
	<-m.closed
	m.listen = nil
}

// RequireAuth makes every connection need to AUTH first. Disable again by
// setting an empty string.
func (m *Miniredis) RequireAuth(pw string) {
	m.Lock()
	defer m.Unlock()
	m.password = pw
}

// DB returns a DB by ID.
func (m *Miniredis) DB(i int) *RedisDB {
	m.Lock()
	defer m.Unlock()
	return m.db(i)
}

// get DB. No locks!
func (m *Miniredis) db(i int) *RedisDB {
	if db, ok := m.dbs[i]; ok {
		return db
	}
	db := newRedisDB(i, &m.Mutex) // the DB has our lock.
	m.dbs[i] = &db
	return &db
}

// Addr returns '127.0.0.1:12345'. Can be given to a Dial(). See also Host()
// and Port(), which return the same things.
func (m *Miniredis) Addr() string {
	m.Lock()
	defer m.Unlock()
	return m.listenAddr
}

// Host returns the host part of Addr().
func (m *Miniredis) Host() string {
	m.Lock()
	defer m.Unlock()
	host, _, _ := net.SplitHostPort(m.listenAddr)
	return host
}

// Port returns the (random) port part of Addr().
func (m *Miniredis) Port() string {
	m.Lock()
	defer m.Unlock()
	_, port, _ := net.SplitHostPort(m.listenAddr)
	return port
}

// CommandCount returns the number of processed commands.
func (m *Miniredis) CommandCount() int {
	m.Lock()
	defer m.Unlock()
	return int(m.srv.Info().TotalCommands())
}

// CurrentConnectionCount returns the number of currently connected clients.
func (m *Miniredis) CurrentConnectionCount() int {
	m.Lock()
	defer m.Unlock()
	return m.srv.Info().ClientsLen()
}

// TotalConnectionCount returns the number of client connections since server start.
func (m *Miniredis) TotalConnectionCount() int {
	m.Lock()
	defer m.Unlock()
	return int(m.srv.Info().TotalConnections())
}

// Dump returns a text version of the selected DB, usable for debugging.
func (m *Miniredis) Dump() string {
	m.Lock()
	defer m.Unlock()

	var (
		maxLen = 60
		indent = "   "
		db     = m.db(m.selectedDB)
		r      = ""
		v      = func(s string) string {
			suffix := ""
			if len(s) > maxLen {
				suffix = fmt.Sprintf("...(%d)", len(s))
				s = s[:maxLen-len(suffix)]
			}
			return fmt.Sprintf("%q%s", s, suffix)
		}
	)
	for _, k := range db.allKeys() {
		r += fmt.Sprintf("- %s\n", k)
		t := db.t(k)
		switch t {
		case "string":
			r += fmt.Sprintf("%s%s\n", indent, v(db.stringKeys[k]))
		case "hash":
			for _, hk := range db.hashFields(k) {
				r += fmt.Sprintf("%s%s: %s\n", indent, hk, v(db.hashGet(k, hk)))
			}
		case "list":
			for _, lk := range db.listKeys[k] {
				r += fmt.Sprintf("%s%s\n", indent, v(lk))
			}
		case "set":
			for _, mk := range db.setMembers(k) {
				r += fmt.Sprintf("%s%s\n", indent, v(mk))
			}
		case "zset":
			for _, el := range db.ssetElements(k) {
				r += fmt.Sprintf("%s%f: %s\n", indent, el.score, v(el.member))
			}
		default:
			r += fmt.Sprintf("%s(a %s, fixme!)\n", indent, t)
		}
	}
	return r
}

// handleAuth returns false if connection has no access. It sends the reply.
func (m *Miniredis) handleAuth(cl *redeo.Client, out *redeo.Responder) bool {
	m.Lock()
	defer m.Unlock()
	if m.password == "" {
		return true
	}
	if cl.Ctx == nil || !getCtx(cl).authenticated {
		out.WriteErrorString("NOAUTH Authentication required.")
		return false
	}
	return true
}

func getCtx(cl *redeo.Client) *connCtx {
	if cl.Ctx == nil {
		cl.Ctx = &connCtx{}
	}
	return cl.Ctx.(*connCtx)
}

func startTx(ctx *connCtx) {
	ctx.transaction = []txCmd{}
	ctx.dirtyTransaction = false
}

func stopTx(ctx *connCtx) {
	ctx.transaction = nil
	unwatch(ctx)
}

func inTx(ctx *connCtx) bool {
	return ctx.transaction != nil
}

func addTxCmd(ctx *connCtx, cb txCmd) {
	ctx.transaction = append(ctx.transaction, cb)
}

func dirtyTx(ctx *connCtx) bool {
	return ctx.dirtyTransaction
}

func watch(db *RedisDB, ctx *connCtx, key string) {
	if ctx.watch == nil {
		ctx.watch = map[dbKey]uint{}
	}
	ctx.watch[dbKey{db: db.id, key: key}] = db.keyVersion[key] // Can be 0.
}

func unwatch(ctx *connCtx) {
	ctx.watch = nil
}

// setDirty can be called even when not in an tx. Is an no-op then.
func setDirty(cl *redeo.Client) {
	if cl.Ctx == nil {
		// No transaction. Not relevant.
		return
	}
	getCtx(cl).dirtyTransaction = true
}

func setAuthenticated(cl *redeo.Client) {
	getCtx(cl).authenticated = true
}
