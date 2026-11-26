package catalog

import (
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/jsonb"
)

// WithoutNUL is a JSON value with every U+0000 of its strings and keys replaced by U+FFFD (jsonb.WithoutNUL).
// PostgreSQL's jsonb (a workflow's journal, a host's invocation record, a run's steps and context) refuses
// \u0000, so a value that carries one (a response body, an action's output with a NUL byte) would not be
// recorded and the call would end as if it had failed. No two keys of an object become one. A value that
// is not JSON is returned as it is.
func WithoutNUL(raw json.RawMessage) json.RawMessage { return jsonb.WithoutNUL(raw) }

// ReplaceNUL replaces U+0000 with U+FFFD in the strings and keys of a decoded JSON value; a key with
// U+0000 never takes the place of another key (jsonb.ReplaceNUL).
func ReplaceNUL(v any) any { return jsonb.ReplaceNUL(v) }

// TextWithoutNUL is a text without U+0000, which jsonb and text columns refuse.
func TextWithoutNUL(s string) string { return jsonb.Text(s) }
