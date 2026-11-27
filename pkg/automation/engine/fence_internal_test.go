package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A fenced call's key is the same for the same request whoever renders it, and another for a request
// rendered differently (the memory changed between two holders), so a deduplicating provider never
// answers one request with another's result.
func TestFencedKeyFollowsTheRenderedRequest(t *testing.T) {
	params := map[string]any{"text": "hello", "n": 1}
	same := map[string]any{"n": 1, "text": "hello"}
	a := fencedKey("backtest:b:e", "d1", 1, params, nil)
	assert.Equal(t, a, fencedKey("backtest:b:e", "d1", 1, same, nil), "map order does not change the key")
	assert.True(t, strings.HasPrefix(a, "backtest:b:e:d1:1:"))
	assert.NotEqual(t, a, fencedKey("backtest:b:e", "d1", 1, map[string]any{"text": "hello, again", "n": 1}, nil), "a different request")
	assert.NotEqual(t, a, fencedKey("backtest:b:e", "d1", 1, params, map[string]any{"vars": 1}), "a different step context")
	assert.NotEqual(t, a, fencedKey("backtest:b:e", "d1", 2, params, nil), "the next call of the node")
	bad := map[string]any{"f": func() {}}
	first, second := fencedKey("s", "d1", 1, bad, nil), fencedKey("s", "d1", 1, bad, nil)
	assert.NotEqual(t, first, second, "a request with no JSON form is never deduplicated")
}
