// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"bytes"

	"github.com/BurntSushi/toml"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/bytesize"
)

type Config struct {
	Listen      string `toml:"listen_address"`
	DumpPath    string `toml:"dump_filepath"`
	ConnTimeout int    `toml:"conn_timeout"`

	SyncFilePath string `toml:"sync_file_path"`
	SyncFileSize int    `toml:"sync_file_size"`
	SyncBuffSize int    `toml:"sync_memory_buffer"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Listen:      "0.0.0.0:6380",
		DumpPath:    "dump.rdb",
		ConnTimeout: 900,

		SyncFilePath: "sync.pipe",
		SyncFileSize: bytesize.GB * 32,
		SyncBuffSize: bytesize.MB * 32,
	}
}

func (c *Config) String() string {
	var b bytes.Buffer
	e := toml.NewEncoder(&b)
	e.Indent = "    "
	e.Encode(c)
	return b.String()
}
