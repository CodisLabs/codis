// Commands from http://redis.io/commands#connection

package miniredis

import (
	"strconv"

	"github.com/bsm/redeo"
)

func commandsConnection(m *Miniredis, srv *redeo.Server) {
	srv.HandleFunc("AUTH", m.cmdAuth)
	srv.HandleFunc("ECHO", m.cmdEcho)
	srv.HandleFunc("PING", m.cmdPing)
	srv.HandleFunc("SELECT", m.cmdSelect)
	srv.HandleFunc("QUIT", m.cmdQuit)
}

// PING
func (m *Miniredis) cmdPing(out *redeo.Responder, r *redeo.Request) error {
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	out.WriteInlineString("PONG")
	return nil
}

// AUTH
func (m *Miniredis) cmdAuth(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	pw := r.Args[0]

	m.Lock()
	defer m.Unlock()
	if m.password == "" {
		out.WriteErrorString("ERR Client sent AUTH, but no password is set")
		return nil
	}
	if m.password != pw {
		out.WriteErrorString("ERR invalid password")
		return nil
	}

	setAuthenticated(r.Client())
	out.WriteOK()
	return nil
}

// ECHO
func (m *Miniredis) cmdEcho(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}
	msg := r.Args[0]
	out.WriteString(msg)
	return nil
}

// SELECT
func (m *Miniredis) cmdSelect(out *redeo.Responder, r *redeo.Request) error {
	if len(r.Args) != 1 {
		setDirty(r.Client())
		return r.WrongNumberOfArgs()
	}
	if !m.handleAuth(r.Client(), out) {
		return nil
	}

	id, err := strconv.Atoi(r.Args[0])
	if err != nil {
		id = 0
	}

	m.Lock()
	defer m.Unlock()

	ctx := getCtx(r.Client())
	ctx.selectedDB = id

	out.WriteOK()
	return nil
}

// QUIT
func (m *Miniredis) cmdQuit(out *redeo.Responder, r *redeo.Request) error {
	// QUIT isn't transactionfied and accepts any arguments.
	out.WriteOK()
	r.Client().Close()
	return nil
}
