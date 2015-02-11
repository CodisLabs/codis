// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"github.com/wandoulabs/codis/extern/redis-binlog/pkg/binlog"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/redis"
)

// SLOTSRESTORE key ttlms value [key ttlms value ...]
func (h *Handler) SlotsRestore(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) == 0 || len(args)%3 != 0 {
		return toRespErrorf("len(args) = %d, expect != 0 && mod 3 == 0", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	if err := s.Binlog().SlotsRestore(s.DB(), iconvert(args)...); err != nil {
		return toRespError(err)
	} else {
		return redis.NewString("OK"), nil
	}
}

// SLOTSMGRTSLOT host port timeout slot
func (h *Handler) SlotsMgrtSlot(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 4 {
		return toRespErrorf("len(args) = %d, expect = 4", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	if n, err := s.Binlog().SlotsMgrtSlot(s.DB(), iconvert(args)...); err != nil {
		return toRespError(err)
	} else {
		resp := redis.NewArray()
		resp.AppendInt(n)
		if n != 0 {
			resp.AppendInt(1)
		} else {
			resp.AppendInt(0)
		}
		return resp, nil
	}
}

// SLOTSMGRTTAGSLOT host port timeout slot
func (h *Handler) SlotsMgrtTagSlot(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 4 {
		return toRespErrorf("len(args) = %d, expect = 4", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	if n, err := s.Binlog().SlotsMgrtTagSlot(s.DB(), iconvert(args)...); err != nil {
		return toRespError(err)
	} else {
		resp := redis.NewArray()
		resp.AppendInt(n)
		if n != 0 {
			resp.AppendInt(1)
		} else {
			resp.AppendInt(0)
		}
		return resp, nil
	}
}

// SLOTSMGRTONE host port timeout key
func (h *Handler) SlotsMgrtOne(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 4 {
		return toRespErrorf("len(args) = %d, expect = 4", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	if n, err := s.Binlog().SlotsMgrtOne(s.DB(), iconvert(args)...); err != nil {
		return toRespError(err)
	} else {
		return redis.NewInt(n), nil
	}
}

// SLOTSMGRTTAGONE host port timeout key
func (h *Handler) SlotsMgrtTagOne(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) != 4 {
		return toRespErrorf("len(args) = %d, expect = 4", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	if n, err := s.Binlog().SlotsMgrtTagOne(s.DB(), iconvert(args)...); err != nil {
		return toRespError(err)
	} else {
		return redis.NewInt(n), nil
	}
}

// SLOTSINFO [start [count]]
func (h *Handler) SlotsInfo(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) > 2 {
		return toRespErrorf("len(args) = %d, expect <= 2", len(args))
	}

	s, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	if m, err := s.Binlog().SlotsInfo(s.DB(), iconvert(args)...); err != nil {
		return toRespError(err)
	} else {
		resp := redis.NewArray()
		for i := uint32(0); i < binlog.MaxSlotNum; i++ {
			v, ok := m[i]
			if ok {
				s := redis.NewArray()
				s.AppendInt(int64(i))
				s.AppendInt(v)
				resp.Append(s)
			}
		}
		return resp, nil
	}
}

// SLOTSHASHKEY key [key...]
func (h *Handler) SlotsHashKey(arg0 interface{}, args [][]byte) (redis.Resp, error) {
	if len(args) == 0 {
		return toRespErrorf("len(args) = %d, expect != 1", len(args))
	}

	_, err := session(arg0, args)
	if err != nil {
		return toRespError(err)
	}

	resp := redis.NewArray()
	for _, key := range args {
		_, slot := binlog.HashKeyToSlot(key)
		resp.AppendInt(int64(slot))
	}
	return resp, nil
}
