// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"testing"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy"
	"github.com/CodisLabs/codis/pkg/utils/assert"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

var config = NewDefaultConfig()

func init() {
	log.SetLevel(log.LevelError)
}

func init() {
	config.AdminAddr = "0.0.0.0:0"
	config.ProductName = "topom_test"
	config.ProductAuth = "topom_auth"
}

func openTopom() *Topom {
	t, err := New(newMemClient(nil), config)
	assert.MustNoError(err)
	return t
}

func openProxy() (*models.Proxy, *proxy.ApiClient) {
	config := proxy.NewDefaultConfig()
	config.AdminAddr = "0.0.0.0:0"
	config.ProxyAddr = "0.0.0.0:0"
	config.ProductName = "topom_test"
	config.ProductAuth = "topom_auth"

	s, err := proxy.New(config)
	assert.MustNoError(err)

	c := proxy.NewApiClient(s.Model().AdminAddr)
	c.SetXAuth(config.ProductName, config.ProductAuth, s.Token())

	p, err := c.Model()
	assert.MustNoError(err)
	return p, c
}

func TestTopomClose(x *testing.T) {
	t := openTopom()
	defer t.Close()

	assert.Must(t.IsClosed() == false)
	assert.MustNoError(t.Close())
	assert.Must(t.IsClosed() == true)
}

func TestTopomExclusive(x *testing.T) {
	store := newMemStore()

	t, err := New(newMemClient(store), config)
	assert.MustNoError(err)

	defer t.Close()

	_, err = New(newMemClient(store), config)
	assert.Must(err != nil)

	t.Close()

	_, err = New(newMemClient(store), config)
	assert.MustNoError(err)
}
