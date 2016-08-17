package miniredis

import (
	"fmt"
	"math"
	"strings"

	"github.com/bsm/redeo"
)

const (
	msgWrongType        = "WRONGTYPE Operation against a key holding the wrong kind of value"
	msgInvalidInt       = "ERR value is not an integer or out of range"
	msgInvalidFloat     = "ERR value is not a valid float"
	msgInvalidMinMax    = "ERR min or max is not a float"
	msgInvalidRangeItem = "ERR min or max not valid string range item"
	msgSyntaxError      = "ERR syntax error"
	msgKeyNotFound      = "ERR no such key"
	msgOutOfRange       = "ERR index out of range"
	msgInvalidCursor    = "ERR invalid cursor"
	msgXXandNX          = "ERR XX and NX options at the same time are not compatible"
)

// withTx wraps the non-argument-checking part of command handling code in
// transaction logic.
func withTx(
	m *Miniredis,
	out *redeo.Responder,
	r *redeo.Request,
	cb txCmd,
) error {
	ctx := getCtx(r.Client())
	if inTx(ctx) {
		addTxCmd(ctx, cb)
		out.WriteInlineString("QUEUED")
		return nil
	}
	m.Lock()
	defer m.Unlock()
	cb(out, ctx)
	return nil
}

// formatFloat formats a float the way redis does (sort-of)
func formatFloat(v float64) string {
	// Format with %f and strip trailing 0s. This is the most like Redis does
	// it :(
	// .12 is the magic number where most output is the same as Redis.
	if math.IsInf(v, +1) {
		return "inf"
	}
	if math.IsInf(v, -1) {
		return "-inf"
	}
	sv := fmt.Sprintf("%.12f", v)
	for strings.Contains(sv, ".") {
		if sv[len(sv)-1] != '0' {
			break
		}
		// Remove trailing 0s.
		sv = sv[:len(sv)-1]
		// Ends with a '.'.
		if sv[len(sv)-1] == '.' {
			sv = sv[:len(sv)-1]
			break
		}
	}
	return sv
}

// redisRange gives Go offsets for something l long with start/end in
// Redis semantics. Both start and end can be negative.
// Used for string range and list range things.
// The results can be used as: v[start:end]
// Note that GETRANGE (on a string key) never returns an empty string when end
// is a large negative number.
func redisRange(l, start, end int, stringSymantics bool) (int, int) {
	if start < 0 {
		start = l + start
		if start < 0 {
			start = 0
		}
	}
	if start > l {
		start = l
	}

	if end < 0 {
		end = l + end
		if end < 0 {
			end = -1
			if stringSymantics {
				end = 0
			}
		}
	}
	end++ // end argument is inclusive in Redis.
	if end > l {
		end = l
	}

	if end < start {
		return 0, 0
	}
	return start, end
}

// matchKeys filters only matching keys.
// Will return an empty list on invalid match expression.
func matchKeys(keys []string, match string) []string {
	re := patternRE(match)
	if re == nil {
		// Special case, the given pattern won't match anything / is
		// invalid.
		return nil
	}
	res := []string{}
	for _, k := range keys {
		if !re.MatchString(k) {
			continue
		}
		res = append(res, k)
	}
	return res
}
