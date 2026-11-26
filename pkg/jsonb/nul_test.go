package jsonb_test

import (
	"encoding/json"
	"strings"
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

// Input that is not one JSON value is left as it is, NUL and all: a trailing value is never dropped.
func TestWithoutNULLeavesMalformedInput(t *testing.T) {
	for _, raw := range []string{`{"s": "\u0000"} {"other": 1}`, `{"s": "\u0000"`, `"\u0000" 1`} {
		assert.Equal(t, raw, string(jsonb.WithoutNUL(json.RawMessage(raw))))
	}
}

// Many keys that become one text take short numbered names, each value kept: the cost grows with the
// keys, not with their square.
func TestManyCollidingKeysStayShort(t *testing.T) {
	const n = 4096
	in := make(map[string]any, n+1)
	in["k\uFFFD"] = "plain"
	for i := 0; i < n; i++ {
		var b strings.Builder
		b.WriteString("k")
		for bit := 0; bit < 12; bit++ {
			if i&(1<<bit) != 0 {
				b.WriteString("\x00")
			} else {
				b.WriteString("\uFFFD")
			}
		}
		in[b.String()] = i
	}
	out, ok := jsonb.ReplaceNUL(in).(map[string]any)
	require.True(t, ok)
	assert.Len(t, out, n+1, "every key keeps its value")
	assert.Equal(t, "plain", out["k\uFFFD"], "a key without NUL keeps its name")
	seen := make([]bool, n)
	for k, v := range out {
		if k == "k\uFFFD" {
			continue
		}
		i, ok := v.(int)
		require.True(t, ok, "%q holds %v", k, v)
		require.False(t, seen[i], "value %d appears twice", i)
		seen[i] = true
	}
	for i, ok := range seen {
		assert.True(t, ok, "value %d is kept", i)
	}
	for k := range out {
		assert.NotContains(t, k, "\x00")
		assert.LessOrEqual(t, len(k), len("k")+12*len("\uFFFD")+len("\uFFFD")+4, "a key grows by a few bytes at most: %q", k)
	}
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
