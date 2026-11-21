package filterq

import (
	"slices"
	"strconv"
	"time"
)

// In-memory counterparts of AppendSQL, for lists assembled in Go. They follow
// the same semantics, so a field filters a SQL-backed list and an in-memory
// list identically.

// MatchValue reports whether a reference or bool value satisfies the
// condition. An empty string is "no value": it matches OpEmpty and OpIsNot.
func (c Condition) MatchValue(v string) bool {
	switch c.Op {
	case OpEmpty:
		return v == ""
	case OpIs:
		return v != "" && slices.Contains(c.Values, v)
	case OpIsNot:
		return v == "" || !slices.Contains(c.Values, v)
	case OpBefore, OpAfter, OpOn, OpBetween, OpEq, OpGt, OpLt:
		// Date and number operators never apply to a reference value.
	}
	return false
}

// MatchValues is MatchValue for a record carrying several values (tags, a
// project with several stages): "is" needs any of them listed, "is not" none.
func (c Condition) MatchValues(vs []string) bool {
	switch c.Op {
	case OpEmpty:
		return len(vs) == 0
	case OpIs:
		return slices.ContainsFunc(vs, func(v string) bool { return slices.Contains(c.Values, v) })
	case OpIsNot:
		return !slices.ContainsFunc(vs, func(v string) bool { return slices.Contains(c.Values, v) })
	case OpBefore, OpAfter, OpOn, OpBetween, OpEq, OpGt, OpLt:
		// Date and number operators never apply to reference values.
	}
	return false
}

// MatchBool reports whether a flag satisfies an "is true/false" condition.
func (c Condition) MatchBool(v bool) bool {
	if c.Op != OpIs || len(c.Values) != 1 {
		return false
	}
	return (c.Values[0] == "true") == v
}

// MatchDate reports whether a date satisfies the condition, comparing whole
// days in now's location. A zero time is "no value".
func (c Condition) MatchDate(t time.Time, now time.Time) bool {
	if t.IsZero() {
		return c.Op == OpEmpty
	}
	loc := now.Location()
	y, m, d := t.In(loc).Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, loc)
	if c.Op == OpBetween {
		from, to, ok := c.DateRange(now)
		return ok && !day.Before(from) && !day.After(to)
	}
	if len(c.Values) != 1 {
		return false
	}
	v, err := time.ParseInLocation(DateLayout, c.Values[0], loc)
	if err != nil {
		return false
	}
	switch c.Op {
	case OpOn:
		return day.Equal(v)
	case OpBefore:
		return day.Before(v)
	case OpAfter:
		return day.After(v)
	case OpIs, OpIsNot, OpBetween, OpEq, OpGt, OpLt, OpEmpty:
		// Between and empty are handled above; the rest are not date operators.
	}
	return false
}

// MatchNumber reports whether a number satisfies the condition. Between is
// inclusive and accepts its bounds in either order.
func (c Condition) MatchNumber(v float64) bool {
	nums := make([]float64, 0, len(c.Values))
	for _, s := range c.Values {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return false
		}
		nums = append(nums, n)
	}
	switch c.Op {
	case OpBetween:
		if len(nums) != 2 {
			return false
		}
		lo, hi := min(nums[0], nums[1]), max(nums[0], nums[1])
		return v >= lo && v <= hi
	case OpEq:
		return len(nums) == 1 && v == nums[0]
	case OpGt:
		return len(nums) == 1 && v > nums[0]
	case OpLt:
		return len(nums) == 1 && v < nums[0]
	case OpIs, OpIsNot, OpBefore, OpAfter, OpOn, OpEmpty:
		// Not number operators.
	}
	return false
}
