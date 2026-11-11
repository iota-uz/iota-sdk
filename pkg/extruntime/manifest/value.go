package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RefValue is the wire form of Ref<Entity>.
type RefValue struct {
	Entity string `json:"entity"`
	Key    string `json:"key"`
}

var (
	int64Pattern   = regexp.MustCompile(`^-?(0|[1-9][0-9]{0,18})$`)
	decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
)

// PathError locates a wire validation failure.
type PathError struct {
	Path    []string
	Message string
}

func (e *PathError) Error() string {
	if len(e.Path) == 0 {
		return e.Message
	}
	return strings.Join(e.Path, ".") + ": " + e.Message
}

// ValidateValue checks raw against t with exact wire rules (§2.1): safe JSON
// integers, int64/decimal as strings, ISO dates, typed refs.
func ValidateValue(t Type, raw json.RawMessage) error {
	return validate(t, raw, nil)
}

func fail(path []string, format string, args ...any) error {
	return &PathError{Path: append([]string(nil), path...), Message: fmt.Sprintf(format, args...)}
}

func isNull(raw json.RawMessage) bool {
	return len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func validate(t Type, raw json.RawMessage, path []string) error {
	if isNull(raw) {
		if t.Nullable {
			return nil
		}
		return fail(path, "value is required")
	}
	switch t.Kind {
	case KindString, KindDate, KindDateTime, KindUUID, KindEnum, KindInt64, KindDecimal:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return fail(path, "expected string for %s", t.Kind)
		}
		return validateString(t, s, path)
	case KindInt:
		var f json.Number
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&f); err != nil {
			return fail(path, "expected integer")
		}
		n, err := strconv.ParseInt(f.String(), 10, 64)
		if err != nil || math.Abs(float64(n)) > 1<<53-1 {
			return fail(path, "expected safe integer")
		}
		return nil
	case KindBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return fail(path, "expected boolean")
		}
		return nil
	case KindJSON:
		if !json.Valid(raw) {
			return fail(path, "invalid json")
		}
		return nil
	case KindRef:
		var r RefValue
		if err := json.Unmarshal(raw, &r); err != nil || r.Key == "" || r.Entity == "" {
			return fail(path, "expected ref {entity,key}")
		}
		if t.Entity != "" && r.Entity != t.Entity {
			return fail(path, "expected ref to %s, got %s", t.Entity, r.Entity)
		}
		return nil
	case KindArray:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return fail(path, "expected array")
		}
		if t.Items == nil {
			return fail(path, "array without item type")
		}
		for i, it := range items {
			if err := validate(*t.Items, it, append(path, strconv.Itoa(i))); err != nil {
				return err
			}
		}
		return nil
	case KindObject:
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return fail(path, "expected object")
		}
		known := make(map[string]bool, len(t.Properties))
		for _, p := range t.Properties {
			known[p.Name] = true
			v, ok := obj[p.Name]
			if !ok {
				if p.Type.Optional {
					continue
				}
				return fail(append(path, p.Name), "value is required")
			}
			if err := validate(p.Type, v, append(path, p.Name)); err != nil {
				return err
			}
		}
		for k := range obj {
			if !known[k] {
				return fail(append(path, k), "unknown property")
			}
		}
		return nil
	default:
		return fail(path, "unsupported type %q", t.Kind)
	}
}

func validateString(t Type, s string, path []string) error {
	switch t.Kind {
	case KindString:
		if t.MaxLength > 0 && len([]rune(s)) > t.MaxLength {
			return fail(path, "longer than %d", t.MaxLength)
		}
	case KindDate:
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return fail(path, "expected date YYYY-MM-DD")
		}
	case KindDateTime:
		if _, err := time.Parse(time.RFC3339Nano, s); err != nil {
			return fail(path, "expected RFC3339 datetime with offset")
		}
	case KindUUID:
		if _, err := uuid.Parse(s); err != nil || len(s) != 36 {
			return fail(path, "expected canonical uuid")
		}
	case KindEnum:
		for _, v := range t.Values {
			if v == s {
				return nil
			}
		}
		return fail(path, "expected one of %s", strings.Join(t.Values, ", "))
	case KindInt64:
		if !int64Pattern.MatchString(s) {
			return fail(path, "expected int64 decimal string")
		}
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			return fail(path, "int64 out of range")
		}
	case KindDecimal:
		if !decimalPattern.MatchString(s) {
			return fail(path, "expected decimal string")
		}
		if t.Scale > 0 {
			if i := strings.IndexByte(s, '.'); i >= 0 && len(s)-i-1 > t.Scale {
				return fail(path, "more than %d fractional digits", t.Scale)
			}
		}
		if t.Precision > 0 {
			digits := strings.TrimLeft(strings.NewReplacer("-", "", ".", "").Replace(s), "0")
			if len(digits) > t.Precision {
				return fail(path, "more than %d digits", t.Precision)
			}
		}
	}
	return nil
}
