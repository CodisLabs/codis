// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package log

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/trace"
)

const (
	Ldate         = log.Ldate
	Llongfile     = log.Llongfile
	Lmicroseconds = log.Lmicroseconds
	Lshortfile    = log.Lshortfile
	LstdFlags     = log.LstdFlags
	Ltime         = log.Ltime
)

type (
	LogType  int64
	LogLevel int64
)

const (
	TYPE_ERROR = LogType(1 << iota)
	TYPE_WARN
	TYPE_INFO
	TYPE_DEBUG
	TYPE_PANIC = LogType(^0)
)

const (
	LevelNone = LogLevel(1<<iota - 1)
	LevelError
	LevelWarn
	LevelInfo
	LevelDebug
	LevelAll = LevelDebug
)

func (t LogType) String() string {
	switch t {
	default:
		return "[LOG]"
	case TYPE_PANIC:
		return "[PANIC]"
	case TYPE_ERROR:
		return "[ERROR]"
	case TYPE_WARN:
		return "[WARN]"
	case TYPE_INFO:
		return "[INFO]"
	case TYPE_DEBUG:
		return "[DEBUG]"
	}
}

func (l LogLevel) String() string {
	switch l {
	default:
		return "UNKNOWN"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelNone:
		return "NONE"
	}
}

func (l *LogLevel) ParseFromString(s string) bool {
	switch strings.ToUpper(s) {
	case "ERROR":
		*l = LevelError
	case "DEBUG":
		*l = LevelDebug
	case "WARN", "WARNING":
		*l = LevelWarn
	case "INFO":
		*l = LevelInfo
	case "NONE":
		*l = LevelNone
	default:
		return false
	}
	return true
}

func (l *LogLevel) Set(v LogLevel) {
	atomic.StoreInt64((*int64)(l), int64(v))
}

func (l *LogLevel) Test(m LogType) bool {
	v := atomic.LoadInt64((*int64)(l))
	return (v & int64(m)) != 0
}

type nopCloser struct {
	io.Writer
}

func (*nopCloser) Close() error {
	return nil
}

func NopCloser(w io.Writer) io.WriteCloser {
	return &nopCloser{w}
}

type Logger struct {
	mu    sync.Mutex
	out   io.WriteCloser
	log   *log.Logger
	level LogLevel
	trace LogLevel
}

var StdLog = New(NopCloser(os.Stderr), "")

func New(writer io.Writer, prefix string) *Logger {
	out, ok := writer.(io.WriteCloser)
	if !ok {
		out = NopCloser(writer)
	}
	return &Logger{
		out:   out,
		log:   log.New(out, prefix, LstdFlags|Lshortfile),
		level: LevelAll,
		trace: LevelError,
	}
}

func (l *Logger) Flags() int {
	return l.log.Flags()
}

func (l *Logger) Prefix() string {
	return l.log.Prefix()
}

func (l *Logger) SetFlags(flags int) {
	l.log.SetFlags(flags)
}

func (l *Logger) SetPrefix(prefix string) {
	l.log.SetPrefix(prefix)
}

func (l *Logger) SetLevel(v LogLevel) {
	l.level.Set(v)
}

func (l *Logger) SetLevelString(s string) bool {
	var v LogLevel
	if !v.ParseFromString(s) {
		return false
	} else {
		l.SetLevel(v)
		return true
	}
}

func (l *Logger) SetTraceLevel(v LogLevel) {
	l.trace.Set(v)
}

func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out.Close()
}

func (l *Logger) isDisabled(t LogType) bool {
	return t != TYPE_PANIC && !l.level.Test(t)
}

func (l *Logger) isTraceEnabled(t LogType) bool {
	return t == TYPE_PANIC || l.trace.Test(t)
}

func (l *Logger) Panic(v ...interface{}) {
	t := TYPE_PANIC
	s := fmt.Sprint(v...)
	l.output(1, nil, t, s)
	os.Exit(1)
}

func (l *Logger) Panicf(format string, v ...interface{}) {
	t := TYPE_PANIC
	s := fmt.Sprintf(format, v...)
	l.output(1, nil, t, s)
	os.Exit(1)
}

func (l *Logger) PanicError(err error, v ...interface{}) {
	t := TYPE_PANIC
	s := fmt.Sprint(v...)
	l.output(1, err, t, s)
	os.Exit(1)
}

func (l *Logger) PanicErrorf(err error, format string, v ...interface{}) {
	t := TYPE_PANIC
	s := fmt.Sprintf(format, v...)
	l.output(1, err, t, s)
	os.Exit(1)
}

func (l *Logger) Error(v ...interface{}) {
	t := TYPE_ERROR
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	l.output(1, nil, t, s)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	t := TYPE_ERROR
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	l.output(1, nil, t, s)
}

func (l *Logger) ErrorError(err error, v ...interface{}) {
	t := TYPE_ERROR
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	l.output(1, err, t, s)
}

func (l *Logger) ErrorErrorf(err error, format string, v ...interface{}) {
	t := TYPE_ERROR
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	l.output(1, err, t, s)
}

func (l *Logger) Warn(v ...interface{}) {
	t := TYPE_WARN
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	l.output(1, nil, t, s)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	t := TYPE_WARN
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	l.output(1, nil, t, s)
}

func (l *Logger) WarnError(err error, v ...interface{}) {
	t := TYPE_WARN
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	l.output(1, err, t, s)
}

func (l *Logger) WarnErrorf(err error, format string, v ...interface{}) {
	t := TYPE_WARN
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	l.output(1, err, t, s)
}

