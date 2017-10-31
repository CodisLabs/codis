// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"bytes"

	"github.com/BurntSushi/toml"

	"github.com/CodisLabs/codis/pkg/utils/bytesize"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/timesize"
)

const DefaultConfig = `
##################################################
#                                                #
#                  Codis-Proxy                   #
#                                                #
##################################################

# Set Codis Product Name/Auth.
product_name = "codis-demo"
product_auth = ""

# Set auth for client session
#   1. product_auth is used for auth validation among codis-dashboard,
#      codis-proxy and codis-server.
#   2. session_auth is different from product_auth, it requires clients
#      to issue AUTH <PASSWORD> before processing any other commands.
session_auth = ""

# Set bind address for admin(rpc), tcp only.
admin_addr = "0.0.0.0:11080"

# Set bind address for proxy, proto_type can be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
proto_type = "tcp4"
proxy_addr = "0.0.0.0:19000"

# Set jodis address & session timeout
#   1. jodis_name is short for jodis_coordinator_name, only accept "zookeeper" & "etcd".
#   2. jodis_addr is short for jodis_coordinator_addr
#   3. jodis_auth is short for jodis_coordinator_auth, for zookeeper/etcd, "user:password" is accepted.
#   4. proxy will be registered as node:
#        if jodis_compatible = true (not suggested):
#          /zk/codis/db_{PRODUCT_NAME}/proxy-{HASHID} (compatible with Codis2.0)
#        or else
#          /jodis/{PRODUCT_NAME}/proxy-{HASHID}
jodis_name = ""
jodis_addr = ""
jodis_auth = ""
jodis_timeout = "20s"
jodis_compatible = false

# Set datacenter of proxy.
proxy_datacenter = ""

# Set max number of alive sessions.
proxy_max_clients = 1000

# Set max offheap memory size. (0 to disable)
proxy_max_offheap_size = "1024mb"

# Set heap placeholder to reduce GC frequency.
proxy_heap_placeholder = "256mb"

# Proxy will ping backend redis (and clear 'MASTERDOWN' state) in a predefined interval. (0 to disable)
backend_ping_period = "5s"

# Set backend recv buffer size & timeout.
backend_recv_bufsize = "128kb"
backend_recv_timeout = "30s"

# Set backend send buffer & timeout.
backend_send_bufsize = "128kb"
backend_send_timeout = "30s"

# Set backend pipeline buffer size.
backend_max_pipeline = 20480

# Set backend never read replica groups, default is false
backend_primary_only = false

# Set backend parallel connections per server
backend_primary_parallel = 1
backend_replica_parallel = 1

# Set backend tcp keepalive period. (0 to disable)
backend_keepalive_period = "75s"

# Set number of databases of backend.
backend_number_databases = 16

# If there is no request from client for a long time, the connection will be closed. (0 to disable)
# Set session recv buffer size & timeout.
session_recv_bufsize = "128kb"
session_recv_timeout = "30m"

# Set session send buffer size & timeout.
session_send_bufsize = "64kb"
session_send_timeout = "30s"

# Make sure this is higher than the max number of requests for each pipeline request, or your client may be blocked.
# Set session pipeline buffer size.
session_max_pipeline = 10000

# Set session tcp keepalive period. (0 to disable)
session_keepalive_period = "75s"

# Set session to be sensitive to failures. Default is false, instead of closing socket, proxy will send an error response to client.
session_break_on_failure = false

# Set metrics server (such as http://localhost:28000), proxy will report json formatted metrics to specified server in a predefined period.
metrics_report_server = ""
metrics_report_period = "1s"

# Set influxdb server (such as http://localhost:8086), proxy will report metrics to influxdb.
metrics_report_influxdb_server = ""
metrics_report_influxdb_period = "1s"
metrics_report_influxdb_username = ""
metrics_report_influxdb_password = ""
metrics_report_influxdb_database = ""

# Set statsd server (such as localhost:8125), proxy will report metrics to statsd.
metrics_report_statsd_server = ""
metrics_report_statsd_period = "1s"
metrics_report_statsd_prefix = ""
`

