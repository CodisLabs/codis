package models

import (
	"testing"

	"github.com/wandoulabs/codis/pkg/zkhelper"
)

func TestProxy(t *testing.T) {
	fakeZkConn := zkhelper.NewFakeConn()

	pi := &ProxyInfo{
		Id:    "proxy_1",
		Addr:  "localhost:1234",
		State: PROXY_STATE_OFFLINE,
	}

	_, err := CreateProxyInfo(fakeZkConn, productName, pi)
	if err != nil {
		t.Error(err)
	}

	ps, err := ProxyList(fakeZkConn, productName, nil)
	if err != nil {
		t.Error(err)
	}

	if len(ps) != 1 || ps[0].Id != "proxy_1" {
		t.Error("create proxy error")
	}

	err = SetProxyStatus(fakeZkConn, productName, pi.Id, PROXY_STATE_ONLINE)
	if err != nil {
		t.Error(err)
	}

	p, err := GetProxyInfo(fakeZkConn, productName, pi.Id)
	if err != nil {
		t.Error(err)
	}

	if p.State != PROXY_STATE_ONLINE {
		t.Error("change status error")
	}
}
