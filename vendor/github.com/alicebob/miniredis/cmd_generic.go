// Commands from http://redis.io/commands#generic

package miniredis

import (
	"math/rand"
	"strconv"
	"strings"

	"github.com/bsm/redeo"
)

// commandsGeneric handles EXPIRE, TTL, PERSIST, &c.
func commandsGeneric(m *Miniredis, srv *redeo.Server) {
	srv.HandleFunc("DEL", m.cmdDel)
	// DUMP
	srv.HandleFunc("EXISTS", m.cmdExists)
	srv.HandleFunc("EXPIRE", makeCmdExpire(m, "expire"))
	srv.HandleFunc("EXPIREAT", makeCmdExpire(m, "expireat"))
	srv.HandleFunc("KEYS", m.cmdKeys)
	// MIGRATE
	srv.HandleFunc("MOVE", m.cmdMove)
	// OBJECT
	srv.HandleFunc("PERSIST", m.cmdPersist)
	srv.HandleFunc("PEXPIRE", makeCmdExpire(m, "pexpire"))
	srv.HandleFunc("PEXPIREAT", makeCmdExpire(m, "pexpireat"))
	srv.HandleFunc("PTTL", m.cmdPTTL)
	srv.HandleFunc("RANDOMKEY", m.cmdRandomkey)
	srv.HandleFunc("RENAME", m.cmdRename)
	srv.HandleFunc("RENAMENX", m.cmdRenamenx)
	// RESTORE
	// SORT
	srv.HandleFunc("TTL", m.cmdTTL)
	srv.HandleFunc("TYPE", m.cmdType)
	srv.HandleFunc("SCAN", m.cmdScan)
}

// generic expire command for EXPIRE, PEXPIRE, EXPIREAT, PEXPIREAT
func makeCmdExpire(m *Miniredis, cmd string) func(*redeo.Responder, *redeo.Request) error {
	return func(out *redeo.Responder, r *redeo.Request) error {
		if len(r.Args) != 2 {
			setDirty(r.Client())
			return r.WrongNumberOfArgs()
		}
		if !m.handleAuth(r.Client(), out) {
			return nil
		}

		key := r.Args[0]
		value := r.Args[1]
		i, err := strconv.Atoi(value)
		if err != nil {
			setDirty(r.Client())
			out.WriteErrorString(msgInvalidInt)
			return nil
		}

		return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
			db := m.db(ctx.selectedDB)

			// Key must be present.
			if _, ok := db.keys[key]; !ok {
				out.WriteZero()
				return
			}
			db.expire[key] = i
			db.keyVersion[key]++
			out.WriteOne()
		})
	}
}

// TTL
func (m *Miniredis) cmdTTL(out *redeo.Responder, r *redeo.Request) error {
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

		if _, ok := db.keys[key]; !ok {
			// No such key
			out.WriteInt(-2)
			return
		}

		value, ok := db.expire[key]
		if !ok {
			// No expire value
			out.WriteInt(-1)
			return
		}
		out.WriteInt(value)
	})
}

// PTTL
func (m *Miniredis) cmdPTTL(out *redeo.Responder, r *redeo.Request) error {
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

		if _, ok := db.keys[key]; !ok {
			// No such key
			out.WriteInt(-2)
			return
		}

		value, ok := db.expire[key]
		if !ok {
			// No expire value
			out.WriteInt(-1)
			return
		}
		out.WriteInt(value)
	})
}

// PERSIST
func (m *Miniredis) cmdPersist(out *redeo.Responder, r *redeo.Request) error {
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

		if _, ok := db.keys[key]; !ok {
			// No such key
			out.WriteInt(0)
			return
		}

		_, ok := db.expire[key]
		if !ok {
			// No expire value
			out.WriteInt(0)
			return
		}
		delete(db.expire, key)
		db.keyVersion[key]++
		out.WriteInt(1)
	})
}

// DEL
func (m *Miniredis) cmdDel(out *redeo.Responder, r *redeo.Request) error {
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		count := 0
		for _, key := range r.Args {
			if db.exists(key) {
				count++
			}
			db.del(key, true) // delete expire
		}
		out.WriteInt(count)
	})
}

// TYPE
func (m *Miniredis) cmdType(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		out.WriteErrorString("usage error")
		return nil
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			out.WriteInlineString("none")
			return
		}

		out.WriteInlineString(t)
	})
}

// EXISTS
func (m *Miniredis) cmdExists(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		found := 0
		for _, k := range r.Args {
			if db.exists(k) {
				found++
			}
		}
		out.WriteInt(found)
	})
}

// MOVE
func (m *Miniredis) cmdMove(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	targetDB, err := strconv.Atoi(r.Args[1])
	if err != nil {
		targetDB = 0
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		if ctx.selectedDB == targetDB {
			out.WriteErrorString("ERR source and destination objects are the same")
			return
		}
		db := m.db(ctx.selectedDB)
		targetDB := m.db(targetDB)

		if !db.move(key, targetDB) {
			out.WriteZero()
			return
		}
		out.WriteOne()
	})
}

// KEYS
func (m *Miniredis) cmdKeys(out *redeo.Responder, r *redeo.Request) error {
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

		keys := matchKeys(db.allKeys(), key)
		out.WriteBulkLen(len(keys))
		for _, s := range keys {
			out.WriteString(s)
		}
	})
}

// RANDOMKEY
func (m *Miniredis) cmdRandomkey(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 0 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if len(db.keys) == 0 {
			out.WriteNil()
			return
		}
		nr := rand.Intn(len(db.keys))
		for k := range db.keys {
			if nr == 0 {
				out.WriteString(k)
				return
			}
			nr--
		}
	})
}

// RENAME
func (m *Miniredis) cmdRename(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	from := r.Args[0]
	to := r.Args[1]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(from) {
			out.WriteErrorString(msgKeyNotFound)
			return
		}

		db.rename(from, to)
		out.WriteOK()
	})
}

// RENAMENX
func (m *Miniredis) cmdRenamenx(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	from := r.Args[0]
	to := r.Args[1]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(from) {
			out.WriteErrorString(msgKeyNotFound)
			return
		}

		if db.exists(to) {
			out.WriteZero()
			return
		}

		db.rename(from, to)
		out.WriteOne()
	})
}

// SCAN
func (m *Miniredis) cmdScan(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	cursor, err := strconv.Atoi(r.Args[0])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidCursor)
		return nil
	}
	// MATCH and COUNT options
	var withMatch bool
	var match string
	args := r.Args[1:]
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

		keys := db.allKeys()
		if withMatch {
			keys = matchKeys(keys, match)
		}

		out.WriteBulkLen(2)
		out.WriteString("0") // no next cursor
		out.WriteBulkLen(len(keys))
		for _, k := range keys {
			out.WriteString(k)
		}
	})
}
