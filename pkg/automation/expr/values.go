package expr

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// MaxSafeInt is the largest integer a JS client reads exactly (2^53 - 1).
const MaxSafeInt = 1<<53 - 1

// InputError is a value that does not fit its declared field: a string in an int field, an int outside the
// JS-safe range.
type InputError struct {
	Path    string
	Message string
}

func (e *InputError) Error() string { return e.Path + ": " + e.Message }

// segment is one step of a value path: a key (string) or a list index (int64).
type segment = any

type missing struct{ path []segment }

// normalizer turns a context value into CEL-ready Go values by its field and records the required fields
// that are absent, so they become CEL unknowns.
type normalizer struct {
	missing []missing
}

func pathString(path []segment) string {
	var b strings.Builder
	for i, s := range path {
		switch v := s.(type) {
		case string:
			if i > 0 {
				b.WriteByte('.')
			}
			b.WriteString(v)
		case int64:
			fmt.Fprintf(&b, "[%d]", v)
		}
	}
	return b.String()
}

func appendPath(path []segment, s segment) []segment {
	out := make([]segment, len(path), len(path)+1)
	copy(out, path)
	return append(out, s)
}

// value normalizes v for field f. present=false: the value is absent and recorded as missing.
func (n *normalizer) value(f Field, v any, path []segment) (any, bool, error) {
	if v == nil && f.Kind != Optional {
		n.missing = append(n.missing, missing{path: path})
		return nil, false, nil
	}
	bad := func(want string) (any, bool, error) {
		return nil, false, &InputError{Path: pathString(path), Message: fmt.Sprintf("%s expected, got %T", want, v)}
	}
	switch f.Kind {
	case Optional:
		if v == nil {
			return zero(*f.Elem), true, nil
		}
		return n.value(*f.Elem, v, path)
	case Dyn:
		d, err := dynValue(v, path)
		if m, ok := d.(map[string]any); ok && err == nil {
			return newDynMap(m, path, true), true, nil
		}
		return d, true, err
	case String, Enum:
		s, ok := v.(string)
		if !ok {
			return bad("string")
		}
		return s, true, nil
	case Bool:
		b, ok := v.(bool)
		if !ok {
			return bad("bool")
		}
		return b, true, nil
	case Int:
		i, ok, err := intValue(v)
		if err != nil {
			return nil, false, &InputError{Path: pathString(path), Message: err.Error()}
		}
		if !ok {
			return bad("int")
		}
		if i > MaxSafeInt || i < -MaxSafeInt {
			return nil, false, &InputError{Path: pathString(path), Message: fmt.Sprintf("%d is outside the JS-safe integer range", i)}
		}
		return i, true, nil
	case Double:
		d, ok := floatValue(v)
		if !ok {
			return bad("number")
		}
		return d, true, nil
	case Timestamp:
		switch t := v.(type) {
		case time.Time:
			return t.UTC(), true, nil
		case string:
			parsed, err := time.Parse(time.RFC3339Nano, t)
			if err != nil {
				return nil, false, &InputError{Path: pathString(path), Message: "an RFC 3339 timestamp expected"}
			}
			return parsed.UTC(), true, nil
		}
		return bad("timestamp")
	case List:
		list, ok := toList(v)
		if !ok {
			return bad("list")
		}
		out := make([]any, 0, len(list))
		for i, x := range list {
			if x == nil && f.Elem.Kind == Dyn {
				out = append(out, nil) // null is a dyn value
				continue
			}
			if x == nil && f.Elem.Kind != Optional {
				return nil, false, &InputError{Path: pathString(appendPath(path, int64(i))), Message: "null in a list"}
			}
			nv, present, err := n.value(*f.Elem, x, appendPath(path, int64(i)))
			if err != nil {
				return nil, false, err
			}
			if present {
				out = append(out, nv)
			} else {
				out = append(out, nil)
			}
		}
		return out, true, nil
	case Map:
		m, ok := v.(map[string]any)
		if !ok {
			return bad("map")
		}
		out := make(map[string]any, len(m))
		for k, x := range m {
			if x == nil {
				continue // a null value of a map is no key
			}
			nv, present, err := n.value(*f.Elem, x, appendPath(path, k))
			if err != nil {
				return nil, false, err
			}
			if present {
				out[k] = nv
			}
		}
		return newDynMap(out, path, false), true, nil
	case Object:
		m, ok := v.(map[string]any)
		if !ok {
			return bad("object")
		}
		out := make(map[string]any, len(f.Props))
		for _, p := range f.Props {
			nv, present, err := n.value(p.Field, m[p.Name], appendPath(path, p.Name))
			if err != nil {
				return nil, false, err
			}
			if present {
				out[p.Name] = nv
			}
		}
		return out, true, nil
	}
	return nil, false, &InputError{Path: pathString(path), Message: fmt.Sprintf("unknown field kind %d", int(f.Kind))}
}

