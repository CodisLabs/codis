package utils

import (
	"syscall"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"
)

func SysUsage() (*syscall.Rusage, error) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return nil, errors.Trace(err)
	}
	return &usage, nil
}

func MemTotal() (int64, error) {
	u, err := SysUsage()
	if err != nil {
		return 0, errors.Trace(err)
	}
	return u.Maxrss * 1024, nil
}

func CPUTotal() (time.Duration, error) {
	u, err := SysUsage()
	if err != nil {
		return 0, errors.Trace(err)
	}
	return time.Duration(u.Utime.Nano() + u.Stime.Nano()), nil
}

func CPUUsage(d time.Duration) (float64, error) {
	var now = time.Now()
	b, err := CPUTotal()
	if err != nil {
		return 0, err
	}
	time.Sleep(d)
	e, err := CPUTotal()
	if err != nil {
		return 0, err
	}
	usage := e - b
	return float64(usage) / float64(time.Since(now)) * 100, nil
}
