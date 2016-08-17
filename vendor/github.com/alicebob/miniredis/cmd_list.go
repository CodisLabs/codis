// Commands from http://redis.io/commands#list

package miniredis

import (
	"strconv"
	"strings"

	"github.com/bsm/redeo"
)

// commandsList handles list commands (mostly L*)
func commandsList(m *Miniredis, srv *redeo.Server) {
	// BLPOP key [key ...] timeout
	// BRPOP key [key ...] timeout
	// BRPOPLPUSH source destination timeout
	srv.HandleFunc("LINDEX", m.cmdLindex)
	srv.HandleFunc("LINSERT", m.cmdLinsert)
	srv.HandleFunc("LLEN", m.cmdLlen)
	srv.HandleFunc("LPOP", m.cmdLpop)
	srv.HandleFunc("LPUSH", m.cmdLpush)
	srv.HandleFunc("LPUSHX", m.cmdLpushx)
	srv.HandleFunc("LRANGE", m.cmdLrange)
	srv.HandleFunc("LREM", m.cmdLrem)
	srv.HandleFunc("LSET", m.cmdLset)
	srv.HandleFunc("LTRIM", m.cmdLtrim)
	srv.HandleFunc("RPOP", m.cmdRpop)
	srv.HandleFunc("RPOPLPUSH", m.cmdRpoplpush)
	srv.HandleFunc("RPUSH", m.cmdRpush)
	srv.HandleFunc("RPUSHX", m.cmdRpushx)
}

// LINDEX
func (m *Miniredis) cmdLindex(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	offset, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			// No such key
			out.WriteNil()
			return
		}
		if t != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		l := db.listKeys[key]
		if offset < 0 {
			offset = len(l) + offset
		}
		if offset < 0 || offset > len(l)-1 {
			out.WriteNil()
			return
		}
		out.WriteString(l[offset])
	})
}

// LINSERT
func (m *Miniredis) cmdLinsert(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 4 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	where := 0
	switch strings.ToLower(r.Args[1]) {
	case "before":
		where = -1
	case "after":
		where = +1
	default:
		setDirty(r.Client())
		out.WriteErrorString(msgSyntaxError)
		return nil
	}
	pivot := r.Args[2]
	value := r.Args[3]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			// No such key
			out.WriteZero()
			return
		}
		if t != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		l := db.listKeys[key]
		for i, el := range l {
			if el != pivot {
				continue
			}

			if where < 0 {
				l = append(l[:i], append(listKey{value}, l[i:]...)...)
			} else {
				if i == len(l)-1 {
					l = append(l, value)
				} else {
					l = append(l[:i+1], append(listKey{value}, l[i+1:]...)...)
				}
			}
			db.listKeys[key] = l
			db.keyVersion[key]++
			out.WriteInt(len(l))
			return
		}
		out.WriteInt(-1)
	})
}

// LLEN
func (m *Miniredis) cmdLlen(out *redeo.Responder, r *redeo.Request) error {
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
			// No such key. That's zero length.
			out.WriteZero()
			return
		}
		if t != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		out.WriteInt(len(db.listKeys[key]))
	})
}

// LPOP
func (m *Miniredis) cmdLpop(out *redeo.Responder, r *redeo.Request) error {
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
			// Non-existing key is fine.
			out.WriteNil()
			return
		}
		if db.t(key) != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		elem := db.listLpop(key)
		out.WriteString(elem)
	})
}

// LPUSH
func (m *Miniredis) cmdLpush(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	args := r.Args[1:]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if db.exists(key) && db.t(key) != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		var newLen int
		for _, value := range args {
			newLen = db.listLpush(key, value)
		}
		out.WriteInt(newLen)
	})
}

// LPUSHX
func (m *Miniredis) cmdLpushx(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	value := r.Args[1]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(key) {
			out.WriteZero()
			return
		}
		if db.t(key) != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		newLen := db.listLpush(key, value)
		out.WriteInt(newLen)
	})
}

// LRANGE
func (m *Miniredis) cmdLrange(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	start, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}
	end, err := strconv.Atoi(r.Args[2])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		l := db.listKeys[key]
		if len(l) == 0 {
			out.WriteBulkLen(0)
			return
		}

		rs, re := redisRange(len(l), start, end, false)
		out.WriteBulkLen(re - rs)
		for _, el := range l[rs:re] {
			out.WriteString(el)
		}
	})
}

// LREM
func (m *Miniredis) cmdLrem(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	count, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}
	value := r.Args[2]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(key) {
			out.WriteZero()
			return
		}
		if db.t(key) != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		l := db.listKeys[key]
		if count < 0 {
			reverseSlice(l)
		}
		deleted := 0
		newL := []string{}
		toDelete := len(l)
		if count < 0 {
			toDelete = -count
		}
		if count > 0 {
			toDelete = count
		}
		for _, el := range l {
			if el == value {
				if toDelete > 0 {
					deleted++
					toDelete--
					continue
				}
			}
			newL = append(newL, el)
		}
		if count < 0 {
			reverseSlice(newL)
		}
		db.listKeys[key] = newL
		db.keyVersion[key]++

		out.WriteInt(deleted)
	})
}

// LSET
func (m *Miniredis) cmdLset(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	index, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}
	value := r.Args[2]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(key) {
			out.WriteErrorString(msgKeyNotFound)
			return
		}
		if db.t(key) != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		l := db.listKeys[key]
		if index < 0 {
			index = len(l) + index
		}
		if index < 0 || index > len(l)-1 {
			out.WriteErrorString(msgOutOfRange)
			return
		}
		l[index] = value
		db.keyVersion[key]++

		out.WriteOK()
	})
}

// LTRIM
func (m *Miniredis) cmdLtrim(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	start, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}
	end, err := strconv.Atoi(r.Args[2])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		t, ok := db.keys[key]
		if !ok {
			out.WriteOK()
			return
		}
		if t != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		l := db.listKeys[key]
		rs, re := redisRange(len(l), start, end, false)
		db.listKeys[key] = l[rs:re]
		db.keyVersion[key]++
		out.WriteOK()
	})
}

// RPOP
func (m *Miniredis) cmdRpop(out *redeo.Responder, r *redeo.Request) error {
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
			out.WriteNil()
			return
		}
		if db.t(key) != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		elem := db.listPop(key)
		out.WriteString(elem)
	})
}

// RPOPLPUSH
func (m *Miniredis) cmdRpoplpush(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	src := r.Args[0]
	dst := r.Args[1]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(src) {
			out.WriteNil()
			return
		}
		if db.t(src) != "list" || (db.exists(dst) && db.t(dst) != "list") {
			out.WriteErrorString(msgWrongType)
			return
		}
		elem := db.listPop(src)
		db.listLpush(dst, elem)
		out.WriteString(elem)
	})
}

// RPUSH
func (m *Miniredis) cmdRpush(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	args := r.Args[1:]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if db.exists(key) && db.t(key) != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		var newLen int
		for _, value := range args {
			newLen = db.listPush(key, value)
		}
		out.WriteInt(newLen)
	})
}

// RPUSHX
func (m *Miniredis) cmdRpushx(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	value := r.Args[1]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(key) {
			out.WriteZero()
			return
		}
		if db.t(key) != "list" {
			out.WriteErrorString(msgWrongType)
			return
		}

		newLen := db.listPush(key, value)
		out.WriteInt(newLen)
	})
}
