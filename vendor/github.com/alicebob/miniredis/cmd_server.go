// Commands from http://redis.io/commands#server

package miniredis

import (
	"github.com/bsm/redeo"
)

func commandsServer(m *Miniredis, srv *redeo.Server) {
	srv.HandleFunc("DBSIZE", m.cmdDbsize)
	srv.HandleFunc("FLUSHALL", m.cmdFlushall)
	srv.HandleFunc("FLUSHDB", m.cmdFlushdb)
}

// DBSIZE
func (m *Miniredis) cmdDbsize(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) > 0 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		db := m.db(ctx.selectedDB)

		out.WriteInt(len(db.keys))
	})
}

// FLUSHALL
func (m *Miniredis) cmdFlushall(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) > 0 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		m.dbs = map[int]*RedisDB{}
		out.WriteOK()
	})
}

// FLUSHDB
func (m *Miniredis) cmdFlushdb(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) > 0 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		delete(m.dbs, ctx.selectedDB)
		out.WriteOK()
	})
}
