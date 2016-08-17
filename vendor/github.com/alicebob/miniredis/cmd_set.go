// Commands from http://redis.io/commands#set

package miniredis

import (
	"math/rand"
	"strconv"
	"strings"

	"github.com/bsm/redeo"
)

// commandsSet handles all set value operations.
func commandsSet(m *Miniredis, srv *redeo.Server) {
	srv.HandleFunc("SADD", m.cmdSadd)
	srv.HandleFunc("SCARD", m.cmdScard)
	srv.HandleFunc("SDIFF", m.cmdSdiff)
	srv.HandleFunc("SDIFFSTORE", m.cmdSdiffstore)
	srv.HandleFunc("SINTER", m.cmdSinter)
	srv.HandleFunc("SINTERSTORE", m.cmdSinterstore)
	srv.HandleFunc("SISMEMBER", m.cmdSismember)
	srv.HandleFunc("SMEMBERS", m.cmdSmembers)
	srv.HandleFunc("SMOVE", m.cmdSmove)
	srv.HandleFunc("SPOP", m.cmdSpop)
	srv.HandleFunc("SRANDMEMBER", m.cmdSrandmember)
	srv.HandleFunc("SREM", m.cmdSrem)
	srv.HandleFunc("SUNION", m.cmdSunion)
	srv.HandleFunc("SUNIONSTORE", m.cmdSunionstore)
	srv.HandleFunc("SSCAN", m.cmdSscan)
}

// SADD
func (m *Miniredis) cmdSadd(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	elems := r.Args[1:]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if db.exists(key) && db.t(key) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		added := db.setAdd(key, elems...)
		out.WriteInt(added)
	})
}

// SCARD
func (m *Miniredis) cmdScard(out *redeo.Responder, r *redeo.Request) error {
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
			out.WriteZero()
			return
		}

		if db.t(key) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		members := db.setMembers(key)
		out.WriteInt(len(members))
	})
}

// SDIFF
func (m *Miniredis) cmdSdiff(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	keys := r.Args

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		set, err := db.setDiff(keys)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}

		out.WriteBulkLen(len(set))
		for k := range set {
			out.WriteString(k)
		}
	})
}

// SDIFFSTORE
func (m *Miniredis) cmdSdiffstore(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	dest := r.Args[0]
	keys := r.Args[1:]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		set, err := db.setDiff(keys)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}

		db.del(dest, true)
		db.setSet(dest, set)
		out.WriteInt(len(set))
	})
}

// SINTER
func (m *Miniredis) cmdSinter(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	keys := r.Args

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		set, err := db.setInter(keys)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}

		out.WriteBulkLen(len(set))
		for k := range set {
			out.WriteString(k)
		}
	})
}

// SINTERSTORE
func (m *Miniredis) cmdSinterstore(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	dest := r.Args[0]
	keys := r.Args[1:]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		set, err := db.setInter(keys)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}

		db.del(dest, true)
		db.setSet(dest, set)
		out.WriteInt(len(set))
	})
}

// SISMEMBER
func (m *Miniredis) cmdSismember(out *redeo.Responder, r *redeo.Request) error {
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

		if db.t(key) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		if db.setIsMember(key, value) {
			out.WriteOne()
			return
		}
		out.WriteZero()
	})
}

// SMEMBERS
func (m *Miniredis) cmdSmembers(out *redeo.Responder, r *redeo.Request) error {
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

		if db.t(key) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		members := db.setMembers(key)

		out.WriteBulkLen(len(members))
		for _, elem := range members {
			out.WriteString(elem)
		}
	})
}

// SMOVE
func (m *Miniredis) cmdSmove(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	src := r.Args[0]
	dst := r.Args[1]
	member := r.Args[2]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(src) {
			out.WriteInt(0)
			return
		}

		if db.t(src) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		if db.exists(dst) && db.t(dst) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		if !db.setIsMember(src, member) {
			out.WriteInt(0)
			return
		}
		db.setRem(src, member)
		db.setAdd(dst, member)
		out.WriteInt(1)
	})
}

// SPOP
func (m *Miniredis) cmdSpop(out *redeo.Responder, r *redeo.Request) error {
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

		if db.t(key) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		members := db.setMembers(key)
		member := members[rand.Intn(len(members))]
		db.setRem(key, member)
		out.WriteString(member)
	})
}

// SRANDMEMBER
func (m *Miniredis) cmdSrandmember(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if len(r.Args) > 2 {
		setDirty(r.Client())
		out.WriteErrorString(msgSyntaxError)
		return nil
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	count := 0
	withCount := false
	if len(r.Args) == 2 {
		var err error
		count, err = strconv.Atoi(r.Args[1])
		if err != nil {
			setDirty(r.Client())
			out.WriteErrorString(msgInvalidInt)
			return nil
		}
		withCount = true
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(key) {
			out.WriteNil()
			return
		}

		if db.t(key) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		members := db.setMembers(key)
		if count < 0 {
			// Non-unique elements is allowed with negative count.
			out.WriteBulkLen(-count)
			for count != 0 {
				member := members[rand.Intn(len(members))]
				out.WriteString(member)
				count++
			}
			return
		}

		// Must be unique elements.
		shuffle(members)
		if count > len(members) {
			count = len(members)
		}
		if !withCount {
			out.WriteString(members[0])
			return
		}
		out.WriteBulkLen(count)
		for i := range make([]struct{}, count) {
			out.WriteString(members[i])
		}
	})
}

// SREM
func (m *Miniredis) cmdSrem(out *redeo.Responder, r *redeo.Request) error {
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

		if !db.exists(key) {
			out.WriteInt(0)
			return
		}

		if db.t(key) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		out.WriteInt(db.setRem(key, fields...))
	})
}

// SUNION
func (m *Miniredis) cmdSunion(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	keys := r.Args

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		set, err := db.setUnion(keys)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}

		out.WriteBulkLen(len(set))
		for k := range set {
			out.WriteString(k)
		}
	})
}

// SUNIONSTORE
func (m *Miniredis) cmdSunionstore(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	dest := r.Args[0]
	keys := r.Args[1:]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		set, err := db.setUnion(keys)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}

		db.del(dest, true)
		db.setSet(dest, set)
		out.WriteInt(len(set))
	})
}

// SSCAN
func (m *Miniredis) cmdSscan(out *redeo.Responder, r *redeo.Request) error {
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
		if db.exists(key) && db.t(key) != "set" {
			out.WriteErrorString(ErrWrongType.Error())
			return
		}

		members := db.setMembers(key)
		if withMatch {
			members = matchKeys(members, match)
		}

		out.WriteBulkLen(2)
		out.WriteString("0") // no next cursor
		out.WriteBulkLen(len(members))
		for _, k := range members {
			out.WriteString(k)
		}
	})
}

// shuffle shuffles a string. Kinda.
func shuffle(m []string) {
	for _ = range m {
		i := rand.Intn(len(m))
		j := rand.Intn(len(m))
		m[i], m[j] = m[j], m[i]
	}
}