type Config struct {
	ProtoType string `toml:"proto_type" json:"proto_type"`
	ProxyAddr string `toml:"proxy_addr" json:"proxy_addr"`
	AdminAddr string `toml:"admin_addr" json:"admin_addr"`

	HostProxy string `toml:"-" json:"-"`
	HostAdmin string `toml:"-" json:"-"`

	JodisName       string            `toml:"jodis_name" json:"jodis_name"`
	JodisAddr       string            `toml:"jodis_addr" json:"jodis_addr"`
	JodisAuth       string            `toml:"jodis_auth" json:"jodis_auth"`
	JodisTimeout    timesize.Duration `toml:"jodis_timeout" json:"jodis_timeout"`
	JodisCompatible bool              `toml:"jodis_compatible" json:"jodis_compatible"`

	ProductName string `toml:"product_name" json:"product_name"`
	ProductAuth string `toml:"product_auth" json:"-"`
	SessionAuth string `toml:"session_auth" json:"-"`

	ProxyDataCenter      string         `toml:"proxy_datacenter" json:"proxy_datacenter"`
	ProxyMaxClients      int            `toml:"proxy_max_clients" json:"proxy_max_clients"`
	ProxyMaxOffheapBytes bytesize.Int64 `toml:"proxy_max_offheap_size" json:"proxy_max_offheap_size"`
	ProxyHeapPlaceholder bytesize.Int64 `toml:"proxy_heap_placeholder" json:"proxy_heap_placeholder"`

	BackendPingPeriod      timesize.Duration `toml:"backend_ping_period" json:"backend_ping_period"`
	BackendRecvBufsize     bytesize.Int64    `toml:"backend_recv_bufsize" json:"backend_recv_bufsize"`
	BackendRecvTimeout     timesize.Duration `toml:"backend_recv_timeout" json:"backend_recv_timeout"`
	BackendSendBufsize     bytesize.Int64    `toml:"backend_send_bufsize" json:"backend_send_bufsize"`
	BackendSendTimeout     timesize.Duration `toml:"backend_send_timeout" json:"backend_send_timeout"`
	BackendMaxPipeline     int               `toml:"backend_max_pipeline" json:"backend_max_pipeline"`
	BackendPrimaryOnly     bool              `toml:"backend_primary_only" json:"backend_primary_only"`
	BackendPrimaryParallel int               `toml:"backend_primary_parallel" json:"backend_primary_parallel"`
	BackendReplicaParallel int               `toml:"backend_replica_parallel" json:"backend_replica_parallel"`
	BackendKeepAlivePeriod timesize.Duration `toml:"backend_keepalive_period" json:"backend_keepalive_period"`
	BackendNumberDatabases int32             `toml:"backend_number_databases" json:"backend_number_databases"`

	SessionRecvBufsize     bytesize.Int64    `toml:"session_recv_bufsize" json:"session_recv_bufsize"`
	SessionRecvTimeout     timesize.Duration `toml:"session_recv_timeout" json:"session_recv_timeout"`
	SessionSendBufsize     bytesize.Int64    `toml:"session_send_bufsize" json:"session_send_bufsize"`
	SessionSendTimeout     timesize.Duration `toml:"session_send_timeout" json:"session_send_timeout"`
	SessionMaxPipeline     int               `toml:"session_max_pipeline" json:"session_max_pipeline"`
	SessionKeepAlivePeriod timesize.Duration `toml:"session_keepalive_period" json:"session_keepalive_period"`
	SessionBreakOnFailure  bool              `toml:"session_break_on_failure" json:"session_break_on_failure"`

	MetricsReportServer           string            `toml:"metrics_report_server" json:"metrics_report_server"`
	MetricsReportPeriod           timesize.Duration `toml:"metrics_report_period" json:"metrics_report_period"`
	MetricsReportInfluxdbServer   string            `toml:"metrics_report_influxdb_server" json:"metrics_report_influxdb_server"`
	MetricsReportInfluxdbPeriod   timesize.Duration `toml:"metrics_report_influxdb_period" json:"metrics_report_influxdb_period"`
	MetricsReportInfluxdbUsername string            `toml:"metrics_report_influxdb_username" json:"metrics_report_influxdb_username"`
	MetricsReportInfluxdbPassword string            `toml:"metrics_report_influxdb_password" json:"-"`
	MetricsReportInfluxdbDatabase string            `toml:"metrics_report_influxdb_database" json:"metrics_report_influxdb_database"`
	MetricsReportStatsdServer     string            `toml:"metrics_report_statsd_server" json:"metrics_report_statsd_server"`
	MetricsReportStatsdPeriod     timesize.Duration `toml:"metrics_report_statsd_period" json:"metrics_report_statsd_period"`
	MetricsReportStatsdPrefix     string            `toml:"metrics_report_statsd_prefix" json:"metrics_report_statsd_prefix"`
}

