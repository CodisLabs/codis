package router

import (
	"testing"

	"github.com/ngaut/zkhelper"
)

var s = NewServer(":1900", ":11000",
	&Conf{
		proxyId:     "proxy_test",
		productName: "test",
		zkAddr:      "localhost:2181",
		f:           func(string) (zkhelper.Conn, error) { return zkhelper.NewConn(), nil },
	},
)

func TestWaitOnline(t *testing.T) {
}
