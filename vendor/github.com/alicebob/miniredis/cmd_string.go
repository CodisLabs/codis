// Commands from http://redis.io/commands#string

package miniredis

import (
	"strconv"
	"strings"

	"github.com/bsm/redeo"
)

// commandsString handles all string value operations.
func commandsString(m *Miniredis, srv *redeo.Server) {
	srv.HandleFunc("APPEND", m.cmdAppend)
	srv.HandleFunc("BITCOUNT", m.cmdBitcount)
	srv.HandleFunc("BITOP", m.cmdBitop)
	srv.HandleFunc("BITPOS", m.cmdBitpos)
	srv.HandleFunc("DECRBY", m.cmdDecrby)
	srv.HandleFunc("DECR", m.cmdDecr)
	srv.HandleFunc("GETBIT", m.cmdGetbit)
	srv.HandleFunc("GET", m.cmdGet)
	srv.HandleFunc("GETRANGE", m.cmdGetrange)
	srv.HandleFunc("GETSET", m.cmdGetset)
	srv.HandleFunc("INCRBYFLOAT", m.cmdIncrbyfloat)
	srv.HandleFunc("INCRBY", m.cmdIncrby)
	srv.HandleFunc("INCR", m.cmdIncr)
	srv.HandleFunc("MGET", m.cmdMget)
	srv.HandleFunc("MSET", m.cmdMset)
	srv.HandleFunc("MSETNX", m.cmdMsetnx)
	srv.HandleFunc("PSETEX", m.cmdPsetex)
	srv.HandleFunc("SETBIT", m.cmdSetbit)
	srv.HandleFunc("SETEX", m.cmdSetex)
	srv.HandleFunc("SET", m.cmdSet)
	srv.HandleFunc("SETNX", m.cmdSetnx)
	srv.HandleFunc("SETRANGE", m.cmdSetrange)
	srv.HandleFunc("STRLEN", m.cmdStrlen)
}

// SET
func (m *Miniredis) cmdSet(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	var (
		nx     = false // set iff not exists
		xx     = false // set iff exists
		expire = 0     // For seconds and milliseconds.
	)

	key := r.Args[0]
	value := r.Args[1]
	r.Args = r.Args[2:]
	for len(r.Args) > 0 {
		switch strings.ToUpper(r.Args[0]) {
		case "NX":
			nx = true
			r.Args = r.Args[1:]
			continue
		case "XX":
			xx = true
			r.Args = r.Args[1:]
			continue
		case "EX", "PX":
			if len(r.Args) < 2 {
				setDirty(r.Client())
				out.WriteErrorString(msgInvalidInt)
				return nil
			}
			var err error
			expire, err = strconv.Atoi(r.Args[1])
			if err != nil {
				setDirty(r.Client())
				out.WriteErrorString(msgInvalidInt)
				return nil
			}
			r.Args = r.Args[2:]
			continue
		default:
			setDirty(r.Client())
			out.WriteErrorString(msgSyntaxError)
			return nil
		}
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if nx {
			if db.exists(key) {
				out.WriteNil()
				return
			}
		}
		if xx {
			if !db.exists(key) {
				out.WriteNil()
				return
			}
		}

		db.del(key, true) // be sure to remove existing values of other type keys.
		// a vanilla SET clears the expire
		db.stringSet(key, value)
		if expire != 0 {
			db.expire[key] = expire
		}
		out.WriteOK()
	})
}

// SETEX
func (m *Miniredis) cmdSetex(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	ttl, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}
	value := r.Args[2]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		db.del(key, true) // Clear any existing keys.
		db.stringSet(key, value)
		db.expire[key] = ttl
		out.WriteOK()
	})
}

// PSETEX
func (m *Miniredis) cmdPsetex(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	key := r.Args[0]
	ttl, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}
	value := r.Args[2]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		db.del(key, true) // Clear any existing keys.
		db.stringSet(key, value)
		db.expire[key] = ttl // We put millisecond keys in with the second keys.
		out.WriteOK()
	})
}

// SETNX
func (m *Miniredis) cmdSetnx(out *redeo.Responder, r *redeo.Request) error {
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

		if _, ok := db.keys[key]; ok {
			out.WriteZero()
			return
		}

		db.stringSet(key, value)
		out.WriteOne()
	})
}

