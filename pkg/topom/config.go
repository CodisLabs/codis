// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"bytes"

	"github.com/BurntSushi/toml"

	"github.com/wandoulabs/codis/pkg/models/zk"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

type Config struct {
	Coordinator struct {
		Name string `toml:"name" json:"name"`
		Addr string `toml:"addr" json:"addr"`
	} `toml:"coordinator" json:"coordinator"`

	AdminAddr string `toml:"admin_addr" json:"admin_addr"`

	HostAdmin string `toml:"-" json:"-"`

	ProductName string `toml:"product_name" json:"product_name"`
	ProductAuth string `toml:"product_auth" json:"-"`
}

func NewDefaultConfig() *Config {
	c := &Config{
		AdminAddr: "0.0.0.0:18080",

		ProductName: "Demo3",
		ProductAuth: "",
	}
	c.Coordinator.Name = zkclient.CoordinatorName
	c.Coordinator.Addr = "127.0.0.1:2181"
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
