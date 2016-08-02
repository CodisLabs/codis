// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package math2

import (
	"fmt"
	"time"
)

func MaxInt(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func MinInt(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func MinMaxInt(v, min, max int) int {
	if min <= max {
		v = MaxInt(v, min)
		v = MinInt(v, max)
		return v
	}
	panic(fmt.Sprintf("min = %d, max = %d", min, max))
}

func MaxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	} else {
		return b
	}
}

func MinDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	} else {
		return b
	}
}

func MinMaxDuration(v, min, max time.Duration) time.Duration {
	if min <= max {
		v = MaxDuration(v, min)
		v = MinDuration(v, max)
		return v
	}
	panic(fmt.Sprintf("min = %s, max = %s", min, max))
}
