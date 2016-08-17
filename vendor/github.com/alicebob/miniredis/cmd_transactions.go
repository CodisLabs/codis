// Commands from http://redis.io/commands#transactions

package miniredis

import (
	"github.com/bsm/redeo"
)

// commandsTransaction handles MULTI &c.
func commandsTransaction(m *Miniredis, srv *redeo.Server) {
	srv.HandleFunc("DISCARD", m.cmdDiscard)
	srv.HandleFunc("EXEC", m.cmdExec)
	srv.HandleFunc("MULTI", m.cmdMulti)
	srv.HandleFunc("UNWATCH", m.cmdUnwatch)
	srv.HandleFunc("WATCH", m.cmdWatch)
}

// MULTI
func (m *Miniredis) cmdMulti(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 0 {
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	ctx := getCtx(r.Client())

	if inTx(ctx) {
		return redeo.ClientError("MULTI calls can not be nested")
	}

	startTx(ctx)

	out.WriteOK()
	return nil
}

// EXEC
func (m *Miniredis) cmdExec(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 0 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	ctx := getCtx(r.Client())

	if !inTx(ctx) {
		return redeo.ClientError("EXEC without MULTI")
	}

	if dirtyTx(ctx) {
		out.WriteErrorString("EXECABORT Transaction discarded because of previous errors.")
		return nil
	}

	m.Lock()
	defer m.Unlock()

	// Check WATCHed keys.
	for t, version := range ctx.watch {
		if m.db(t.db).keyVersion[t.key] > version {
			// Abort! Abort!
			stopTx(ctx)
			out.WriteBulkLen(0)
			return nil
		}
	}

	out.WriteBulkLen(len(ctx.transaction))
	for _, cb := range ctx.transaction {
		cb(out, ctx)
	}
	// We're done
	stopTx(ctx)
	return nil
}

// DISCARD
func (m *Miniredis) cmdDiscard(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 0 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	ctx := getCtx(r.Client())
	if !inTx(ctx) {
		return redeo.ClientError("DISCARD without MULTI")
	}

	stopTx(ctx)
	out.WriteOK()
	return nil
}

// WATCH
func (m *Miniredis) cmdWatch(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) == 0 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	ctx := getCtx(r.Client())
	if inTx(ctx) {
		return redeo.ClientError("WATCH in MULTI")
	}

	m.Lock()
	defer m.Unlock()
	db := m.db(ctx.selectedDB)

	for _, key := range r.Args {
		watch(db, ctx, key)
	}
	out.WriteOK()
	return nil
}

// UNWATCH
func (m *Miniredis) cmdUnwatch(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 0 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	// Doesn't matter if UNWATCH is in a TX or not. Looks like a Redis bug to me.
	unwatch(getCtx(r.Client()))

	return withTx(m, out, r, func(out *redeo.Responder, ctx *connCtx) {
		// Do nothing if it's called in a transaction.
		out.WriteOK()
	})
}
