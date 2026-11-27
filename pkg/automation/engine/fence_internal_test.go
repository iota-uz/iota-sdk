package engine

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

type plainCall struct{}

func (plainCall) Spec() graph.NodeSpec              { return graph.NodeSpec{Type: "t.plain"} }
func (plainCall) Requires() []catalog.ConnectorKind { return nil }
func (plainCall) Idempotent() bool                  { return true }
func (plainCall) Execute(context.Context, catalog.CallInput) (catalog.CallResult, error) {
	return catalog.CallResult{}, nil
}

// decisionCall keys its decision by the text param only, as a model's prompt.
type decisionCall struct{ plainCall }

func (decisionCall) DecisionKey(in catalog.CallInput) (string, bool) {
	text, ok := in.Params["text"].(string)
	if !ok {
		return "", false
	}
	return DecisionKey(text, nil, "m", nil)
}

// requestCall names the request it sends: the "body" param.
type requestCall struct{ decisionCall }

func (requestCall) RequestKey(in catalog.CallInput) (string, bool) {
	b, ok := in.Params["body"].(string)
	return b, ok
}

func digestOf(t *testing.T, cn catalog.CallNode, params, stepCtx map[string]any) string {
	t.Helper()
	d, ok := requestDigest(cn, catalog.CallInput{Params: params, Context: stepCtx})
	require.True(t, ok)
	return d
}

// The digest of a fenced call follows the request the node sends: the same request has the same digest
// whatever else of the step changed (a context the node ignores, map order), a changed request another.
func TestRequestDigestFollowsTheRequestSent(t *testing.T) {
	// A decision: its cache key, not the step context, is the request.
	dec := decisionCall{}
	a := digestOf(t, dec, map[string]any{"text": "hello"}, map[string]any{"nodes": map[string]any{"set": map[string]any{"tag": "a"}}})
	assert.Equal(t, a, digestOf(t, dec, map[string]any{"text": "hello"}, map[string]any{"nodes": map[string]any{"set": map[string]any{"tag": "b"}}}),
		"an upstream output the decision does not read does not change its key")
	assert.NotEqual(t, a, digestOf(t, dec, map[string]any{"text": "hello, again"}, nil), "a changed prompt is another request")

	// A node that names its request: only that counts.
	req := requestCall{}
	r := digestOf(t, req, map[string]any{"body": "x", "text": "1"}, nil)
	assert.Equal(t, r, digestOf(t, req, map[string]any{"body": "x", "text": "2"}, map[string]any{"vars": 1}))
	assert.NotEqual(t, r, digestOf(t, req, map[string]any{"body": "y", "text": "1"}, nil))

	// Any other call: its params and step context, with sorted keys.
	p := digestOf(t, plainCall{}, map[string]any{"text": "hello", "n": 1}, nil)
	assert.Equal(t, p, digestOf(t, plainCall{}, map[string]any{"n": 1, "text": "hello"}, nil), "map order does not change the key")
	assert.NotEqual(t, p, digestOf(t, plainCall{}, map[string]any{"text": "hello", "n": 2}, nil))
	assert.NotEqual(t, p, digestOf(t, plainCall{}, map[string]any{"text": "hello", "n": 1}, map[string]any{"vars": 1}))

	assert.Equal(t, "backtest:b:e:d1@each[1]#2:"+p, fencedKey("backtest:b:e", "d1", graph.CallSite{Token: "each[1]", Visit: 2}, p))
}

// A request with no stable form (a value JSON cannot encode, a decision its node cannot key) has no
// digest: a strict fence refuses the call rather than send a key no repeat would match.
func TestRequestDigestWithoutStableForm(t *testing.T) {
	_, ok := requestDigest(plainCall{}, catalog.CallInput{Params: map[string]any{"x": math.Inf(1)}})
	assert.False(t, ok)
	_, ok = requestDigest(decisionCall{}, catalog.CallInput{Params: map[string]any{"text": 1}})
	assert.False(t, ok)
	_, ok = requestDigest(requestCall{}, catalog.CallInput{Params: map[string]any{"text": "x"}})
	assert.False(t, ok, "the node's own key wins over the decision's")
}
