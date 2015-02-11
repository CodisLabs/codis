// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"bytes"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/docopt/docopt-go"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/binlog"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/service"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store/leveldb"
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/store/rocksdb"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
)

var args struct {
	config string
	create bool
	repair bool
}

func init() {
	log.SetLevel(log.LEVEL_INFO)
	log.SetTrace(log.LEVEL_WARN)
	log.SetFlags(log.Flags() | log.Lshortfile)
}

type Config struct {
	DBType string `toml:"dbtype"`
	DBPath string `toml:"dbpath"`

	Service *service.Config `toml:"service"`
	LevelDB *leveldb.Config `toml:"leveldb"`
	RocksDB *rocksdb.Config `toml:"rocksdb"`
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

func main() {
	usage := `
Usage:
	redis-binlog [--config=CONF] [--create|--repair] [--ncpu=N]

Options:
	-n N, --ncpu=N                    set runtime.GOMAXPROCS to N
	-c CONF, --config=CONF            specify the config file
	--create                          create if not exists
	--repair                          repair database
`
	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicErrorf(err, "parse arguments failed")
	}
	if s, ok := d["--ncpu"].(string); ok && len(s) != 0 {
		if n, err := strconv.ParseInt(s, 10, 64); err != nil {
			log.PanicErrorf(err, "parse --ncpu failed")
		} else if n <= 0 || n > 64 {
			log.Panicf("parse --ncpu = %d, only accept [1,64]", n)
		} else {
			runtime.GOMAXPROCS(int(n))
		}
	}

	args.config, _ = d["--config"].(string)
	args.create, _ = d["--create"].(bool)
	args.repair, _ = d["--repair"].(bool)

	conf := &Config{
		DBType:  "rocksdb",
		DBPath:  "testdb-rocksdb",
		LevelDB: leveldb.NewDefaultConfig(),
		RocksDB: rocksdb.NewDefaultConfig(),
		Service: service.NewDefaultConfig(),
	}

	if args.config != "" {
		if err := conf.LoadFromFile(args.config); err != nil {
			log.PanicErrorf(err, "load config failed")
		}
	}

	log.Infof("load config\n%s\n\n", conf)

	var db store.Database
	switch t := strings.ToLower(conf.DBType); t {
	default:
		log.Panicf("unknown db type = '%s'", conf.DBType)
	case "leveldb":
		db, err = leveldb.Open(conf.DBPath, conf.LevelDB, args.create, args.repair)
	case "rocksdb":
		db, err = rocksdb.Open(conf.DBPath, conf.RocksDB, args.create, args.repair)
	}

	if err != nil {
		log.PanicErrorf(err, "open database failed")
	}

	bl := binlog.New(db)
	defer bl.Close()

	if args.repair {
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		for _ = range c {
			log.Infof("interrupt and shutdown")
			bl.Close()
			os.Exit(0)
		}
	}()

	if err := service.Serve(conf.Service, bl); err != nil {
		log.ErrorErrorf(err, "service failed")
	}
}
