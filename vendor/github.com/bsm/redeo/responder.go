package redeo

import (
	"io"
	"strconv"
	"strings"
)

const (
	CodeInline  = '+'
	CodeError   = '-'
	CodeFixnum  = ':'
	CodeStrLen  = '$'
	CodeBulkLen = '*'
)

var (
	binCRLF = []byte("\r\n")
	binOK   = []byte("+OK\r\n")
	binZERO = []byte(":0\r\n")
	binONE  = []byte(":1\r\n")
	binNIL  = []byte("$-1\r\n")
)

// Responder generates client responses
type Responder struct {
	w io.Writer

	err     error
	written bool
}

// NewResponder creates a new responder instance
func NewResponder(w io.Writer) *Responder {
	return &Responder{w: w}
}

// Valid returns true if still accepting writes
func (res *Responder) Valid() bool {
	return res.err == nil
}

// WriteBulkLen writes a bulk length
func (res *Responder) WriteBulkLen(n int) {
	res.inline(CodeBulkLen, strconv.Itoa(n))
}

// WriteBulk writes a slice
func (res *Responder) WriteBulk(bulk [][]byte) {
	res.WriteBulkLen(len(bulk))
	for _, b := range bulk {
		if b == nil {
			res.WriteNil()
		} else {
			res.WriteBytes(b)
		}
	}
}

// WriteStringBulk writes a string slice
func (res *Responder) WriteStringBulk(bulk []string) {
	res.WriteBulkLen(len(bulk))
	for _, b := range bulk {
		res.WriteString(b)
	}
}

// WriteString writes a bulk string
func (res *Responder) WriteString(s string) {
	lns := len(s)
	ssz := strconv.Itoa(lns)
	lnz := len(ssz)

	p := make([]byte, lns+lnz+5)
	p[0] = CodeStrLen
	copy(p[1:], ssz)
	copy(p[1+lnz:], binCRLF)
	copy(p[3+lnz:], s)
	copy(p[3+lnz+lns:], binCRLF)
	res.Write(p)
}

// WriteBytes writes a bulk string
func (res *Responder) WriteBytes(b []byte) {
	lnb := len(b)
	ssz := strconv.Itoa(lnb)
	lnz := len(ssz)

	p := make([]byte, lnb+lnz+5)
	p[0] = CodeStrLen
	copy(p[1:], ssz)
	copy(p[1+lnz:], binCRLF)
	copy(p[3+lnz:], b)
	copy(p[3+lnz+lnb:], binCRLF)
	res.Write(p)
}

// WriteString writes an inline string
func (res *Responder) WriteInlineString(s string) {
	res.inline(CodeInline, s)
}

// WriteNil writes a nil value
func (res *Responder) WriteNil() {
	res.Write(binNIL)
}

// WriteOK writes OK
func (res *Responder) WriteOK() {
	res.Write(binOK)
}

// WriteInt writes an inline integer
func (res *Responder) WriteInt(n int) {
	res.inline(CodeFixnum, strconv.Itoa(n))
}

// WriteZero writes a 0 integer
func (res *Responder) WriteZero() {
	res.Write(binZERO)
}

// WriteOne writes a 1 integer
func (res *Responder) WriteOne() {
	res.Write(binONE)
}

// WriteErrorString writes an error string
func (res *Responder) WriteErrorString(s string) {
	res.inline(CodeError, s)
}

// WriteError writes an error using the standard "ERR message" format
func (res *Responder) WriteError(err error) {
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i > -1 {
		s = s[i+2:]
	}
	res.WriteErrorString("ERR " + s)
}

// WriteN streams data from a reader
func (res *Responder) WriteN(rd io.Reader, n int64) {
	o := strconv.FormatInt(n, 10)
	b := append([]byte{CodeStrLen}, []byte(o)...)

	res.Write(append(b, binCRLF...))
	if res.Valid() {
		io.CopyN(res.w, rd, n)
	}
	res.Write(binCRLF)
}

// Write allows servers to write raw data straight to the socket without buffering.
// This is useful e.g. for streaming responses but may break the redis protocol.
// Be careful with this!
func (res *Responder) Write(p []byte) (int, error) {
	if res.err != nil {
		return 0, res.err
	}

	res.written = true
	n, err := res.w.Write(p)
	if err != nil {
		res.err = err
	}
	return n, err
}

// ------------------------------------------------------------------------

func (res *Responder) inline(prefix byte, s string) {
	p := make([]byte, len(s)+3)
	p[0] = prefix
	copy(p[1:], s)
	copy(p[len(s)+1:], binCRLF)
	res.Write(p)
}