func (l *Logger) Info(v ...interface{}) {
	t := TYPE_INFO
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	l.output(1, nil, t, s)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	t := TYPE_INFO
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	l.output(1, nil, t, s)
}

func (l *Logger) InfoError(err error, v ...interface{}) {
	t := TYPE_INFO
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	l.output(1, err, t, s)
}

func (l *Logger) InfoErrorf(err error, format string, v ...interface{}) {
	t := TYPE_INFO
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	l.output(1, err, t, s)
}

func (l *Logger) Debug(v ...interface{}) {
	t := TYPE_DEBUG
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	l.output(1, nil, t, s)
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	t := TYPE_DEBUG
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	l.output(1, nil, t, s)
}

func (l *Logger) DebugError(err error, v ...interface{}) {
	t := TYPE_DEBUG
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	l.output(1, err, t, s)
}

func (l *Logger) DebugErrorf(err error, format string, v ...interface{}) {
	t := TYPE_DEBUG
	if l.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	l.output(1, err, t, s)
}

func (l *Logger) Print(v ...interface{}) {
	s := fmt.Sprint(v...)
	l.output(1, nil, 0, s)
}

func (l *Logger) Printf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.output(1, nil, 0, s)
}

func (l *Logger) Println(v ...interface{}) {
	s := fmt.Sprintln(v...)
	l.output(1, nil, 0, s)
}

func (l *Logger) output(traceskip int, err error, t LogType, s string) error {
	var stack trace.Stack
	if l.isTraceEnabled(t) {
		stack = trace.TraceN(traceskip+1, 32)
	}

	var b bytes.Buffer
	fmt.Fprint(&b, t, " ", s)

	if len(s) == 0 || s[len(s)-1] != '\n' {
		fmt.Fprint(&b, "\n")
	}

	if err != nil {
		fmt.Fprint(&b, "[error]: ", err.Error(), "\n")
		if stack := errors.Stack(err); stack != nil {
			fmt.Fprint(&b, stack.StringWithIndent(1))
		}
	}
	if len(stack) != 0 {
		fmt.Fprint(&b, "[stack]: \n", stack.StringWithIndent(1))
	}

	s = b.String()
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.log.Output(traceskip+2, s)
}

func Flags() int {
	return StdLog.Flags()
}

func Prefix() string {
	return StdLog.Prefix()
}

func SetFlags(flags int) {
	StdLog.SetFlags(flags)
}

func SetPrefix(prefix string) {
	StdLog.SetPrefix(prefix)
}

func SetLevel(v LogLevel) {
	StdLog.SetLevel(v)
}

func SetLevelString(s string) bool {
	return StdLog.SetLevelString(s)
}

func SetTrace(v LogLevel) {
	StdLog.SetTraceLevel(v)
}

func Panic(v ...interface{}) {
	t := TYPE_PANIC
	s := fmt.Sprint(v...)
	StdLog.output(1, nil, t, s)
	os.Exit(1)
}

func Panicf(format string, v ...interface{}) {
	t := TYPE_PANIC
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, nil, t, s)
	os.Exit(1)
}

func PanicError(err error, v ...interface{}) {
	t := TYPE_PANIC
	s := fmt.Sprint(v...)
	StdLog.output(1, err, t, s)
	os.Exit(1)
}

func PanicErrorf(err error, format string, v ...interface{}) {
	t := TYPE_PANIC
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, err, t, s)
	os.Exit(1)
}

func Error(v ...interface{}) {
	t := TYPE_ERROR
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	StdLog.output(1, nil, t, s)
}

func Errorf(format string, v ...interface{}) {
	t := TYPE_ERROR
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, nil, t, s)
}

func ErrorError(err error, v ...interface{}) {
	t := TYPE_ERROR
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	StdLog.output(1, err, t, s)
}

func ErrorErrorf(err error, format string, v ...interface{}) {
	t := TYPE_ERROR
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, err, t, s)
}

func Warn(v ...interface{}) {
	t := TYPE_WARN
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	StdLog.output(1, nil, t, s)
}

func Warnf(format string, v ...interface{}) {
	t := TYPE_WARN
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, nil, t, s)
}

func WarnError(err error, v ...interface{}) {
	t := TYPE_WARN
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	StdLog.output(1, err, t, s)
}

func WarnErrorf(err error, format string, v ...interface{}) {
	t := TYPE_WARN
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, err, t, s)
}

func Info(v ...interface{}) {
	t := TYPE_INFO
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	StdLog.output(1, nil, t, s)
}

func Infof(format string, v ...interface{}) {
	t := TYPE_INFO
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, nil, t, s)
}

func InfoError(err error, v ...interface{}) {
	t := TYPE_INFO
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	StdLog.output(1, err, t, s)
}

func InfoErrorf(err error, format string, v ...interface{}) {
	t := TYPE_INFO
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, err, t, s)
}

func Debug(v ...interface{}) {
	t := TYPE_DEBUG
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	StdLog.output(1, nil, t, s)
}

func Debugf(format string, v ...interface{}) {
	t := TYPE_DEBUG
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, nil, t, s)
}

func DebugError(err error, v ...interface{}) {
	t := TYPE_DEBUG
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprint(v...)
	StdLog.output(1, err, t, s)
}

func DebugErrorf(err error, format string, v ...interface{}) {
	t := TYPE_DEBUG
	if StdLog.isDisabled(t) {
		return
	}
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, err, t, s)
}

func Print(v ...interface{}) {
	s := fmt.Sprint(v...)
	StdLog.output(1, nil, 0, s)
}

func Printf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	StdLog.output(1, nil, 0, s)
}

func Println(v ...interface{}) {
	s := fmt.Sprintln(v...)
	StdLog.output(1, nil, 0, s)
}