// MSET
func (m *Miniredis) cmdMset(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	if len(r.Args)%2 != 0 {
		setDirty(r.Client())
		// non-default error message
		out.WriteErrorString("ERR wrong number of arguments for MSET")
		return nil
	}
	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		for len(r.Args) > 0 {
			key := r.Args[0]
			value := r.Args[1]
			r.Args = r.Args[2:]

			db.del(key, true) // clear TTL
			db.stringSet(key, value)
		}
		out.WriteOK()
	})
}

// MSETNX
func (m *Miniredis) cmdMsetnx(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	if len(r.Args)%2 != 0 {
		setDirty(r.Client())
		// non-default error message (yes, with 'MSET').
		out.WriteErrorString("ERR wrong number of arguments for MSET")
		return nil
	}
	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		keys := map[string]string{}
		existing := false
		for len(r.Args) > 0 {
			key := r.Args[0]
			value := r.Args[1]
			r.Args = r.Args[2:]
			keys[key] = value
			if _, ok := db.keys[key]; ok {
				existing = true
			}
		}

		res := 0
		if !existing {
			res = 1
			for k, v := range keys {
				// Nothing to delete. That's the whole point.
				db.stringSet(k, v)
			}
		}
		out.WriteInt(res)
	})
}

// GET
func (m *Miniredis) cmdGet(out *redeo.Responder, r *redeo.Request) error {
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
		if db.t(key) != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		out.WriteString(db.stringGet(key))
	})
}

// GETSET
func (m *Miniredis) cmdGetset(out *redeo.Responder, r *redeo.Request) error {
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

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		old, ok := db.stringKeys[key]
		db.stringSet(key, value)
		// a GETSET clears the expire
		delete(db.expire, key)

		if !ok {
			out.WriteNil()
			return
		}
		out.WriteString(old)
		return
	})
}

// MGET
func (m *Miniredis) cmdMget(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		out.WriteBulkLen(len(r.Args))
		for _, k := range r.Args {
			if t, ok := db.keys[k]; !ok || t != "string" {
				out.WriteNil()
				continue
			}
			v, ok := db.stringKeys[k]
			if !ok {
				// Should not happen, we just checked keys[]
				out.WriteNil()
				continue
			}
			out.WriteString(v)
		}
	})
}

// INCR
func (m *Miniredis) cmdIncr(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		key := r.Args[0]
		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}
		v, err := db.stringIncr(key, +1)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}
		// Don't touch TTL
		out.WriteInt(v)
	})
}

// INCRBY
func (m *Miniredis) cmdIncrby(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	delta, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		v, err := db.stringIncr(key, delta)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}
		// Don't touch TTL
		out.WriteInt(v)
	})
}

// INCRBYFLOAT
func (m *Miniredis) cmdIncrbyfloat(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	delta, err := strconv.ParseFloat(r.Args[1], 64)
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidFloat)
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		v, err := db.stringIncrfloat(key, delta)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}
		// Don't touch TTL
		out.WriteString(formatFloat(v))
	})
}

// DECR
func (m *Miniredis) cmdDecr(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		key := r.Args[0]
		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}
		v, err := db.stringIncr(key, -1)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}
		// Don't touch TTL
		out.WriteInt(v)
	})
}

// DECRBY
func (m *Miniredis) cmdDecrby(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	delta, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		v, err := db.stringIncr(key, -delta)
		if err != nil {
			out.WriteErrorString(err.Error())
			return
		}
		// Don't touch TTL
		out.WriteInt(v)
	})
}

// STRLEN
func (m *Miniredis) cmdStrlen(out *redeo.Responder, r *redeo.Request) error {
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

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		out.WriteInt(len(db.stringKeys[key]))
	})
}

// APPEND
func (m *Miniredis) cmdAppend(out *redeo.Responder, r *redeo.Request) error {
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

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		newValue := db.stringKeys[key] + value
		db.stringSet(key, newValue)

		out.WriteInt(len(newValue))
	})
}

// GETRANGE
func (m *Miniredis) cmdGetrange(out *redeo.Responder, r *redeo.Request) error {
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

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		v := db.stringKeys[key]
		out.WriteString(withRange(v, start, end))
	})
}

