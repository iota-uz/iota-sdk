package catalog

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// WithoutNUL is a JSON value with every U+0000 of its strings and keys replaced by U+FFFD. PostgreSQL's
// jsonb (a workflow's journal, a host's invocation record, a run's steps and context) refuses \u0000, so
// a value that carries one (a response body, an action's output with a NUL byte) would not be recorded
// and the call would end as if it had failed. A value that is not JSON is returned as it is.
func WithoutNUL(raw json.RawMessage) json.RawMessage {
	if !bytes.Contains(raw, []byte(`\u0000`)) {
		return raw
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return raw
	}
	out, err := json.Marshal(ReplaceNUL(v))
	if err != nil {
		return raw
	}
	return out
}

// ReplaceNUL replaces U+0000 with U+FFFD in the strings and keys of a decoded JSON value. Keys are
// visited in order, so when two keys become one the value of the key that sorts first is kept, every time.
func ReplaceNUL(v any) any {
	switch x := v.(type) {
	case string:
		return strings.ReplaceAll(x, "\x00", "\uFFFD")
	case []any:
		for i := range x {
			x[i] = ReplaceNUL(x[i])
		}
		return x
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(x))
		for _, k := range keys {
			nk := strings.ReplaceAll(k, "\x00", "\uFFFD")
			if _, taken := out[nk]; taken {
				continue
			}
			out[nk] = ReplaceNUL(x[k])
		}
		return out
	}
	return v
}
