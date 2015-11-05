// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy_test

import (
	"testing"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/assert"
)

var config = newProxyConfig()

func newProxyConfig() *proxy.Config {
	config := proxy.NewDefaultConfig()
	config.ProxyAddr = "0.0.0.0:0"
	config.AdminAddr = "0.0.0.0:0"
	return config
}

func openProxy() (*proxy.Proxy, string) {
	s, err := proxy.New(config)
	assert.MustNoError(err)
	return s, s.GetModel().AdminAddr
}

func TestModel(x *testing.T) {
	s, addr := openProxy()
	defer s.Close()

	var c = proxy.NewApiClient(addr)

	p, err := c.Model()
	assert.MustNoError(err)
	assert.Must(p.Token == s.GetToken())
	assert.Must(p.ProductName == config.ProductName)
}

func TestStats(x *testing.T) {
	s, addr := openProxy()
	defer s.Close()

	var c = proxy.NewApiClient(addr)

	c.SetXAuth(config.ProductName, config.ProductAuth, "")
	_, err1 := c.Stats()
	assert.Must(err1 != nil)

	c.SetXAuth(config.ProductName, config.ProductAuth, s.GetToken())
	_, err2 := c.Stats()
	assert.MustNoError(err2)
}

func verifySlots(c *proxy.ApiClient, expect map[int]*models.Slot) {
	slots, err := c.Slots()
	assert.MustNoError(err)

	assert.Must(len(slots) == models.MaxSlotNum)

	for i, slot := range expect {
		if slot != nil {
			assert.Must(slots[i].Id == i)
			assert.Must(slot.Locked == slots[i].Locked)
			assert.Must(slot.BackendAddr == slots[i].BackendAddr)
			assert.Must(slot.MigrateFrom == slots[i].MigrateFrom)
		}
	}
}

func TestFillSlot(x *testing.T) {
	s, addr := openProxy()
	defer s.Close()

	var c = proxy.NewApiClient(addr)
	c.SetXAuth(config.ProductName, config.ProductAuth, s.GetToken())

	expect := make(map[int]*models.Slot)

	for i := 0; i < 16; i++ {
		slot := &models.Slot{
			Id:          i,
			Locked:      i%2 == 0,
			BackendAddr: "x.x.x.x:xxxx",
		}
		assert.MustNoError(c.FillSlots(slot))
		expect[i] = slot
	}
	verifySlots(c, expect)

	slots := []*models.Slot{}
	for i := 0; i < 16; i++ {
		slot := &models.Slot{
			Id:          i,
			Locked:      i%2 != 0,
			BackendAddr: "y.y.y.y:yyyy",
			MigrateFrom: "x.x.x.x:xxxx",
		}
		slots = append(slots, slot)
		expect[i] = slot
	}
	assert.MustNoError(c.FillSlots(slots...))
	verifySlots(c, expect)
}

func TestStartAndShutdown(x *testing.T) {
	s, addr := openProxy()
	defer s.Close()

	var c = proxy.NewApiClient(addr)
	c.SetXAuth(config.ProductName, config.ProductAuth, s.GetToken())

	expect := make(map[int]*models.Slot)

	for i := 0; i < 16; i++ {
		slot := &models.Slot{
			Id:          i,
			BackendAddr: "x.x.x.x:xxxx",
		}
		assert.MustNoError(c.FillSlots(slot))
		expect[i] = slot
	}
	verifySlots(c, expect)

	err1 := c.Start()
	assert.MustNoError(err1)

	err2 := c.Shutdown()
	assert.MustNoError(err2)

	err3 := c.Start()
	assert.Must(err3 != nil)
}
