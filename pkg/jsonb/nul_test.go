package jsonb_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/jsonb"
)

func TestTextReplacesNUL(t *testing.T) {
	assert.Equal(t, "a\uFFFDb", jsonb.Text("a\x00b"))
	assert.Equal(t, "plain", jsonb.Text("plain"))
}

// A key with a NUL and a key that already has U+FFFD in its place stay two keys: no value is lost, and
// the names are the same on every call.
func TestKeysWithNULNeverMerge(t *testing.T) {
	raw := json.RawMessage(`{"a\u0000": 1, "a\uFFFD": 2, "a\u0000\u0000": 3, "a\uFFFD\u0000": 4, "n": {"k\u0000": "v\u0000"}}`)
	first := jsonb.WithoutNUL(raw)
	var got map[string]any
	require.NoError(t, json.Unmarshal(first, &got))
	assert.Len(t, got, 5, "every key keeps its value")
	assert.Equal(t, json.Number("2"), json.Number(string(mustJSON(t, got["a\uFFFD"]))), "a key without NUL keeps its name")
	values := map[string]bool{}
	for k, v := range got {
		assert.NotContains(t, k, "\x00")
		if k != "n" {
			values[string(mustJSON(t, v))] = true
		}
	}
	assert.Equal(t, map[string]bool{"1": true, "2": true, "3": true, "4": true}, values)
	assert.Equal(t, map[string]any{"k\uFFFD": "v\uFFFD"}, got["n"])
	for range 20 {
		assert.JSONEq(t, string(first), string(jsonb.WithoutNUL(raw)), "the same names every time")
	}
	assert.NotContains(t, string(first), `\u0000`)
}

func TestWithoutNULLeavesOtherValues(t *testing.T) {
	raw := json.RawMessage(`{"a": "\uFFFD", "b": 1.50}`)
	assert.Equal(t, string(raw), string(jsonb.WithoutNUL(raw)))
	assert.Equal(t, "not json \\u0000", string(jsonb.WithoutNUL(json.RawMessage("not json \\u0000"))))
	assert.JSONEq(t, `{"b": 1.50, "s": "x\uFFFD"}`, string(jsonb.WithoutNUL(json.RawMessage(`{"b": 1.50, "s": "x\u0000"}`))))
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
