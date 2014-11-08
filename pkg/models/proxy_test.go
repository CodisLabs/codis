// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"testing"

	"github.com/ngaut/zkhelper"
)

func TestProxy(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()

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
