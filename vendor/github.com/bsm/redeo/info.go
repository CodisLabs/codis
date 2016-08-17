package redeo

import (
	"net"
	"os"
	"strconv"
	"time"

	"github.com/bsm/redeo/info"
)

type ServerInfo struct {
	registry *info.Registry

	startTime time.Time
	port      string
	socket    string
	pid       int

	clients     *clients
	connections *info.Counter
	commands    *info.Counter
}

// newServerInfo creates a new server info container
func newServerInfo(config *Config, clients *clients) *ServerInfo {
	info := &ServerInfo{
		registry:    info.New(),
		startTime:   time.Now(),
		connections: info.NewCounter(),
		commands:    info.NewCounter(),
		clients:     clients,
	}
	return info.withDefaults(config)
}

// ------------------------------------------------------------------------

// Section finds-or-creates an info section
func (i *ServerInfo) Section(name string) *info.Section { return i.registry.Section(name) }

// String generates an info string
func (i *ServerInfo) String() string { return i.registry.String() }

// ClientsLen returns the number of connected clients
func (i *ServerInfo) ClientsLen() int { return i.clients.Len() }

// Clients generates a slice of connected clients
func (i *ServerInfo) Clients() []*Client { return i.clients.All() }

// ClientsString generates a client list
func (i *ServerInfo) ClientsString() string {
	str := ""
	for _, client := range i.Clients() {
		str += client.String() + "\n"
	}
	return str
}

// TotalConnections returns the total number of connections made since the
// start of the server.
func (i *ServerInfo) TotalConnections() int64 { return i.connections.Value() }

// TotalCommands returns the total number of commands executed since the start
// of the server.
func (i *ServerInfo) TotalCommands() int64 { return i.commands.Value() }

// ------------------------------------------------------------------------

// Apply default info
func (i *ServerInfo) withDefaults(config *Config) *ServerInfo {
	_, port, _ := net.SplitHostPort(config.Addr)

	server := i.Section("Server")
	server.Register("process_id", info.PlainInt(os.Getpid()))
	server.Register("tcp_port", info.PlainString(port))
	server.Register("unix_socket", info.PlainString(config.Socket))
	server.Register("uptime_in_seconds", info.Callback(func() string {
		d := time.Now().Sub(i.startTime) / time.Second
		return strconv.FormatInt(int64(d), 10)
	}))
	server.Register("uptime_in_days", info.Callback(func() string {
		d := time.Now().Sub(i.startTime) / time.Hour / 24
		return strconv.FormatInt(int64(d), 10)
	}))

	clients := i.Section("Clients")
	clients.Register("connected_clients", info.Callback(func() string {
		return strconv.Itoa(i.ClientsLen())
	}))

	stats := i.Section("Stats")
	stats.Register("total_connections_received", i.connections)
	stats.Register("total_commands_processed", i.commands)

	return i
}

// Callback to register a new client connection
func (i *ServerInfo) onConnect() { i.connections.Inc(1) }

// Callback to track processed command
func (i *ServerInfo) onCommand() { i.commands.Inc(1) }
