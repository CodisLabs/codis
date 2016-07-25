// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"bytes"

	"github.com/BurntSushi/toml"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

const DefaultConfig = `
##################################################
#                                                #
#                  Codis-Proxy                   #
#                                                #
##################################################

# Set Codis Product {Name/Auth}.
product_name = "codis-demo"
product_auth = ""

# Set bind address for admin(rpc), tcp only.
admin_addr = "0.0.0.0:11080"

# Set bind address for proxy, proto_type can be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
proto_type = "tcp4"
proxy_addr = "0.0.0.0:19000"

# Set jodis address & session timeout, only accept "zookeeper" & "etcd".
jodis_name = ""
jodis_addr = ""
jodis_timeout = 20
jodis_compatible = 0

# Proxy will ping-pong backend redis periodly to keep-alive
backend_ping_period = 5

# If there is no request from client for a long time, the connection will be droped. Set 0 to disable.
session_max_timeout = 1800

# Buffer size for each client connection.
session_max_bufsize = 131072

# Number of buffered requests for each client connection.
# Make sure this is higher than the max number of requests for each pipeline request, or your client may be blocked.
session_max_pipeline = 1024

# Set period between keep alives. Set 0 to disable.
session_keepalive_period = 60

# Set max number of alive sessions. Set 0 to unlimited number (2147483647).
max_alive_sessions = 1000

# Set max offheap memory size (MB). Set 0 to disable.
max_offheap_mbytes = 1024
`

type Config struct {
	ProtoType string `toml:"proto_type" json:"proto_type"`
	ProxyAddr string `toml:"proxy_addr" json:"proxy_addr"`
	AdminAddr string `toml:"admin_addr" json:"admin_addr"`

	HostProxy string `toml:"-" json:"-"`
	HostAdmin string `toml:"-" json:"-"`

	JodisName       string `toml:"jodis_name" json:"jodis_name"`
	JodisAddr       string `toml:"jodis_addr" json:"jodis_addr"`
	JodisTimeout    int    `toml:"jodis_timeout" json:"jodis_timeout"`
	JodisCompatible int    `toml:"jodis_compatible" json:"jodis_compatible"`

	ProductName string `toml:"product_name" json:"product_name"`
	ProductAuth string `toml:"product_auth" json:"-"`

	BackendPingPeriod      int `toml:"backend_ping_period" json:"backend_ping_period"`
	SessionMaxTimeout      int `toml:"session_max_timeout" json:"session_max_timeout"`
	SessionMaxBufSize      int `toml:"session_max_bufsize" json:"session_max_bufsize"`
	SessionMaxPipeline     int `toml:"session_max_pipeline" json:"session_max_pipeline"`
	SessionKeepAlivePeriod int `toml:"session_keepalive_period" json:"session_keepalive_period"`

	MaxAliveSessions int `toml:"max_alive_sessions" json:"max_alive_sessions"`
	MaxOffheapMBytes int `toml:"max_offheap_mbytes" json:"max_offheap_mbytes"`
}

func NewDefaultConfig() *Config {
	c := &Config{}
	if _, err := toml.Decode(DefaultConfig, c); err != nil {
		log.PanicErrorf(err, "decode toml failed")
	}
	return c
}

func (c *Config) LoadFromFile(path string) error {
	_, err := toml.DecodeFile(path, c)
	return errors.Trace(err)
}

func (c *Config) String() string {
	var b bytes.Buffer
	e := toml.NewEncoder(&b)
	e.Indent = "    "
	e.Encode(c)
	return b.String()
}
