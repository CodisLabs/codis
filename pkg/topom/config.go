// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"bytes"

	"github.com/BurntSushi/toml"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

const DefaultConfig = `
##################################################
#                                                #
#                  Codis-Dashboard               #
#                                                #
##################################################

# Set Coordinator, only accept "zookeeper" & "etcd"
coordinator_name = "zookeeper"
coordinator_addr = "127.0.0.1:2181"

# Set Codis Product Name/Auth.
product_name = "codis-demo"
product_auth = ""

# Set bind address for admin(rpc), tcp only.
admin_addr = "0.0.0.0:18080"
`

type Config struct {
	CoordinatorName string `toml:"coordinator_name" json:"coordinator_name"`
	CoordinatorAddr string `toml:"coordinator_addr" json:"coordinator_addr"`

	AdminAddr string `toml:"admin_addr" json:"admin_addr"`

	HostAdmin string `toml:"-" json:"-"`

	ProductName string `toml:"product_name" json:"product_name"`
	ProductAuth string `toml:"product_auth" json:"-"`
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