// SETRANGE
func (m *Miniredis) cmdSetrange(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	pos, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}
	if pos < 0 {
		setDirty(r.Client())
		return redeo.ClientError("offset is out of range")
	}
	subst := r.Args[2]

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		v := []byte(db.stringKeys[key])
		if len(v) < pos+len(subst) {
			newV := make([]byte, pos+len(subst))
			copy(newV, v)
			v = newV
		}
		copy(v[pos:pos+len(subst)], subst)
		db.stringSet(key, string(v))
		out.WriteInt(len(v))
	})
}

// BITCOUNT
func (m *Miniredis) cmdBitcount(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	var (
		key        = r.Args[0]
		useRange   = false
		start, end = 0, 0
		args       = r.Args[1:]
	)
	if len(args) >= 2 {
		useRange = true
		var err error
		start, err = strconv.Atoi(args[0])
		if err != nil {
			setDirty(r.Client())
			out.WriteErrorString(msgInvalidInt)
			return nil
		}
		end, err = strconv.Atoi(args[1])
		if err != nil {
			setDirty(r.Client())
			out.WriteErrorString(msgInvalidInt)
			return nil
		}
		args = args[2:]
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if !db.exists(key) {
			out.WriteZero()
			return
		}
		if db.t(key) != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}

		// Real redis only checks after it knows the key is there and a string.
		if len(args) != 0 {
			out.WriteErrorString(msgSyntaxError)
			return
		}

		v := db.stringKeys[key]
		if useRange {
			v = withRange(v, start, end)
		}

		out.WriteInt(countBits([]byte(v)))
	})
}

// BITOP
func (m *Miniredis) cmdBitop(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	var (
		op     = strings.ToUpper(r.Args[0])
		target = r.Args[1]
		input  = r.Args[2:]
	)

	// 'op' is tested when the transaction is executed.
	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		switch op {
		case "AND", "OR", "XOR":
			first := input[0]
			if t, ok := db.keys[first]; ok && t != "string" {
				out.WriteErrorString(msgWrongType)
				return
			}
			res := []byte(db.stringKeys[first])
			for _, vk := range input[1:] {
				if t, ok := db.keys[vk]; ok && t != "string" {
					out.WriteErrorString(msgWrongType)
					return
				}
				v := db.stringKeys[vk]
				cb := map[string]func(byte, byte) byte{
					"AND": func(a, b byte) byte { return a & b },
					"OR":  func(a, b byte) byte { return a | b },
					"XOR": func(a, b byte) byte { return a ^ b },
				}[op]
				res = sliceBinOp(cb, res, []byte(v))
			}
			db.del(target, false) // Keep TTL
			if len(res) == 0 {
				db.del(target, true)
			} else {
				db.stringSet(target, string(res))
			}
			out.WriteInt(len(res))
		case "NOT":
			// NOT only takes a single argument.
			if len(input) != 1 {
				out.WriteErrorString("ERR BITOP NOT must be called with a single source key.")
				return
			}
			key := input[0]
			if t, ok := db.keys[key]; ok && t != "string" {
				out.WriteErrorString(msgWrongType)
				return
			}
			value := []byte(db.stringKeys[key])
			for i := range value {
				value[i] = ^value[i]
			}
			db.del(target, false) // Keep TTL
			if len(value) == 0 {
				db.del(target, true)
			} else {
				db.stringSet(target, string(value))
			}
			out.WriteInt(len(value))
		default:
			out.WriteErrorString(msgSyntaxError)
		}
	})
}

