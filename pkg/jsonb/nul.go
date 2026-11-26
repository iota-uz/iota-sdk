// Package jsonb makes values PostgreSQL records: jsonb and text refuse U+0000, so a value that carries
// one (a response body, a provider's message with a NUL byte) would not be written and its write would
// fail. Every U+0000 becomes U+FFFD, and no two keys of an object become one.
package jsonb

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// Replacement is what a U+0000 becomes.
const Replacement = "\uFFFD"

// Text is s without U+0000.
func Text(s string) string {
	if !strings.Contains(s, "\x00") {
		return s
	}
	return strings.ReplaceAll(s, "\x00", Replacement)
}

// WithoutNUL is a JSON value without U+0000 in its strings and keys (ReplaceNUL). Input that is not one
// JSON value (malformed, or a value followed by more) is returned as it is.
func WithoutNUL(raw json.RawMessage) json.RawMessage {
	if !bytes.Contains(raw, []byte(`\u0000`)) || !json.Valid(raw) {
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

// ReplaceNUL replaces U+0000 with U+FFFD in the strings and keys of a decoded JSON value. A key without
// U+0000 keeps its name; a key with one never takes the place of another key: it becomes its text with
// U+FFFD, or, when that is taken, the text followed by U+FFFD and the first free number (2, 3, ...; keys
// in sorted order, so the names are the same every time). No value of the object is lost, and a key grows
// by a few bytes at most however many keys collide.
func ReplaceNUL(v any) any {
	switch x := v.(type) {
	case string:
		return Text(x)
	case []any:
		for i := range x {
			x[i] = ReplaceNUL(x[i])
		}
		return x
	case map[string]any:
		out := make(map[string]any, len(x))
		var withNUL []string
		for k, val := range x {
			if strings.Contains(k, "\x00") {
				withNUL = append(withNUL, k)
				continue
			}
			out[k] = ReplaceNUL(val)
		}
		sort.Strings(withNUL)
		next := map[string]int{} // the next number to try for a base taken already
		for _, k := range withNUL {
			base := Text(k)
			nk := base
			if _, taken := out[nk]; taken {
				n := max(next[base], 2)
				for {
					nk = base + Replacement + strconv.Itoa(n)
					n++
					if _, taken := out[nk]; !taken {
						break
					}
				}
				next[base] = n
			}
			out[nk] = ReplaceNUL(x[k])
		}
		return out
	}
	return v
}
