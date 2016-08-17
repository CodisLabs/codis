package redeo

import "time"

// Server configuration
type Config struct {

	// Accept connections on the specified port, default is 0.0.0.0:9736.
	// If not specified server will not listen on a TCP socket.
	Addr string

	// Specify the path for the unix socket that will be used to listen for
	// incoming connections. There is no default, so server will not listen
	// on a unix socket when not specified.
	Socket string

	// Close the connection after a client is idle for N seconds (0 to disable)
	Timeout time.Duration

	// If non-zero, use SO_KEEPALIVE to send TCP ACKs to clients in absence
	// of communication. This is useful for two reasons:
	// 1) Detect dead peers.
	// 2) Take the connection alive from the point of view of network
	//    equipment in the middle.
	// On Linux, the specified value (in seconds) is the period used to send ACKs.
	// Note that to close the connection the double of the time is needed.
	// On other kernels the period depends on the kernel configuration.
	TCPKeepAlive time.Duration
}

// Default configuration is used when nil is passed to NewServer
var DefaultConfig = &Config{
	Addr: "0.0.0.0:9736",
}
