// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

type cmdProxy struct {
	addr string
	auth string
}

func (t *cmdProxy) Main(d map[string]interface{}) {
	t.addr = utils.ArgumentMust(d, "--proxy")
	t.auth, _ = d["--auth"].(string)

	switch {
	default:
		t.handleOverview(d)
	case d["--start"].(bool):
		t.handleStart(d)
	case d["--shutdown"].(bool):
		t.handleShutdown(d)
	case d["--log-level"] != nil:
		t.handleLogLevel(d)
	case d["--fillslots"] != nil:
		t.handleFillSlots(d)
	case d["--reset-stats"].(bool):
		t.handleResetStats(d)
	case d["--forcegc"].(bool):
		t.handleForceGC(d)
	}
}

func (t *cmdProxy) newProxyClient(xauth bool) *proxy.ApiClient {
	c := proxy.NewApiClient(t.addr)

	if !xauth {
		return c
	}

	log.Debugf("call rpc model to proxy %s", t.addr)
	p, err := c.Model()
	if err != nil {
		log.PanicErrorf(err, "call rpc model to proxy %s failed", t.addr)
	}
	log.Debugf("call rpc model OK")

	c.SetXAuth(p.ProductName, t.auth, p.Token)

	log.Debugf("call rpc xping to proxy %s", t.addr)
	if err := c.XPing(); err != nil {
		log.PanicErrorf(err, "call rpc xping failed")
	}
	log.Debugf("call rpc xping OK")

	return c
}

func (t *cmdProxy) handleOverview(d map[string]interface{}) {
	c := t.newProxyClient(false)

	log.Debugf("call rpc overview to proxy %s", t.addr)
	o, err := c.Overview()
	if err != nil {
		log.PanicErrorf(err, "call rpc overview to proxy %s failed", t.addr)
	}
	log.Debugf("call rpc overview OK")

	var cmd string
	for _, s := range []string{"config", "model", "slots", "stats"} {
		if d[s].(bool) {
			cmd = s
		}
	}

	var obj interface{}
	switch cmd {
	default:
		obj = o
	case "config":
		obj = o.Config
	case "model":
		obj = o.Model
	case "slots":
		obj = o.Slots
	case "stats":
		obj = o.Stats
	}

	b, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "json marshal failed")
	}
	fmt.Println(string(b))
}

func (t *cmdProxy) handleStart(d map[string]interface{}) {
	c := t.newProxyClient(true)

	log.Debugf("call rpc start to proxy %s", t.addr)
	if err := c.Start(); err != nil {
		log.PanicErrorf(err, "call rpc start to proxy %s failed", t.addr)
	}
	log.Debugf("call rpc start to proxy OK")
}

func (t *cmdProxy) handleLogLevel(d map[string]interface{}) {
	c := t.newProxyClient(true)

	s := utils.ArgumentMust(d, "--log-level")

	var v log.LogLevel
	if !v.ParseFromString(s) {
		log.Panicf("option --log-level = %s", s)
	}

	log.Debugf("call rpc loglevel to proxy %s", t.addr)
	if err := c.LogLevel(v); err != nil {
		log.PanicErrorf(err, "call rpc loglevel to proxy %s failed", t.addr)
	}
	log.Debugf("call rpc loglevel OK")
}

func (t *cmdProxy) handleFillSlots(d map[string]interface{}) {
	c := t.newProxyClient(true)

	b, err := ioutil.ReadFile(utils.ArgumentMust(d, "--fillslots"))
	if err != nil {
		log.PanicErrorf(err, "load slots from file failed")
	}

	var slots []*models.Slot
	if err := json.Unmarshal(b, &slots); err != nil {
		log.PanicErrorf(err, "decode slots from json failed")
	}

	for _, m := range slots {
		if m.Id < 0 || m.Id >= models.MaxSlotNum {
			log.Panicf("invalid slot id = %d", m.Id)
		}
	}

	if d["--locked"].(bool) {
		for _, m := range slots {
			m.Locked = true
		}
	}

	log.Debugf("call rpc fillslots to proxy %s", t.addr)
	if err := c.FillSlots(slots...); err != nil {
		log.PanicErrorf(err, "call rpc fillslots to proxy %s failed", t.addr)
	}
	log.Debugf("call rpc fillslots OK")
}

func (t *cmdProxy) handleResetStats(d map[string]interface{}) {
	c := t.newProxyClient(true)

	log.Debugf("call rpc resetstats to proxy %s", t.addr)
	if err := c.ResetStats(); err != nil {
		log.PanicErrorf(err, "call rpc resetstats to proxy %s failed", t.addr)
	}
	log.Debugf("call rpc resetstats OK")
}

func (t *cmdProxy) handleForceGC(d map[string]interface{}) {
	c := t.newProxyClient(true)

	log.Debugf("call rpc forcegc to proxy %s", t.addr)
	if err := c.ForceGC(); err != nil {
		log.PanicErrorf(err, "call rpc forcegc to proxy %s failed", t.addr)
	}
	log.Debugf("call rpc forcegc OK")
}

func (t *cmdProxy) handleShutdown(d map[string]interface{}) {
	c := t.newProxyClient(true)

	log.Debugf("call rpc shutdown to proxy %s", t.addr)
	if err := c.Shutdown(); err != nil {
		log.PanicErrorf(err, "call rpc shutdown to proxy %s failed", t.addr)
	}
	log.Debugf("call rpc shutdown OK")
}
