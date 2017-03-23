// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"
)

type rollingFile struct {
	mu sync.Mutex

	closed bool

	file     *os.File
	basePath string
	filePath string
	fileFrag string

	rolling RollingFormat
}

var ErrClosedRollingFile = errors.New("rolling file is closed")

type RollingFormat string

const (
	MonthlyRolling  RollingFormat = "2006-01"
	DailyRolling                  = "2006-01-02"
	HourlyRolling                 = "2006-01-02-15"
	MinutelyRolling               = "2006-01-02-15-04"
	SecondlyRolling               = "2006-01-02-15-04-05"
)

func (r *rollingFile) roll() error {
	suffix := time.Now().Format(string(r.rolling))
	if r.file != nil {
		if suffix == r.fileFrag {
			return nil
		}
		r.file.Close()
		r.file = nil
	}
	r.fileFrag = suffix
	r.filePath = fmt.Sprintf("%s.%s", r.basePath, r.fileFrag)

	if dir, _ := filepath.Split(r.basePath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return errors.Trace(err)
		}
	}

	f, err := os.OpenFile(r.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return errors.Trace(err)
	} else {
		r.file = f
		return nil
	}
}

func (r *rollingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	if f := r.file; f != nil {
		r.file = nil
		return errors.Trace(f.Close())
	}
	return nil
}

func (r *rollingFile) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return 0, errors.Trace(ErrClosedRollingFile)
	}

	if err := r.roll(); err != nil {
		return 0, err
	}

	n, err := r.file.Write(b)
	if err != nil {
		return n, errors.Trace(err)
	} else {
		return n, nil
	}
}

func NewRollingFile(basePath string, rolling RollingFormat) (io.WriteCloser, error) {
	if _, file := filepath.Split(basePath); file == "" {
		return nil, errors.Errorf("invalid base-path = %s, file name is required", basePath)
	}
	return &rollingFile{basePath: basePath, rolling: rolling}, nil
}
