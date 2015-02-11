// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package leveldb

import "github.com/wandoulabs/codis/extern/redis-port/pkg/libs/bytesize"

type Config struct {
	BlockSize       int `toml:"block_size"`
	CacheSize       int `toml:"cache_size"`
	WriteBufferSize int `toml:"write_buffer_size"`
	BloomFilterSize int `toml:"bloom_filter_size"`
	MaxOpenFiles    int `toml:"max_open_files"`
}

func NewDefaultConfig() *Config {
	return &Config{
		BlockSize:       bytesize.KB * 64,
		CacheSize:       bytesize.GB * 4,
		WriteBufferSize: bytesize.MB * 64,
		BloomFilterSize: 24,
		MaxOpenFiles:    4096,
	}
}
