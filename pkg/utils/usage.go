package utils

import (
	"runtime"
	"syscall"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"
)

func Microseconds() int64 {
	return time.Now().UnixNano() / int64(time.Microsecond)
}

func CPUTime() (time.Duration, error) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, errors.Trace(err)
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano()), nil
}

func CPUUsage(d time.Duration) (float64, error) {
	var now = time.Now()
	b, err := CPUTime()
	if err != nil {
		return 0, err
	}
	time.Sleep(d)
	e, err := CPUTime()
	if err != nil {
		return 0, err
	}
	usage := e - b
	return float64(usage) / float64(time.Since(now)) / float64(runtime.GOMAXPROCS(0)), nil
}
