package math2

import "time"

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
