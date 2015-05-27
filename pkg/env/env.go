package env

import (
	"os"
	"strings"

	"github.com/c4pt0r/cfg"
	"github.com/ngaut/zkhelper"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Env interface {
	ProductName() string
	DashboardAddr() string
	NewZkConn() (zkhelper.Conn, error)
}

type CodisEnv struct {
	zkAddr        string
	dashboardAddr string
	productName   string
	provider      string
}

func LoadCodisEnv(cfg *cfg.Cfg) Env {
	if cfg == nil {
		log.Panicf("config is nil")
	}

	productName, err := cfg.ReadString("product", "test")
	if err != nil {
		log.PanicErrorf(err, "read product name failed")
	}

	zkAddr, err := cfg.ReadString("zk", "localhost:2181")
	if err != nil {
		log.PanicErrorf(err, "read zk address failed")
	}

	hostname, _ := os.Hostname()
	dashboardAddr, err := cfg.ReadString("dashboard_addr", hostname+":18087")
	if err != nil {
		log.PanicErrorf(err, "read dashboard address failed")
	}

	provider, err := cfg.ReadString("coordinator", "zookeeper")
	if err != nil {
		log.PanicErrorf(err, "read coordinator failed")
	}

	return &CodisEnv{
		zkAddr:        zkAddr,
		dashboardAddr: dashboardAddr,
		productName:   productName,
		provider:      provider,
	}
}

func (e *CodisEnv) ProductName() string {
	return e.productName
}

func (e *CodisEnv) DashboardAddr() string {
	return e.dashboardAddr
}

func (e *CodisEnv) NewZkConn() (zkhelper.Conn, error) {
	switch e.provider {
	case "zookeeper":
		return zkhelper.ConnectToZk(e.zkAddr)
	case "etcd":
		addr := strings.TrimSpace(e.zkAddr)
		if !strings.HasPrefix(addr, "http://") {
			addr = "http://" + addr
		}
		return zkhelper.NewEtcdConn(addr)
	}
	return nil, errors.Errorf("need coordinator in config file, %+v", e)
}
