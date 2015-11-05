// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"bytes"

	"github.com/BurntSushi/toml"

	"github.com/wandoulabs/codis/pkg/utils/errors"
)

type Config struct {
	AdminAddr string `toml:"admin_addr" json:"admin_addr"`

	ProductName string `toml:"product_name" json:"product_name"`
	ProductAuth string `toml:"product_auth" json:"-"`
}

func NewDefaultConfig() *Config {
	return &Config{
		AdminAddr: "0.0.0.0:18080",

		ProductName: "Demo2",
		ProductAuth: "",
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