// zero is the value an absent optional field reads as.
func zero(f Field) any {
	switch f.Kind {
	case String, Enum:
		return ""
	case Int:
		return int64(0)
	case Double:
		return float64(0)
	case Bool:
		return false
	case Timestamp:
		return time.Unix(0, 0).UTC()
	case List:
		return []any{}
	case Map:
		return map[string]any{}
	case Optional:
		return zero(*f.Elem)
	case Object:
		out := make(map[string]any, len(f.Props))
		for _, p := range f.Props {
			out[p.Name] = zero(p.Field)
		}
		return out
	case Dyn:
		return nil
	}
	return nil
}

// dynValue is a JSON value as CEL reads dyn: integers as int64, null keys of objects dropped.
func dynValue(v any, path []segment) (any, error) {
	switch x := v.(type) {
	case nil, string, bool, time.Time:
		return x, nil
	case json.Number, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		if i, ok, _ := intValue(x); ok {
			return i, nil
		}
		f, _ := floatValue(x)
		return f, nil
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			d, err := dynValue(e, appendPath(path, int64(i)))
			if err != nil {
				return nil, err
			}
			out[i] = d
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			if e == nil {
				continue
			}
			d, err := dynValue(e, appendPath(path, k))
			if err != nil {
				return nil, err
			}
			out[k] = d
		}
		return out, nil
	}
	// Other Go values (structs, typed slices) are read through their JSON form.
	b, err := json.Marshal(v)
	if err != nil {
		return nil, &InputError{Path: pathString(path), Message: err.Error()}
	}
	var decoded any
	if err := UnmarshalJSON(b, &decoded); err != nil {
		return nil, &InputError{Path: pathString(path), Message: err.Error()}
	}
	return dynValue(decoded, path)
}

func toList(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, true
	case []map[string]any:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}

// intValue reads an integer: an integral number. A fractional number is ok=false.
func intValue(v any) (int64, bool, error) {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i, true, nil
		}
		f, err := x.Float64()
		if err != nil {
			return 0, false, fmt.Errorf("%s is not a number", x)
		}
		return floatInt(f)
	case float64:
		return floatInt(x)
	case float32:
		return floatInt(float64(x))
	case int:
		return int64(x), true, nil
	case int8:
		return int64(x), true, nil
	case int16:
		return int64(x), true, nil
	case int32:
		return int64(x), true, nil
	case int64:
		return x, true, nil
	case uint:
		return uintInt(uint64(x))
	case uint8:
		return int64(x), true, nil
	case uint16:
		return int64(x), true, nil
	case uint32:
		return int64(x), true, nil
	case uint64:
		return uintInt(x)
	}
	return 0, false, nil
}

func uintInt(u uint64) (int64, bool, error) {
	if u > math.MaxInt64 {
		return 0, false, fmt.Errorf("%d does not fit an int", u)
	}
	return int64(u), true, nil
}

func floatInt(f float64) (int64, bool, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return 0, false, nil
	}
	if f >= math.MaxInt64 || f < math.MinInt64 {
		return 0, false, fmt.Errorf("%v does not fit an int", f)
	}
	return int64(f), true, nil
}

func floatValue(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case float64:
		return x, true
	case float32:
		return float64(x), true
	}
	if i, ok, err := intValue(v); ok && err == nil {
		return float64(i), true
	}
	return 0, false
}

// Number reads a JSON number of any Go representation as float64.
func Number(v any) (float64, bool) { return floatValue(v) }

// FormatDouble writes a double the way JavaScript's Number#toString does: 0.75, 2, 1e+21, 1.5e-7.
func FormatDouble(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}
	// Shortest round-trip digits and the decimal exponent: d.ddd × 10^e.
	s := strconv.FormatFloat(f, 'e', -1, 64)
	mant, expPart, _ := strings.Cut(s, "e")
	digits := strings.Replace(mant, ".", "", 1)
	e, _ := strconv.Atoi(expPart)
	k := len(digits)
	n := e + 1 // the position of the decimal point, as in ECMAScript's algorithm
	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}
	exp := n - 1
	expSign := "+"
	if exp < 0 {
		expSign = "-"
		exp = -exp
	}
	if k == 1 {
		return sign + digits + "e" + expSign + strconv.Itoa(exp)
	}
	return sign + digits[:1] + "." + digits[1:] + "e" + expSign + strconv.Itoa(exp)
}

// UnmarshalJSON decodes JSON with numbers as json.Number, as the format reads every value.
func UnmarshalJSON(data []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("unexpected data after the JSON value")
	}
	return nil
}
