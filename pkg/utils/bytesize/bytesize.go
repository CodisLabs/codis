// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package bytesize

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

const (
	_ = 1 << (10 * iota)
	KB
	MB
	GB
	TB
	PB
)

type Int64 int64

func (b Int64) Int() int {
	return int(b)
}

func (b Int64) MarshalText() ([]byte, error) {
	if b == 0 {
		return []byte("0"), nil
	}
	var abs = int64(b)
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs%PB == 0:
		return []byte(fmt.Sprintf("%dpb", b/PB)), nil
	case abs%TB == 0:
		return []byte(fmt.Sprintf("%dtb", b/TB)), nil
	case abs%GB == 0:
		return []byte(fmt.Sprintf("%dgb", b/GB)), nil
	case abs%MB == 0:
		return []byte(fmt.Sprintf("%dmb", b/MB)), nil
	case abs%KB == 0:
		return []byte(fmt.Sprintf("%dkb", b/KB)), nil
	default:
		return []byte(fmt.Sprintf("%d", b)), nil
	}
}

func (p *Int64) UnmarshalText(text []byte) error {
	n, err := Parse(string(text))
	if err != nil {
		return err
	}
	*p = Int64(n)
	return nil
}

var (
	fullRegexp = regexp.MustCompile(`^\s*(\-?[\d\.]+)\s*([kmgtp]?b|[bkmgtp]|)\s*$`)
	digitsOnly = regexp.MustCompile(`^\-?\d+$`)
)

var (
	ErrBadByteSize     = errors.New("invalid bytesize")
	ErrBadByteSizeUnit = errors.New("invalid bytesize unit")
)

func Parse(s string) (int64, error) {
	if !fullRegexp.MatchString(s) {
		return 0, errors.Trace(ErrBadByteSize)
	}

	subs := fullRegexp.FindStringSubmatch(s)
	if len(subs) != 3 {
		return 0, errors.Trace(ErrBadByteSize)
	}

	text := subs[1]
	unit := subs[2]

	size := int64(1)
	switch unit {
	case "b", "":
	case "k", "kb":
		size = KB
	case "m", "mb":
		size = MB
	case "g", "gb":
		size = GB
	case "t", "tb":
		size = TB
	case "p", "pb":
		size = PB
	default:
		return 0, errors.Trace(ErrBadByteSizeUnit)
	}

	if digitsOnly.MatchString(text) {
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return 0, errors.Trace(ErrBadByteSize)
		}
		size *= n
	} else {
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, errors.Trace(ErrBadByteSize)
		}
		size = int64(float64(size) * n)
	}
	return size, nil
}

func MustParse(s string) int64 {
	v, err := Parse(s)
	if err != nil {
		log.PanicError(err, "parse bytesize failed")
	}
	return v
}
