package listext

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// LabelFor picks the label for lang (the primary subtag is also tried), then
// English, then the alphabetically first label, then fallback.
func LabelFor(label manifest.Label, lang, fallback string) string {
	if len(label) == 0 {
		return fallback
	}
	lang = strings.ToLower(strings.TrimSpace(lang))
	if v := label[lang]; v != "" {
		return v
	}
	if i := strings.IndexAny(lang, "-_"); i > 0 {
		if v := label[lang[:i]]; v != "" {
			return v
		}
	}
	if v := label["en"]; v != "" {
		return v
	}
	keys := make([]string, 0, len(label))
	for k := range label {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if label[k] != "" {
			return label[k]
		}
	}
	return fallback
}

// Value is one cell read from a contributed column. Raw is the canonical wire
// text of the value (decimal and int64 exact, date "YYYY-MM-DD", datetime
// RFC 3339 UTC, bool "true"/"false"); nil is NULL.
type Value struct {
	Type manifest.Type
	Raw  *string
}

// Null reports an absent value.
func (v Value) Null() bool { return v.Raw == nil }

// Text is the canonical text, empty for NULL.
func (v Value) Text() string {
	if v.Raw == nil {
		return ""
	}
	return *v.Raw
}

// Bool returns the value of a bool column.
func (v Value) Bool() (bool, bool) {
	if v.Raw == nil {
		return false, false
	}
	return *v.Raw == "true", true
}

// Time parses date and datetime values.
func (v Value) Time() (time.Time, bool) {
	if v.Raw == nil {
		return time.Time{}, false
	}
	switch v.Type.Kind {
	case manifest.KindDate:
		t, err := time.Parse("2006-01-02", *v.Raw)
		return t, err == nil
	case manifest.KindDateTime:
		t, err := time.Parse(time.RFC3339, *v.Raw)
		return t, err == nil
	case manifest.KindString, manifest.KindInt, manifest.KindInt64, manifest.KindDecimal,
		manifest.KindBool, manifest.KindUUID, manifest.KindEnum, manifest.KindJSON,
		manifest.KindRef, manifest.KindObject, manifest.KindArray:
		// Not a point in time.
	}
	return time.Time{}, false
}

// Export is the value for a spreadsheet cell: int columns as int64, bool as
// bool, everything else as its exact text. NULL is nil.
func (v Value) Export() any {
	if v.Raw == nil {
		return nil
	}
	switch v.Type.Kind {
	case manifest.KindInt:
		if n, err := strconv.ParseInt(*v.Raw, 10, 64); err == nil {
			return n
		}
	case manifest.KindBool:
		return *v.Raw == "true"
	case manifest.KindString, manifest.KindInt64, manifest.KindDecimal, manifest.KindDate,
		manifest.KindDateTime, manifest.KindUUID, manifest.KindEnum, manifest.KindJSON,
		manifest.KindRef, manifest.KindObject, manifest.KindArray:
		// Exported as the exact text.
	}
	return *v.Raw
}

// Cell pairs a column with the value of one row.
type Cell struct {
	Column Column
	Value  Value
}

// condition turns a user-supplied filter into a typed condition. A value that
// does not parse as the column's type is an error (the entry is ignored).
func condition(t manifest.Type, f Filter) (manifest.Condition, error) {
	var c manifest.Condition
	value := f.Value
	if t.Kind == manifest.KindDateTime && len(strings.TrimSpace(value)) == len("2006-01-02") {
		// A date picked for a datetime column bounds the whole day (UTC).
		value = strings.TrimSpace(value) + "T00:00:00Z"
		if f.Op == OpLte {
			value = strings.TrimSpace(f.Value) + "T23:59:59Z"
		}
	}
	lit, err := literal(t, value)
	if err != nil {
		return c, err
	}
	switch f.Op {
	case OpEq:
		c.Eq = lit
	case OpGte:
		c.Gte = lit
	case OpLte:
		c.Lte = lit
	case OpContains:
		if t.Kind != manifest.KindString {
			c.Eq = lit
			return c, nil
		}
		pattern := "%" + escapeLike(f.Value) + "%"
		c.Like = &pattern
	default:
		return c, fmt.Errorf("unknown operator %q", f.Op)
	}
	return c, nil
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func literal(t manifest.Type, raw string) (json.RawMessage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty value")
	}
	switch t.Kind {
	case manifest.KindInt:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", raw)
		}
		return json.Marshal(n)
	case manifest.KindBool:
		switch strings.ToLower(raw) {
		case "true", "1", "yes", "on":
			return json.RawMessage("true"), nil
		case "false", "0", "no", "off":
			return json.RawMessage("false"), nil
		}
		return nil, fmt.Errorf("%q is not a boolean", raw)
	case manifest.KindEnum:
		for _, v := range t.Values {
			if v == raw {
				return json.Marshal(raw)
			}
		}
		return nil, fmt.Errorf("%q is not one of the enum values", raw)
	case manifest.KindString, manifest.KindInt64, manifest.KindDecimal, manifest.KindDate, manifest.KindDateTime, manifest.KindUUID:
		return json.Marshal(raw)
	case manifest.KindJSON, manifest.KindRef, manifest.KindObject, manifest.KindArray:
		// Not filterable from list controls; reported below.
	}
	return nil, fmt.Errorf("%s columns cannot be filtered", t.Kind)
}

// Query parameter names of the standard list controls.
const (
	ParamPrefix = "ext."
	ParamFrom   = "extfrom."
	ParamTo     = "extto."
	ParamSort   = "ext.sort"
	ParamDir    = "ext.dir"
)

// ParseQuery reads the standard filter and sort parameters:
//
//	ext.<columnID>=v        equality (substring match for text columns)
//	extfrom.<columnID>=v    lower bound (dates, numbers)
//	extto.<columnID>=v      upper bound
//	ext.sort=<columnID>&ext.dir=asc|desc
//
// Blank values are skipped. Column IDs are validated at compile time.
func ParseQuery(values url.Values) Request {
	var r Request
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := strings.TrimSpace(values.Get(k))
		if v == "" {
			continue
		}
		switch {
		case k == ParamSort || k == ParamDir:
		case strings.HasPrefix(k, ParamFrom):
			r.Filters = append(r.Filters, Filter{Column: strings.TrimPrefix(k, ParamFrom), Op: OpGte, Value: v})
		case strings.HasPrefix(k, ParamTo):
			r.Filters = append(r.Filters, Filter{Column: strings.TrimPrefix(k, ParamTo), Op: OpLte, Value: v})
		case strings.HasPrefix(k, ParamPrefix):
			r.Filters = append(r.Filters, Filter{Column: strings.TrimPrefix(k, ParamPrefix), Op: OpContains, Value: v})
		}
	}
	if col := strings.TrimSpace(values.Get(ParamSort)); col != "" {
		r.Sort = &Sort{Column: col, Desc: strings.EqualFold(values.Get(ParamDir), "desc")}
	}
	return r
}
