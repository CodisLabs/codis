package utils

import (
	"codis/pkg/zkhelper"
	"fmt"
	"os"

	"github.com/c4pt0r/cfg"
)

func InitConfig() (*cfg.Cfg, error) {
	configFile := os.Getenv("CODIS_CONF")
	if len(configFile) == 0 {
		configFile = "config.ini"
	}
	ret := cfg.NewCfg(configFile)
	if err := ret.Load(); err != nil {
		return nil, err
	}
	return ret, nil
}

func InitConfigFromFile(filename string) (*cfg.Cfg, error) {
	ret := cfg.NewCfg(filename)
	if err := ret.Load(); err != nil {
		return nil, err
	}
	return ret, nil
}

func GetZkLock(zkConn zkhelper.Conn, productName string) zkhelper.ZLocker {
	zkPath := fmt.Sprintf("/zk/codis/db_%s/LOCK", productName)
	ret := zkhelper.CreateMutex(zkConn, zkPath)
	return ret
}

type Strings []string

func (s1 Strings) Eq(s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}
