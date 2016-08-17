// Commands from http://redis.io/commands#hash

package miniredis

import (
	"strconv"
	"strings"

	"github.com/bsm/redeo"
)

// commandsHash handles all hash value operations.
func commandsHash(m *Miniredis, srv *redeo.Server) {
	srv.HandleFunc("HDEL", m.cmdHdel)
	srv.HandleFunc("HEXISTS", m.cmdHexists)
	srv.HandleFunc("HGET", m.cmdHget)
	srv.HandleFunc("HGETALL", m.cmdHgetall)
	srv.HandleFunc("HINCRBY", m.cmdHincrby)
	srv.HandleFunc("HINCRBYFLOAT", m.cmdHincrbyfloat)
	srv.HandleFunc("HKEYS", m.cmdHkeys)
	srv.HandleFunc("HLEN", m.cmdHlen)
	srv.HandleFunc("HMGET", m.cmdHmget)
	srv.HandleFunc("HMSET", m.cmdHmset)
	srv.HandleFunc("HSET", m.cmdHset)
	srv.HandleFunc("HSETNX", m.cmdHsetnx)
	srv.HandleFunc("HVALS", m.cmdHvals)
	srv.HandleFunc("HSCAN", m.cmdHscan)
}

// HSET
func (m *Miniredis) cmdHset(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	var (
		key   = r.Args[0]
		field = r.Args[1]
		value = r.Args[2]
	)

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		if db.hashSet(key, field, value) {
			out.WriteZero()
		} else {
			out.WriteOne()
		}
	})
}

// HSETNX
func (m *Miniredis) cmdHsetnx(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	var (
		key   = r.Args[0]
		field = r.Args[1]
		value = r.Args[2]
	)

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		if _, ok := db.hashKeys[key]; !ok {
			db.hashKeys[key] = map[string]string{}
			db.keys[key] = "hash"
		}
		_, ok := db.hashKeys[key][field]
		if ok {
			out.WriteZero()
			return
		}
		db.hashKeys[key][field] = value
		db.keyVersion[key]++
		out.WriteOne()
	})
}

// HMSET
func (m *Miniredis) cmdHmset(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	args := r.Args[1:]
	if len(args)%2 != 0 {
		setDirty(r.Client())
		// non-default error message
		out.WriteErrorString("ERR wrong number of arguments for HMSET")
		return nil
	}
	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		for len(args) > 0 {
			field := args[0]
			value := args[1]
			args = args[2:]

			db.hashSet(key, field, value)
		}
		out.WriteOK()
	})
}

// HGET
func (m *Miniredis) cmdHget(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	field := r.Args[1]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			out.WriteNil()
			return
		}
		if t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}
		value, ok := db.hashKeys[key][field]
		if !ok {
			out.WriteNil()
			return
		}
		out.WriteString(value)
	})
}

// HDEL
func (m *Miniredis) cmdHdel(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	fields := r.Args[1:]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			// No key is zero deleted
			out.WriteInt(0)
			return
		}
		if t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		deleted := 0
		for _, f := range fields {
			_, ok := db.hashKeys[key][f]
			if !ok {
				continue
			}
			delete(db.hashKeys[key], f)
			deleted++
		}
		out.WriteInt(deleted)

		// Nothing left. Remove the whole key.
		if len(db.hashKeys[key]) == 0 {
			db.del(key, true)
		}
	})
}

// HEXISTS
func (m *Miniredis) cmdHexists(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	field := r.Args[1]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			out.WriteInt(0)
			return
		}
		if t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		if _, ok := db.hashKeys[key][field]; !ok {
			out.WriteInt(0)
			return
		}
		out.WriteInt(1)
	})
}

// HGETALL
func (m *Miniredis) cmdHgetall(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			out.WriteBulkLen(0)
			return
		}
		if t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		out.WriteBulkLen(len(db.hashKeys[key]) * 2)
		for _, k := range db.hashFields(key) {
			out.WriteString(k)
			out.WriteString(db.hashGet(key, k))
		}
	})
}

// HKEYS
func (m *Miniredis) cmdHkeys(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(key) {
			out.WriteBulkLen(0)
			return
		}
		if db.t(key) != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		fields := db.hashFields(key)
		out.WriteBulkLen(len(fields))
		for _, f := range fields {
			out.WriteString(f)
		}
	})
}

// HVALS
func (m *Miniredis) cmdHvals(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			out.WriteBulkLen(0)
			return
		}
		if t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		out.WriteBulkLen(len(db.hashKeys[key]))
		for _, v := range db.hashKeys[key] {
			out.WriteString(v)
		}
	})
}

// HLEN
func (m *Miniredis) cmdHlen(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			out.WriteInt(0)
			return
		}
		if t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		out.WriteInt(len(db.hashKeys[key]))
	})
}

// HMGET
func (m *Miniredis) cmdHmget(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		f, ok := db.hashKeys[key]
		if !ok {
			f = map[string]string{}
		}

		out.WriteBulkLen(len(r.Args) - 1)
		for _, k := range r.Args[1:] {
			v, ok := f[k]
			if !ok {
				out.WriteNil()
				continue
			}
			out.WriteString(v)
		}
	})
}

// HINCRBY
func (m *Miniredis) cmdHincrby(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	var (
		key        = r.Args[0]
		field      = r.Args[1]
		delta, err = strconv.Atoi(r.Args[2])
	)
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		v, err := db.hashIncr(key, field, delta)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}
		out.WriteInt(v)
	})
}

// HINCRBYFLOAT
func (m *Miniredis) cmdHincrbyfloat(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	var (
		key        = r.Args[0]
		field      = r.Args[1]
		delta, err = strconv.ParseFloat(r.Args[2], 64)
	)
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidFloat)
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "hash" {
			out.WriteErrorString(msgWrongType)
			return
		}

		v, err := db.hashIncrfloat(key, field, delta)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}
		out.WriteString(formatFloat(v))
	})
}

// HSCAN
func (m *Miniredis) cmdHscan(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	cursor, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidCursor)
		return nil
	}
	// MATCH and COUNT options
	var withMatch bool
	var match string
	args := r.Args[2:]
	for len(args) > 0 {
		if strings.ToLower(args[0]) == "count" {
			if len(args) < 2 {
				setDirty(r.Client())
				out.WriteErrorString(msgSyntaxError)
				return nil
			}
			_, err := strconv.Atoi(args[1])
			if err != nil {
				setDirty(r.Client())
				out.WriteErrorString(msgInvalidInt)
				return nil
			}
			// We do nothing with count.
			args = args[2:]
			continue
		}
		if strings.ToLower(args[0]) == "match" {
			if len(args) < 2 {
				setDirty(r.Client())
				out.WriteErrorString(msgSyntaxError)
				return nil
			}
			withMatch = true
			match = args[1]
			args = args[2:]
			continue
		}
		setDirty(r.Client())
		out.WriteErrorString(msgSyntaxError)
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)
		// We return _all_ (matched) keys every time.

		if cursor != 0 {
			// Invalid cursor.
			out.WriteBulkLen(2)
			out.WriteString("0") // no next cursor
			out.WriteBulkLen(0)  // no elements
			return
		}
		if db.exists(key) && db.t(key) != "hash" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		members := db.hashFields(key)
		if withMatch {
			members = matchKeys(members, match)
		}

		out.WriteBulkLen(2)
		out.WriteString("0") // no next cursor
		// HSCAN gives key, values.
		out.WriteBulkLen(len(members) * 2)
		for _, k := range members {
			out.WriteString(k)
			out.WriteString(db.hashGet(key, k))
		}
	})
}
