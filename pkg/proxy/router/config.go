// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"strings"

	"github.com/wandoulabs/codis/pkg/proxy/router/topology"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Config struct {
	proxyId     string
	productName string
	zkAddr      string
	passwd      string
	fact        topology.ZkFactory
	proto       string //tcp or tcp4
	provider    string

	pingPeriod  int // seconds
	maxTimeout  int // seconds
	maxBufSize  int
	maxPipeline int
}

func LoadConf(configFile string) (*Config, error) {
	c, err := utils.InitConfigFromFile(configFile)
	if err != nil {
		log.PanicErrorf(err, "load config '%s' failed", configFile)
	}

	conf := &Config{}
	conf.productName, _ = c.ReadString("product", "test")
	if len(conf.productName) == 0 {
		log.Panicf("invalid config: product entry is missing in %s", configFile)
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

	loadConfInt := func(entry string, defInt int) int {
		v, _ := c.ReadInt(entry, defInt)
		if v < 0 {
			log.Panicf("invalid config: read %s = %d", entry, v)
		}
		return v
	}

	conf.pingPeriod = loadConfInt("backend_ping_period", 5)
	conf.maxTimeout = loadConfInt("session_max_timeout", 1800)
	conf.maxBufSize = loadConfInt("session_max_bufsize", 1024*32)
	conf.maxPipeline = loadConfInt("session_max_pipeline", 128)
	return conf, nil
}
