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
	fact        topology.ZkFactory
	netTimeout  int    //seconds
	proto       string //tcp or tcp4
	provider    string
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

	conf.proxyId, _ = c.ReadString("proxy_id", "")
	if len(conf.proxyId) == 0 {
		log.Panicf("invalid config: need proxy_id entry is missing in %s", configFile)
	}

	conf.netTimeout, _ = c.ReadInt("net_timeout", 5)
	conf.proto, _ = c.ReadString("proto", "tcp")
	conf.provider, _ = c.ReadString("coordinator", "zookeeper")
	return conf, nil
}