func NewDefaultConfig() *Config {
	c := &Config{}
	if _, err := toml.Decode(DefaultConfig, c); err != nil {
		log.PanicErrorf(err, "decode toml failed")
	}
	if err := c.Validate(); err != nil {
		log.PanicErrorf(err, "validate config failed")
	}
	return c
}

func (c *Config) LoadFromFile(path string) error {
	_, err := toml.DecodeFile(path, c)
	if err != nil {
		return errors.Trace(err)
	}
	return c.Validate()
}

func (c *Config) String() string {
	var b bytes.Buffer
	e := toml.NewEncoder(&b)
	e.Indent = "    "
	e.Encode(c)
	return b.String()
}

func (c *Config) Validate() error {
	if c.ProtoType == "" {
		return errors.New("invalid proto_type")
	}
	if c.ProxyAddr == "" {
		return errors.New("invalid proxy_addr")
	}
	if c.AdminAddr == "" {
		return errors.New("invalid admin_addr")
	}
	if c.JodisName != "" {
		if c.JodisAddr == "" {
			return errors.New("invalid jodis_addr")
		}
		if c.JodisTimeout < 0 {
			return errors.New("invalid jodis_timeout")
		}
	}
	if c.ProductName == "" {
		return errors.New("invalid product_name")
	}
	if c.ProxyMaxClients < 0 {
		return errors.New("invalid proxy_max_clients")
	}

	const MaxInt = bytesize.Int64(^uint(0) >> 1)

	if d := c.ProxyMaxOffheapBytes; d < 0 || d > MaxInt {
		return errors.New("invalid proxy_max_offheap_size")
	}
	if d := c.ProxyHeapPlaceholder; d < 0 || d > MaxInt {
		return errors.New("invalid proxy_heap_placeholder")
	}
	if c.BackendPingPeriod < 0 {
		return errors.New("invalid backend_ping_period")
	}

	if d := c.BackendRecvBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid backend_recv_bufsize")
	}
	if c.BackendRecvTimeout < 0 {
		return errors.New("invalid backend_recv_timeout")
	}
	if d := c.BackendSendBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid backend_send_bufsize")
	}
	if c.BackendSendTimeout < 0 {
		return errors.New("invalid backend_send_timeout")
	}
	if c.BackendMaxPipeline < 0 {
		return errors.New("invalid backend_max_pipeline")
	}
	if c.BackendPrimaryParallel < 0 {
		return errors.New("invalid backend_primary_parallel")
	}
	if c.BackendReplicaParallel < 0 {
		return errors.New("invalid backend_replica_parallel")
	}
	if c.BackendKeepAlivePeriod < 0 {
		return errors.New("invalid backend_keepalive_period")
	}
	if c.BackendNumberDatabases < 1 {
		return errors.New("invalid backend_number_databases")
	}

	if d := c.SessionRecvBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid session_recv_bufsize")
	}
	if c.SessionRecvTimeout < 0 {
		return errors.New("invalid session_recv_timeout")
	}
	if d := c.SessionSendBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid session_send_bufsize")
	}
	if c.SessionSendTimeout < 0 {
		return errors.New("invalid session_send_timeout")
	}
	if c.SessionMaxPipeline < 0 {
		return errors.New("invalid session_max_pipeline")
	}
	if c.SessionKeepAlivePeriod < 0 {
		return errors.New("invalid session_keepalive_period")
	}

	if c.MetricsReportPeriod < 0 {
		return errors.New("invalid metrics_report_period")
	}
	if c.MetricsReportInfluxdbPeriod < 0 {
		return errors.New("invalid metrics_report_influxdb_period")
	}
	if c.MetricsReportStatsdPeriod < 0 {
		return errors.New("invalid metrics_report_statsd_period")
	}
	return nil
}
