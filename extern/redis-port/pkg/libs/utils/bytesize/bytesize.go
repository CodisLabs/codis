// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package bytesize

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
)

const (
	B = 1 << (10 * iota)
	KB
	MB
	GB
	TB
	PB
)

var (
	BytesizeRegexp = regexp.MustCompile(`(?i)^\s*(\-?[\d\.]+)\s*([KMGTP]?B|[BKMGTP]|)\s*$`)
	digitsRegexp   = regexp.MustCompile(`^\-?\d+$`)
)

var (
	ErrBadBytesize     = errors.Static("invalid byte size")
	ErrBadBytesizeUnit = errors.Static("invalid byte size unit")
)

func Parse(s string) (int64, error) {
	b := []byte(s)
	if !BytesizeRegexp.Match(b) {
		return 0, errors.Trace(ErrBadBytesize)
	}

	subs := BytesizeRegexp.FindSubmatch(b)
	if len(subs) != 3 {
		return 0, errors.Trace(ErrBadBytesize)
	}

	size := int64(0)
	switch strings.ToUpper(string(subs[2])) {
	case "B", "":
		size = 1
	case "KB", "K":
		size = KB
	case "MB", "M":
		size = MB
	case "GB", "G":
		size = GB
	case "PB", "P":
		size = PB
	default:
		return 0, errors.Trace(ErrBadBytesizeUnit)
	}

	text := string(subs[1])
	fmt.Println(text)
	if digitsRegexp.Match(subs[1]) {
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return 0, errors.Trace(ErrBadBytesize)
		}
		size *= n
	} else {
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, errors.Trace(ErrBadBytesize)
		}
		size = int64(float64(size) * n)
	}
	return size, nil
}
