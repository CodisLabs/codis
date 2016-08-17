package redeo

import (
	"bufio"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// Server configuration
type Server struct {
	config   *Config
	info     *ServerInfo
	commands map[string]Handler

	tcp, unix net.Listener
	clients   *clients
}

// NewServer creates a new server instance
func NewServer(config *Config) *Server {
	if config == nil {
		config = DefaultConfig
	}

	clients := newClientRegistry()
	return &Server{
		config:   config,
		clients:  clients,
		info:     newServerInfo(config, clients),
		commands: make(map[string]Handler),
	}
}

// Addr returns the server TCP address
func (srv *Server) Addr() string {
	return srv.config.Addr
}

// Socket returns the server UNIX socket address
func (srv *Server) Socket() string {
	return srv.config.Socket
}

// Info returns the server info registry
func (srv *Server) Info() *ServerInfo {
	return srv.info
}

// Close shuts down the server and closes all connections
func (srv *Server) Close() (err error) {

	// Stop new TCP connections
	if srv.tcp != nil {
		if e := srv.tcp.Close(); e != nil {
			err = e
		}
		srv.tcp = nil
	}

	// Stop new Unix socket connections
	if srv.unix != nil {
		if e := srv.unix.Close(); e != nil {
			err = e
		}
		srv.unix = nil
	}

	// Terminate all clients
	if e := srv.clients.Clear(); err != nil {
		err = e
	}

	return
}

// Handle registers a handler for a command.
// Not thread-safe, don't call from multiple goroutines
func (srv *Server) Handle(name string, handler Handler) {
	srv.commands[strings.ToLower(name)] = handler
}

// HandleFunc registers a handler callback for a command
func (srv *Server) HandleFunc(name string, callback HandlerFunc) {
	srv.Handle(name, Handler(callback))
}

// ListenAndServe starts the server
func (srv *Server) ListenAndServe() (err error) {
	errs := make(chan error, 2)

	if srv.Addr() != "" {
		srv.tcp, err = net.Listen("tcp", srv.Addr())
		if err != nil {
			return
		}
		go func() { errs <- srv.Serve(srv.tcp) }()
	}

	if srv.Socket() != "" {
		srv.unix, err = srv.listenUnix()
		if err != nil {
			return err
		}
		go func() { errs <- srv.Serve(srv.unix) }()
	}

	return <-errs
}

// ------------------------------------------------------------------------

// Applies a request. Returns true when we should continue the client connection
func (srv *Server) apply(req *Request, w io.Writer) bool {
	res := NewResponder(w)
	cmd, ok := srv.commands[req.Name]
	if !ok {
		res.WriteError(UnknownCommand(req.Name))
		return true
	}

	srv.info.onCommand()
	if req.client != nil {
		req.client.trackCommand(req.Name)
	}

	err := cmd.ServeClient(res, req)
	if !res.written {
		if err != nil {
			res.WriteError(err)
		} else {
			res.WriteOK()
		}
	}
	return res.Valid()
}

// Serve accepts incoming connections on a listener, creating a
// new service goroutine for each.
func (srv *Server) Serve(lis net.Listener) error {
	defer lis.Close()

	for {
		conn, err := lis.Accept()
		if err != nil {
			return err
		}
		go srv.serveClient(NewClient(conn))
	}
	return nil
}

// Starts a new session, serving client
func (srv *Server) serveClient(client *Client) {
	// Register client
	srv.clients.Put(client)
	defer srv.clients.Close(client.id)

	// Track connection
	srv.info.onConnect()

	// Apply TCP keep-alive, if configured
	if alive := srv.config.TCPKeepAlive; alive > 0 {
		if tcpconn, ok := client.conn.(*net.TCPConn); ok {
			tcpconn.SetKeepAlive(true)
			tcpconn.SetKeepAlivePeriod(alive)
		}
	}

	// Init request/response loop
	reader := bufio.NewReader(client.conn)
	for {
		if timeout := srv.config.Timeout; timeout > 0 {
			client.conn.SetDeadline(time.Now().Add(timeout))
		}

		req, err := ParseRequest(reader)
		if err != nil {
			NewResponder(client.conn).WriteError(err)
			return
		}
		req.client = client

		ok := srv.apply(req, client.conn)
		if !ok || client.quit {
			return
		}
	}
}

// listenUnix starts the unix listener on socket path
func (srv *Server) listenUnix() (net.Listener, error) {
	if stat, err := os.Stat(srv.Socket()); !os.IsNotExist(err) && !stat.IsDir() {
		if err = os.RemoveAll(srv.Socket()); err != nil {
			return nil, err
		}
	}
	return net.Listen("unix", srv.Socket())
}
