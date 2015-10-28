package proxy

import (
	"bytes"

	"github.com/BurntSushi/toml"

	"github.com/wandoulabs/codis/pkg/utils/errors"
)

type Config struct {
	ProtoType string `toml:"proto_type" json:"proto_type"`
	ProxyAddr string `toml:"proxy_addr" json:"proxy_addr"`
	AdminAddr string `toml:"admin_addr" json:"admin_addr"`

	JodisAddr    string `toml:"jodis_addr" json:"jodis_addr"`
	JodisTimeout int    `toml:"jodis_timeout" json:"jodis_timeout"`

	ProductName string `toml:"product_name" json:"product_name"`
	ProductAuth string `toml:"product_auth" json:"-"`

	BackendPingPeriod      int `toml:"backend_ping_period" json:"backend_ping_period"`
	SessionMaxTimeout      int `toml:"session_max_timeout" json:"session_max_timeout"`
	SessionMaxBufSize      int `toml:"session_max_bufsize" json:"session_max_bufsize"`
	SessionMaxPipeline     int `toml:"session_max_pipeline" json:"session_max_pipeline"`
	SessionKeepAlivePeriod int `toml:"session_keepalive_period" json:"session_keepalive_period"`
}

func NewDefaultConfig() *Config {
	return &Config{
		ProtoType: "tcp4",
		ProxyAddr: "0.0.0.0:19000",
		AdminAddr: "0.0.0.0:11000",

		JodisAddr:    "",
		JodisTimeout: 10,

		ProductName: "Demo2",
		ProductAuth: "",

		BackendPingPeriod:  5,
		SessionMaxTimeout:  60 * 30,
		SessionMaxBufSize:  1024 * 128,
		SessionMaxPipeline: 1024,

		SessionKeepAlivePeriod: 60,
	}
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
