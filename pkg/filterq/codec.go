package filterq

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ParamName is the repeated query parameter carrying one encoded condition each:
//
//	?f=agency:is:<uuid1>,<uuid2>&f=policy_end_at:between:preset:next_30d
//
// Conditions are AND-ed; values within one condition are OR-ed.
const ParamName = "f"

// PresenceParam marks that the filter builder has been touched. It lets a
// consumer distinguish "fresh page, apply default filters" (no f and no fb)
// from "user explicitly cleared everything" (fb present, no f). The builder
// component always submits fb=1.
const PresenceParam = "fb"

// Wire grammar: field ":" op ":" value ("," value)*
// Only the first two colons split, so values may contain colons freely
// (e.g. preset:this_year). Inside a value, "%" → "%25" and "," → "%2C";
// nothing else is escaped — UUIDs, codes, dates and numbers stay readable.

func escapeValue(v string) string {
	v = strings.ReplaceAll(v, "%", "%25")
	return strings.ReplaceAll(v, ",", "%2C")
}

func unescapeValue(v string) string {
	v = strings.ReplaceAll(v, "%2C", ",")
	return strings.ReplaceAll(v, "%25", "%")
}

// EncodeCondition renders a single condition in wire form.
func EncodeCondition(c Condition) string {
	vals := make([]string, 0, len(c.Values))
	for _, v := range c.Values {
		vals = append(vals, escapeValue(v))
	}
	return c.Field + ":" + string(c.Op) + ":" + strings.Join(vals, ",")
}

// Encode renders the set as url.Values holding one ParamName entry per
// condition, in order. The result is deterministic for a given set.
func Encode(fs FilterSet) url.Values {
	q := url.Values{}
	EncodeTo(fs, q)
	return q
}

// EncodeTo appends the encoded conditions to an existing url.Values.
func EncodeTo(fs FilterSet, into url.Values) {
	for _, c := range fs {
		into.Add(ParamName, EncodeCondition(c))
	}
}

// DecodeCondition parses wire form into a Condition. It validates syntax only
// (shape and operator token), not field existence or value types — that is
// Decode's job, which has the schema.
func DecodeCondition(raw string) (Condition, bool) {
	parts := strings.SplitN(raw, ":", 3)
	if len(parts) != 3 || parts[0] == "" {
		return Condition{}, false
	}
	op := Operator(parts[1])
	if !op.Valid() {
		return Condition{}, false
	}
	rawValues := strings.Split(parts[2], ",")
	values := make([]string, 0, len(rawValues))
	for _, v := range rawValues {
		if v == "" {
			continue
		}
		values = append(values, unescapeValue(v))
	}
	if len(values) == 0 {
		return Condition{}, false
	}
	return Condition{Field: parts[0], Op: op, Values: values}, true
}

// HasPresence reports whether the builder presence marker is set.
func HasPresence(q url.Values) bool { return q.Get(PresenceParam) != "" }

// Decode parses every ParamName entry, dropping anything invalid (unknown
// field, operator not allowed on the field, arity mismatch, malformed value
// for the field type). Duplicate set conditions (same field + is/isnot) merge
// their values preserving first appearance order; for any other operator the
// first condition wins and later duplicates are dropped.
func Decode(q url.Values, s Schema) FilterSet {
	var out FilterSet
	idx := map[string]int{} // field+op → position in out, for merging/dedup
	for _, raw := range q[ParamName] {
		c, ok := DecodeCondition(raw)
		if !ok {
			continue
		}
		field, ok := s.Field(c.Field)
		if !ok || !field.AllowsOp(c.Op) || !validCondition(field, c) {
			continue
		}
		key := c.Field + "\x00" + string(c.Op)
		if at, seen := idx[key]; seen {
			if c.Op == OpIs || c.Op == OpIsNot {
				out[at].Values = mergeValues(out[at].Values, c.Values)
			}
			continue
		}
		idx[key] = len(out)
		out = append(out, c)
	}
	return out
}

func mergeValues(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, v := range append(append([]string{}, a...), b...) {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// validCondition checks arity and value syntax against the field type.
func validCondition(f Field, c Condition) bool {
	if f.Type == FieldTypeDate && c.Op == OpBetween {
		if _, isPreset := c.Preset(); isPreset {
			return true
		}
	}
	switch arity := c.Op.Arity(); arity {
	case -1:
		if len(c.Values) == 0 {
			return false
		}
	default:
		if len(c.Values) != arity {
			return false
		}
	}
	for _, v := range c.Values {
		if !validValue(f.Type, v) {
			return false
		}
	}
	return true
}

func validValue(t FieldType, v string) bool {
	switch t {
	case FieldTypeDate:
		_, err := time.Parse(DateLayout, v)
		return err == nil
	case FieldTypeNumber:
		_, err := strconv.ParseFloat(v, 64)
		return err == nil
	case FieldTypeBool:
		return v == "true" || v == "false"
	case FieldTypeReference:
		return v != ""
	}
	return false
}
