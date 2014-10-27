package main

import (
	"codis/pkg/utils"
	"os"
	"text/template"

	log "github.com/ngaut/logging"
)

var confTpl = `
zk={{.Zk}}
product={{.Product}}
ProxyId={{.ProxyId}}
`

var startProxyTpl = `
CODIS_CONF=./conf.ini {{.CodisBinPath}}/proxy --cpu {{.Cpu}} --logpath {{.LogPath}} --addr {{.ProxyAddr}} --httpAddr {{.HttpAddr}}
`

type Config struct {
	Zk      string
	Product string
	ProxyId string
}

type ProxyConf struct {
	Cpu          int
	ProxyAddr    string
	HttpAddr     string
	LogPath      string
	CodisBinPath string
}

func main() {
	conf, err := utils.InitConfig()
	if err != nil {
		log.Fatal(err)
	}

	var config Config
	var proxyConf ProxyConf

	config.Product, _ = conf.ReadString("product", "")
	config.Zk, _ = conf.ReadString("zk", "")
	config.ProxyId, _ = conf.ReadString("proxyId", "")

	proxyConf.Cpu, _ = conf.ReadInt("cpu", 2)
	proxyConf.ProxyAddr, _ = conf.ReadString("proxyAddr", "")
	proxyConf.HttpAddr, _ = conf.ReadString("httpAddr", "")
	proxyConf.LogPath, _ = conf.ReadString("LogPath", "")
	proxyConf.CodisBinPath, _ = conf.ReadString("codisBinPath", "")

	t := template.Must(template.New("confTpl").Parse(confTpl))
	f, err := os.OpenFile("conf.ini", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		log.Fatal(err)
	}
	err = t.Execute(f, config)
	if err != nil {
		log.Fatal(err)
	}

	t = template.Must(template.New("startProxyTpl").Parse(startProxyTpl))
	f, err = os.OpenFile("start_proxy.sh", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		log.Fatal(err)
	}
	err = t.Execute(f, proxyConf)
	if err != nil {
		log.Fatal(err)
	}
}
