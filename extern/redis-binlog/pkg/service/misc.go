// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

// PING
func (h *Handler) Ping(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 0 {
		return toRespErrorf("len(args) = %d, expect = 0", len(args))
	}

	_, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}
	return redis.NewString("PONG"), nil
}

// ECHO text
func (h *Handler) Echo(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 1 {
		return toRespErrorf("len(args) = %d, expect = 1", len(args))
	}

	_, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}
	return redis.NewString(string(args[0])), nil
}

// FLUSHALL
func (h *Handler) FlushAll(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 0 {
		return toRespErrorf("len(args) = %d, expect = 0", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	if err := s.Binlog().Reset(); err != nil {
		return toRespError(err)
	} else {
		return redis.NewString("OK"), nil
	}
}

// COMPACTALL
func (h *Handler) CompactAll(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 0 {
		return toRespErrorf("len(args) = %d, expect = 0", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	if err := s.Binlog().CompactAll(); err != nil {
		return toRespError(err)
	} else {
		return redis.NewString("OK"), nil
	}
}

// SHUTDOWN
func (h *Handler) Shutdown(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 0 {
		return toRespErrorf("len(args) = %d, expect = 0", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	s.Binlog().Close()
	os.Exit(0)
	return nil, nil
}

// INFO
func (h *Handler) Info(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 0 {
		return toRespErrorf("len(args) = %d, expect = 0", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	var b bytes.Buffer
	if v, err := s.Binlog().Info(); err != nil {
		return toRespError(err)
	} else {
		fmt.Fprintf(&b, "# Database\n")
		fmt.Fprintf(&b, "%s\n", v)
		fmt.Fprintf(&b, "\n")

		fmt.Fprintf(&b, "# Config\n")
		fmt.Fprintf(&b, "%s\n", h.config)
		fmt.Fprintf(&b, "\n")

		fmt.Fprintf(&b, "# Clients\n")
		fmt.Fprintf(&b, "bgsave:%d\n", h.counters.bgsave.Get())
		fmt.Fprintf(&b, "clients:%d\n", h.counters.clients.Get())
		fmt.Fprintf(&b, "clients_accepted:%d\n", h.counters.clientsAccepted.Get())
		fmt.Fprintf(&b, "commands:%d\n", h.counters.commands.Get())
		fmt.Fprintf(&b, "commands_failed:%d\n", h.counters.commandsFailed.Get())
		fmt.Fprintf(&b, "sync_rdb_remains:%d\n", h.counters.syncRdbRemains.Get())
		fmt.Fprintf(&b, "sync_cache_bytes:%d\n", h.counters.syncCacheBytes.Get())
		fmt.Fprintf(&b, "sync_total_bytes:%d\n", h.counters.syncTotalBytes.Get())
		fmt.Fprintf(&b, "slaveof:%s\n", h.syncto)
		fmt.Fprintf(&b, "slaveof_since:%d\n", h.syncto_since)
		fmt.Fprintf(&b, "\n")
		return redis.NewString(b.String()), nil
	}
}

// CONFIG get key / set key value
func (h *Handler) Config(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 2 && len(args) != 3 {
		return toRespErrorf("len(args) = %d, expect = 2 or 3", len(args))
	}

	_, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	sub, args := strings.ToLower(string(args[0])), args[1:]

	switch sub {
	default:
		return toRespErrorf("unknown sub-command = %s", sub)
	case "get":
		if len(args) != 2 {
			return toRespErrorf("len(args) = %d, expect = 2", len(args))
		}
		switch e := strings.ToLower(string(args[1])); e {
		default:
			return toRespErrorf("unknown entry %s", e)
		case "maxmemory":
			return redis.NewString("0"), nil
		}
	}
}
