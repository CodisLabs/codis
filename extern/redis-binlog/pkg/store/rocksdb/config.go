// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rocksdb

import "github.com/wandoulabs/codis/extern/redis-port/pkg/libs/bytesize"

type Config struct {
	BlockSize       int `toml:"block_size"`
	CacheSize       int `toml:"cache_size"`
	WriteBufferSize int `toml:"write_buffer_size"`
	MaxOpenFiles    int `toml:"max_open_files"`
	NumLevels       int `toml:"num_levels"`

	BloomFilterSize               int `toml:"bloom_filter_size"`
	BackgroundThreads             int `toml:"background_threads"`
	HighPriorityBackgroundThreads int `toml:"high_priority_background_threads"`
	MaxBackgroundCompactions      int `toml:"max_background_compactions"`
	MaxBackgroundFlushes          int `toml:"max_background_flushes"`

	MaxWriteBufferNumber           int `toml:"max_write_buffer_number"`
	MinWriteBufferNumberToMerge    int `toml:"min_write_buffer_number_to_merge"`
	Level0FileNumCompactionTrigger int `toml:"level0_filenum_compaction_trigger"`
	Level0SlowdownWritesTrigger    int `toml:"level0_slowdown_writes_trigger"`
	Level0StopWritesTrigger        int `toml:"level0_stop_writes_trigger"`
	TargetFileSizeBase             int `toml:"target_file_size_base"`
	TargetFileSizeMultiplier       int `toml:"target_file_size_multiplier"`
	MaxBytesForLevelBase           int `toml:"max_bytes_for_level_base"`
	MaxBytesForLevelMultiplier     int `toml:"max_bytes_for_level_multiplier"`

	DisableAutoCompactions bool `toml:"disable_auto_compations"`
	DisableDataSync        bool `toml:"disable_data_sync"`
	UseFsync               bool `toml:"use_fsync"`
	SnapshotFillCache      bool `toml:"snapshot_fillcache"`
	AllowOSBuffer          bool `toml:"allow_os_buffer"`
}

func NewDefaultConfig() *Config {
	return &Config{
		BlockSize:       bytesize.KB * 64,
		CacheSize:       bytesize.GB * 4,
		WriteBufferSize: bytesize.MB * 64,
		MaxOpenFiles:    4096,
		NumLevels:       5,

		BloomFilterSize:               24,
		BackgroundThreads:             8,
		HighPriorityBackgroundThreads: 2,
		MaxBackgroundCompactions:      6,
		MaxBackgroundFlushes:          2,

		MaxWriteBufferNumber:           4,
		MinWriteBufferNumberToMerge:    1,
		Level0FileNumCompactionTrigger: 8,
		Level0SlowdownWritesTrigger:    16,
		Level0StopWritesTrigger:        64,
		TargetFileSizeBase:             bytesize.MB * 64,
		TargetFileSizeMultiplier:       2,
		MaxBytesForLevelBase:           bytesize.MB * 512,
		MaxBytesForLevelMultiplier:     8,

		DisableAutoCompactions: false,
		DisableDataSync:        false,
		UseFsync:               false,
		SnapshotFillCache:      true,
		AllowOSBuffer:          true,
	}
}
