package utils

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
)

type StackRecord struct {
	Name string
	File string
	Line int
}

func (r *StackRecord) String() string {
	return fmt.Sprintf("%s:%d %s", r.File, r.Line, r.Name)
}

func Trace(skip, depth int) (records []*StackRecord, full bool) {
	records = make([]*StackRecord, 0, depth)
	for i := 0; i < depth; i++ {
		skip++
		if r, ok := Caller(skip); !ok {
			return records, true
		} else {
			records = append(records, r)
		}
	}
	return records, false
}

func Caller(skip int) (*StackRecord, bool) {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return nil, false
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil || strings.HasPrefix(fn.Name(), "runtime.") {
		return nil, false
	}
	name := fn.Name()
	return &StackRecord{
		Name: name,
		File: file,
		Line: line,
	}, true
}

func Panic(format string, v ...interface{}) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "[panic]: ")
	fmt.Fprintf(&b, format, v...)
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, traceString(1, 32))
	log.Printf("%s", b.String())
	os.Exit(1)
}

func TraceErrorf(format string, v ...interface{}) error {
	var b bytes.Buffer
	fmt.Fprintf(&b, "[error]: ")
	fmt.Fprintf(&b, format, v...)
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, traceString(1, 32))
	return errors.New(b.String())
}

func TraceError(err error) error {
	if err == nil {
		return nil
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "[error]: ")
	fmt.Fprintf(&b, "%s", err)
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, traceString(1, 32))
	return errors.New(b.String())
}

func traceString(skip, depth int) string {
	const tab = "        "
	var b bytes.Buffer
	records, full := Trace(skip+1, depth)
	for _, r := range records {
		fmt.Fprintf(&b, tab)
		fmt.Fprintf(&b, "%s:%d\n", r.File, r.Line)
		fmt.Fprintf(&b, tab)
		fmt.Fprintf(&b, tab)
		fmt.Fprintf(&b, "%s\n", r.Name)
	}
	if !full {
		fmt.Fprintf(&b, "        ")
		fmt.Fprintf(&b, "... ...\n")
	}
	return b.String()
}