// BITPOS
func (m *Miniredis) cmdBitpos(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) < 2 || len(r.Args) > 4 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	bit, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		out.WriteErrorString(msgInvalidInt)
		return nil
	}
	var start, end int
	withEnd := false
	if len(r.Args) > 2 {
		start, err = strconv.Atoi(r.Args[2])
		if err != nil {
			setDirty(r.Client())
			out.WriteErrorString(msgInvalidInt)
			return nil
		}
	}
	if len(r.Args) > 3 {
		end, err = strconv.Atoi(r.Args[3])
		if err != nil {
			setDirty(r.Client())
			out.WriteErrorString(msgInvalidInt)
			return nil
		}
		withEnd = true
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}
		value := db.stringKeys[key]
		if start != 0 {
			if start > len(value) {
				start = len(value)
			}
		}
		if withEnd {
			end++ // redis end semantics.
			if end < 0 {
				end = len(value) + end
			}
			if end > len(value) {
				end = len(value)
			}
		} else {
			end = len(value)
		}
		if start != 0 || withEnd {
			if end < start {
				value = ""
			} else {
				value = value[start:end]
			}
		}
		pos := bitPos([]byte(value), bit == 1)
		if pos >= 0 {
			pos += start * 8
		}
		// Special case when looking for 0, but not when start and end are
		// given.
		if bit == 0 && pos == -1 && !withEnd {
			pos = start*8 + len(value)*8
		}
		out.WriteInt(pos)
	})
}

// GETBIT
func (m *Miniredis) cmdGetbit(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 2 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	bit, err := strconv.Atoi(r.Args[1])
	if err != nil {
		setDirty(r.Client())
		return redeo.ClientError("bit offset is not an integer or out of range")
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}
		value := db.stringKeys[key]

		ourByteNr := bit / 8
		var ourByte byte
		if ourByteNr > len(value)-1 {
			ourByte = '\x00'
		} else {
			ourByte = value[ourByteNr]
		}
		res := 0
		if toBits(ourByte)[bit%8] {
			res = 1
		}
		out.WriteInt(res)
	})
}

// SETBIT
func (m *Miniredis) cmdSetbit(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 3 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	key := r.Args[0]
	bit, err := strconv.Atoi(r.Args[1])
	if err != nil || bit < 0 {
		setDirty(r.Client())
		return redeo.ClientError("bit offset is not an integer or out of range")
	}
	newBit, err := strconv.Atoi(r.Args[2])
	if err != nil || (newBit != 0 && newBit != 1) {
		setDirty(r.Client())
		return redeo.ClientError("bit is not an integer or out of range")
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		if t, ok := db.keys[key]; ok && t != "string" {
			out.WriteErrorString(msgWrongType)
			return
		}
		value := []byte(db.stringKeys[key])

		ourByteNr := bit / 8
		ourBitNr := bit % 8
		if ourByteNr > len(value)-1 {
			// Too short. Expand.
			newValue := make([]byte, ourByteNr+1)
			copy(newValue, value)
			value = newValue
		}
		old := 0
		if toBits(value[ourByteNr])[ourBitNr] {
			old = 1
		}
		if newBit == 0 {
			value[ourByteNr] &^= 1 << uint8(7-ourBitNr)
		} else {
			value[ourByteNr] |= 1 << uint8(7-ourBitNr)
		}
		db.stringSet(key, string(value))

		out.WriteInt(old)
	})
}

// Redis range. both start and end can be negative.
func withRange(v string, start, end int) string {
	s, e := redisRange(len(v), start, end, true /* string getrange symantics */)
	return v[s:e]
}

func countBits(v []byte) int {
	count := 0
	for _, b := range []byte(v) {
		for b > 0 {
			count += int((b % uint8(2)))
			b = b >> 1
		}
	}
	return count
}

// sliceBinOp applies an operator to all slice elements, with Redis string
// padding logic.
func sliceBinOp(f func(a, b byte) byte, a, b []byte) []byte {
	maxl := len(a)
	if len(b) > maxl {
		maxl = len(b)
	}
	lA := make([]byte, maxl)
	copy(lA, a)
	lB := make([]byte, maxl)
	copy(lB, b)
	res := make([]byte, maxl)
	for i := range res {
		res[i] = f(lA[i], lB[i])
	}
	return res
}

// Return the number of the first bit set/unset.
func bitPos(s []byte, bit bool) int {
	for i, b := range s {
		for j, set := range toBits(b) {
			if set == bit {
				return i*8 + j
			}
		}
	}
	return -1
}

// toBits changes a byte in 8 bools.
func toBits(s byte) [8]bool {
	r := [8]bool{}
	for i := range r {
		if s&(uint8(1)<<uint8(7-i)) != 0 {
			r[i] = true
		}
	}
	return r
}
