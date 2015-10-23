// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"strings"

	"github.com/c4pt0r/cfg"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Config struct {
	proxyId       string
	productName   string
	zkAddr        string
	passwd        string
	fact          ZkFactory
	proto         string //tcp or tcp4
	provider      string
	dashboardAddr string

	pingPeriod       int // seconds
	maxTimeout       int // seconds
	maxBufSize       int
	maxPipeline      int
	zkSessionTimeout int
}

func LoadConf(configFile string) (*Config, error) {
	c := cfg.NewCfg(configFile)
	if err := c.Load(); err != nil {
		log.PanicErrorf(err, "load config '%s' failed", configFile)
	}

	conf := &Config{}
	conf.productName, _ = c.ReadString("product", "test")
	if len(conf.productName) == 0 {
		log.Panicf("invalid config: product entry is missing in %s", configFile)
	}
	conf.dashboardAddr, _ = c.ReadString("dashboard_addr", "")
	if conf.dashboardAddr == "" {
		log.Panicf("invalid config: dashboard_addr is missing in %s", configFile)
	}
	conf.zkAddr, _ = c.ReadString("zk", "")
	if len(conf.zkAddr) == 0 {
		log.Panicf("invalid config: need zk entry is missing in %s", configFile)
	}
	conf.zkAddr = strings.TrimSpace(conf.zkAddr)
	conf.passwd, _ = c.ReadString("password", "")

	conf.proxyId, _ = c.ReadString("proxy_id", "")
	if len(conf.proxyId) == 0 {
		log.Panicf("invalid config: need proxy_id entry is missing in %s", configFile)
	}

	conf.proto, _ = c.ReadString("proto", "tcp")
	conf.provider, _ = c.ReadString("coordinator", "zookeeper")

	loadConfInt := func(entry string, defval int) int {
		v, _ := c.ReadInt(entry, defval)
		if v < 0 {
			log.Panicf("invalid config: read %s = %d", entry, v)
		}
		return v
	}

	conf.pingPeriod = loadConfInt("backend_ping_period", 5)
	conf.maxTimeout = loadConfInt("session_max_timeout", 1800)
	conf.maxBufSize = loadConfInt("session_max_bufsize", 131072)
	conf.maxPipeline = loadConfInt("session_max_pipeline", 1024)
	conf.zkSessionTimeout = loadConfInt("zk_session_timeout", 30000)
	if conf.zkSessionTimeout <= 100 {
		conf.zkSessionTimeout *= 1000
		log.Warn("zkSessionTimeout is to small, it is ms not second")
	}
	return conf, nil
}
