package jev_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aichat"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aitest"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/jev"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/jevconfig"
)

const choicesJSON = `[
  {"value":"refund","description":"The customer asks for money back."},
  {"value":"exchange","description":"The customer wants another item."},
  {"value":"other","description":"Anything else."}
]`

func choices(t *testing.T) []any {
	t.Helper()
	var c []any
	require.NoError(t, json.Unmarshal([]byte(choicesJSON), &c))
	return c
}

func newNode(t *testing.T, opts ...jev.Option) (catalog.CallNode, connectors.Connector) {
	t.Helper()
	c := jev.New(opts...)
	require.Len(t, c.Nodes(), 1)
	n, ok := c.Nodes()[0].(catalog.CallNode)
	require.True(t, ok)
	return n, c
}

func parsed(t *testing.T, n catalog.CallNode, raw map[string]any) map[string]any {
	t.Helper()
	p, issues := n.Spec().Params.Parse(raw)
	require.Empty(t, issues)
	return p
}

// base is the params of a call as the engine passes them: parsed, the input evaluated.
func base(t *testing.T, n catalog.CallNode, extra map[string]any) map[string]any {
	t.Helper()
	raw := map[string]any{"input": "{'x': 1}", "choices": choices(t)}
	for k, v := range extra {
		raw[k] = v
	}
	p := parsed(t, n, raw)
	p["input"] = map[string]any{"title": "Wrong size", "labels": []any{"shoes", "urgent"}, "count": json.Number("2"), "urgent": true, "meta": map[string]any{"b": "2", "a": "1"}}
	return p
}

func callErr(t *testing.T, err error) *catalog.CallError {
	t.Helper()
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce, "want a CallError")
	return ce
}

// decision is the answer of a chat model (the openai provider).
func decision(choice string, probs map[string]float64) string {
	b, err := json.Marshal(map[string]any{"choice": choice, "probabilities": probs})
	if err != nil {
		panic(err)
	}
	return aitest.Chat(string(b), 120, 18)
}

// typesafe is the answer of OpenRouter's Decisions API; confidence is the provider's own (nil: left out).
func typesafe(choice string, probs map[string]float64, confidence *float64) string {
	answer := map[string]any{"type": "choice", "choice": choice, "probabilities": probs}
	if confidence != nil {
		answer["confidence"] = *confidence
	}
	b, err := json.Marshal(map[string]any{
		"id": "gen-dec-1", "model": "typesafe/jev-1.14-20261014", "provider": "TypeSafe",
		"answers": map[string]any{"decision": answer},
		"usage":   map[string]any{"input_tokens": 120, "output_tokens": 18, "cost": 0.0042},
	})
	if err != nil {
		panic(err)
	}
	return string(b)
}

func f(v float64) *float64 { return &v }

func creds(provider string) map[string]string { return map[string]string{"provider": provider} }

func TestNodeSpecIsACatalogEntry(t *testing.T) {
	n, c := newNode(t)
	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterConnector(c))
	assert.Equal(t, catalog.KindJEV, c.Kind())
	assert.Equal(t, []catalog.ConnectorKind{jev.Kind}, n.Requires(), "exactly one connector")
	assert.True(t, n.Idempotent())
	spec := n.Spec()
	assert.Equal(t, "ai.jev", spec.Type)
	assert.Equal(t, graph.BehaviorCall, spec.Behavior)
	assert.Equal(t, graph.DryRunExecute, spec.DryRun)
	assert.Equal(t, []string{"low_confidence", "failed"}, spec.Outcomes)
	assert.Equal(t, "failed", spec.FailedOutcome)
	assert.Equal(t, connectors.PlatformOptIn, c.Descriptor().Platform)
	assert.True(t, connectors.SecretFields(c.Descriptor())["api_key"])
	_, isDecision := n.(engine.DecisionNode)
	assert.True(t, isDecision)
}

func TestParams(t *testing.T) {
	n, _ := newNode(t)
	parse := func(raw string) []graph.Issue {
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(raw), &m))
		_, issues := n.Spec().Params.Parse(m)
		return issues
	}
	two := `[{"value":"a","description":"A"},{"value":"b","description":"B"}]`
	assert.Empty(t, parse(`{"input":"{}","choices":`+two+`}`))
	assert.Empty(t, parse(`{"input":"{}","choices":`+two+`,"min_confidence":0.9,"model":"gpt-4o-2024-08-06","timeout_ms":90000}`))

	many := make([]string, 101)
	for i := range many {
		many[i] = fmt.Sprintf(`{"value":"c%d","description":"d"}`, i)
	}
	hundred := "[" + strings.Join(many[:100], ",") + "]"
	assert.Empty(t, parse(`{"input":"{}","choices":`+hundred+`}`))

	for name, raw := range map[string]string{
		"no input":           `{"choices":` + two + `}`,
		"no choices":         `{"input":"{}"}`,
		"one choice":         `{"input":"{}","choices":[{"value":"a","description":"A"}]}`,
		"too many":           `{"input":"{}","choices":[` + strings.Join(many, ",") + `]}`,
		"duplicate":          `{"input":"{}","choices":[{"value":"a","description":"A"},{"value":"a","description":"B"}]}`,
		"bad value":          `{"input":"{}","choices":[{"value":"A","description":"A"},{"value":"b","description":"B"}]}`,
		"digit first":        `{"input":"{}","choices":[{"value":"1a","description":"A"},{"value":"b","description":"B"}]}`,
		"long value":         `{"input":"{}","choices":[{"value":"` + strings.Repeat("a", 41) + `","description":"A"},{"value":"b","description":"B"}]}`,
		"no description":     `{"input":"{}","choices":[{"value":"a","description":""},{"value":"b","description":"B"}]}`,
		"long description":   `{"input":"{}","choices":[{"value":"a","description":"` + strings.Repeat("x", 2001) + `"},{"value":"b","description":"B"}]}`,
		"confidence over 1":  `{"input":"{}","choices":` + two + `,"min_confidence":1.1}`,
		"confidence under 0": `{"input":"{}","choices":` + two + `,"min_confidence":-0.1}`,
		"blank description":  `{"input":"{}","choices":[{"value":"a","description":"   "},{"value":"b","description":"B"}]}`,
		"alias":              `{"input":"{}","choices":` + two + `,"model":"gpt-4o-latest"}`,
		"timeout":            `{"input":"{}","choices":` + two + `,"timeout_ms":300001}`,
		"rolling":            `{"input":"{}","choices":` + two + `,"model":"gpt-4o"}`,
		"latest version":     `{"input":"{}","choices":` + two + `,"model":"~typesafe/jev-latest"}`,
		"latest":             `{"input":"{}","choices":` + two + `,"model":"typesafe/jev-latest"}`,
		"string timeout":     `{"input":"{}","choices":` + two + `,"timeout":"60s"}`,
		"empty prompt":       `{"input":"{}","choices":` + two + `,"prompt":""}`,
		"unknown":            `{"input":"{}","choices":` + two + `,"images":"x"}`,
	} {
		assert.NotEmpty(t, parse(raw), name)
	}
}

func TestDefaultsAreAppliedAndPinned(t *testing.T) {
	n, _ := newNode(t)
	p := parsed(t, n, map[string]any{"input": "{}", "choices": choices(t)})
	assert.Equal(t, json.Number("0.6"), p["min_confidence"], "min_confidence defaults to 0.6")
	assert.Equal(t, jev.DefaultPrompt, p["prompt"])
	assert.Contains(t, jev.DefaultPrompt, "When in doubt, choose the safest option")
	assert.Equal(t, json.Number("60000"), p["timeout_ms"])

	// The instruction is pinned by CompleteParams; the model by CompleteParamsFor, which knows the connection.
	cn := n.(catalog.CompletedNode)
	raw := map[string]any{"input": "{}", "choices": choices(t)}
	done := cn.CompleteParams(raw)
	assert.Equal(t, jev.DefaultPrompt, done["prompt"])
	assert.NotContains(t, done, "model")
	assert.NotContains(t, raw, "prompt", "the argument is not changed")
	assert.Equal(t, done, cn.CompleteParams(done))
	assert.Equal(t, "my rules", cn.CompleteParams(map[string]any{"prompt": "my rules"})["prompt"])
	assert.Empty(t, cn.CompleteParams(map[string]any{"prompt": ""})["prompt"], "an empty prompt is refused, not replaced")
}

type providers map[string]string

func (p providers) Provider(_ context.Context, _ uuid.UUID, id string) (string, error) {
	if v, ok := p[id]; ok {
		return v, nil
	}
	return "", connectors.ErrNotFound
}

// A node saved without a model gets the default of the provider of its connection, so an OpenAI connection
// is not saved with a TypeSafe Jev version, nor an OpenRouter one with a chat model; a model that does not
// fit the connection is refused on save, and so is a connection whose provider cannot be read.
func TestModelIsPinnedByTheProviderOfTheConnection(t *testing.T) {
	src := providers{"or": "openrouter", "oa": "openai", "platform:jev": "openai", "odd": "anthropic", "empty": ""}
	n, _ := newNode(t, jev.WithProviders(src))
	cn := n.(catalog.ConnectionNode)
	ctx := context.Background()
	tenant := uuid.New()
	raw := map[string]any{"input": "{}"}
	prepare := func(conn string, p map[string]any) (map[string]any, []graph.Issue) {
		return cn.PrepareParams(ctx, tenant, catalog.ConnectionRef{ID: conn}, p)
	}

	got, issues := prepare("or", raw)
	assert.Empty(t, issues)
	assert.Equal(t, "typesafe/jev-1.14", got["model"])
	got, issues = prepare("oa", raw)
	assert.Empty(t, issues)
	assert.Equal(t, "gpt-4o-mini-2024-07-18", got["model"])
	got, _ = prepare("platform:jev", raw)
	assert.Equal(t, "gpt-4o-mini-2024-07-18", got["model"])
	assert.NotContains(t, raw, "model", "the argument is not changed")
	got, issues = prepare("or", map[string]any{"model": "typesafe/jev-1.13"})
	assert.Empty(t, issues)
	assert.Equal(t, "typesafe/jev-1.13", got["model"])

	for _, bad := range [][2]string{{"oa", "typesafe/jev-1.14"}, {"or", "gpt-4o-mini-2024-07-18"}, {"oa", "gpt-3.5-turbo-0125"}, {"oa", "gpt-4o-2024-05-13"}, {"oa", "anthropic/claude-3.5-sonnet"}, {"oa", "openai/gpt-4o-mini-2024-07-18"}} {
		_, issues := prepare(bad[0], map[string]any{"model": bad[1]})
		require.Len(t, issues, 1, bad)
		assert.Equal(t, jev.ErrorModelProvider, issues[0].Code)
		assert.Equal(t, []any{"model"}, issues[0].Path)
	}

	// A provider that cannot be read is not guessed: nothing is pinned, and the save is refused.
	for _, conn := range []string{"unknown", "odd", "empty"} {
		got, issues := prepare(conn, raw)
		require.Len(t, issues, 1, conn)
		assert.Equal(t, jev.ErrorConnectionProvider, issues[0].Code, conn)
		assert.NotContains(t, got, "model", conn)
	}
	failing := failingProviders{n: new(int)}
	nf, _ := newNode(t, jev.WithProviders(failing))
	got, issues = nf.(catalog.ConnectionNode).PrepareParams(ctx, tenant, catalog.ConnectionRef{ID: "or"}, raw)
	require.Len(t, issues, 1, "a failed lookup")
	assert.Equal(t, jev.ErrorConnectionProvider, issues[0].Code)
	assert.NotContains(t, got, "model")
	assert.Equal(t, 1, failing.calls(), "the provider is looked up once")

	// Without a source of providers the node cannot tell: the OpenRouter default, nothing to check.
	n0, _ := newNode(t)
	got, issues = n0.(catalog.ConnectionNode).PrepareParams(ctx, tenant, catalog.ConnectionRef{ID: "x"}, raw)
	assert.Empty(t, issues)
	assert.Equal(t, "typesafe/jev-1.14", got["model"])

	// The defaults follow the platform configuration, for its provider only.
	n2, _ := newNode(t, jev.WithProviders(src), jev.WithPlatformConfig(&jevconfig.Config{Provider: "openai", APIKey: "k", DefaultModel: "gpt-4.1-2025-04-14"}))
	cn2 := n2.(catalog.ConnectionNode)
	got, _ = cn2.PrepareParams(ctx, tenant, catalog.ConnectionRef{ID: "oa"}, raw)
	assert.Equal(t, "gpt-4.1-2025-04-14", got["model"])
	got, _ = cn2.PrepareParams(ctx, tenant, catalog.ConnectionRef{ID: "or"}, raw)
	assert.Equal(t, "typesafe/jev-1.14", got["model"])
	n3, _ := newNode(t, jev.WithDefaultModel("typesafe/jev-1.13"), jev.WithDefaultModel("gpt-4o-2024-08-06"))
	got, _ = n3.(catalog.ConnectionNode).PrepareParams(ctx, tenant, catalog.ConnectionRef{ID: "x"}, raw)
	assert.Equal(t, "typesafe/jev-1.13", got["model"])
}

type failingProviders struct{ n *int }

func (f failingProviders) calls() int { return *f.n }
func (f failingProviders) Provider(context.Context, uuid.UUID, string) (string, error) {
	*f.n++
	return "", errors.New("the store is down")
}

func TestShippedDefaultsArePinned(t *testing.T) {
	for _, m := range []string{jev.DefaultModelOpenRouter, jev.DefaultModelOpenAI} {
		require.NoError(t, aichat.CheckModel(m), m)
	}
	assert.Equal(t, "typesafe/jev-1.14", jev.DefaultModelOpenRouter)
}

func openrouterNode(t *testing.T, srv *aitest.Server, opts ...jev.Option) catalog.CallNode {
	t.Helper()
	n, _ := newNode(t, append([]jev.Option{jev.WithTransport(srv.Transport())}, opts...)...)
	return n
}

func openaiNode(t *testing.T, srv *aitest.Server, opts ...jev.Option) catalog.CallNode {
	t.Helper()
	return openrouterNode(t, srv, append([]jev.Option{jev.WithDefaultModel(jev.DefaultModelOpenAI)}, opts...)...)
}

// OpenRouter decides with TypeSafe Jev through its Decisions API; the node routes by the provider's own
// confidence.
func TestOpenRouterDecidesWithTypeSafeJev(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: typesafe("refund", map[string]float64{"refund": 0.8, "exchange": 0.15, "other": 0.05}, f(0.7))})
	n := openrouterNode(t, srv)
	usage := &aitest.Usage{}
	in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, map[string]any{"prompt": "Decide for the customer"}), usage)
	in.Params["model"] = "typesafe/jev-1.14"
	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)

	req := srv.Requests()[0]
	assert.Equal(t, "openrouter.ai", req.Host())
	assert.Equal(t, "/api/alpha/decisions", req.Path)
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "Bearer sk-test-key-123456", req.Header.Get("Authorization"))
	body := req.JSON(t)
	assert.Equal(t, "typesafe/jev-1.14", body["model"], "the pinned Jev version")
	assert.Equal(t, map[string]any{
		"count": float64(2), "labels": []any{"shoes", "urgent"}, "meta": map[string]any{"a": "1", "b": "2"}, "title": "Wrong size", "urgent": true,
	}, body["state"], "the input is the state, whatever the types of its values")
	q := body["questions"].(map[string]any)["decision"].(map[string]any)
	assert.Equal(t, "choice", q["type"])
	assert.Equal(t, "Decide for the customer", q["instructions"])
	assert.Equal(t, map[string]any{
		"refund": "The customer asks for money back.", "exchange": "The customer wants another item.", "other": "Anything else.",
	}, q["criteria"])
	assert.NotContains(t, body, "messages", "no chat completion")

	// Routing is by the provider's confidence (0.7), not by the probability of the choice (0.8).
	assert.Equal(t, "refund", res.Outcome)
	assert.InDelta(t, 0.7, res.Output["confidence"], 1e-9)
	assert.Equal(t, "refund", res.Output["choice"])
	assert.Equal(t, "openrouter", res.Output["provider"])
	assert.Equal(t, "typesafe/jev-1.14-20261014", res.Output["model"], "the snapshot that answered")
	probs := res.Output["probabilities"].(map[string]any)
	assert.InDelta(t, 0.8, probs["refund"], 1e-9)
	assert.Equal(t, map[string]any{"input_tokens": int64(120), "output_tokens": int64(18), "cost_usd": 0.0042}, res.Output["usage"])
	assert.EqualValues(t, 120, usage.Total("token_in"))
	assert.EqualValues(t, 18, usage.Total("token_out"))
	for _, e := range usage.Entries {
		assert.Equal(t, jev.Kind, e.Kind)
		assert.Equal(t, in.Connection.ID, e.ConnectionID)
	}

	// min_confidence 0.9 against a confidence of 0.7: low_confidence, and the choice stays in the output.
	in = aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, map[string]any{"min_confidence": 0.9}), nil)
	in.Params["model"] = "typesafe/jev-1.14"
	res, err = n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "low_confidence", res.Outcome)
	assert.Equal(t, "refund", res.Output["choice"])
	assert.InDelta(t, 0.7, res.Output["confidence"], 1e-9)

	// A floor between the provider's confidence (0.7) and the probability of the choice (0.8) tells which one
	// routes: the confidence is under it.
	in = aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, map[string]any{"min_confidence": 0.75}), nil)
	in.Params["model"] = "typesafe/jev-1.14"
	res, err = n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "low_confidence", res.Outcome, "routed by the provider's confidence, not by the probability of the choice")

	// The floor itself takes the choice.
	in = aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, map[string]any{"min_confidence": 0.7}), nil)
	in.Params["model"] = "typesafe/jev-1.14"
	res, err = n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "refund", res.Outcome)
}

// The provider's confidence is required: it is a measure of the distribution, not a copy of the probability
// of the choice, so a reply without it is not routed on.
func TestOpenRouterWithoutAConfidenceIsInvalid(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: typesafe("exchange", map[string]float64{"refund": 0.1, "exchange": 0.85, "other": 0.05}, nil)})
	n := openrouterNode(t, srv)
	usage := &aitest.Usage{}
	in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), usage)
	in.Params["model"] = "typesafe/jev-1.14"
	_, err := n.Execute(context.Background(), in)
	assert.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code)
	assert.Len(t, srv.Requests(), 1)
	assert.Positive(t, usage.Total("token_in"), "the tokens were spent")
}

// A null, a string or a missing value is not a probability (decoded into numbers it would be 0).
func TestOpenRouterNonNumericWireValuesAreInvalid(t *testing.T) {
	usage := `"usage":{"input_tokens":9,"output_tokens":1}`
	for name, body := range map[string]string{
		"null probability":    `{"model":"m","answers":{"decision":{"type":"choice","choice":"refund","probabilities":{"refund":1,"exchange":null,"other":0},"confidence":0.9}},` + usage + `}`,
		"string probability":  `{"model":"m","answers":{"decision":{"type":"choice","choice":"refund","probabilities":{"refund":1,"exchange":"0","other":0},"confidence":0.9}},` + usage + `}`,
		"missing probability": `{"model":"m","answers":{"decision":{"type":"choice","choice":"refund","confidence":0.9}},` + usage + `}`,
		"null confidence":     `{"model":"m","answers":{"decision":{"type":"choice","choice":"refund","probabilities":{"refund":1,"exchange":0,"other":0},"confidence":null}},` + usage + `}`,
		"string confidence":   `{"model":"m","answers":{"decision":{"type":"choice","choice":"refund","probabilities":{"refund":1,"exchange":0,"other":0},"confidence":"high"}},` + usage + `}`,
		"array probabilities": `{"model":"m","answers":{"decision":{"type":"choice","choice":"refund","probabilities":[1,0,0],"confidence":0.9}},` + usage + `}`,
		"numeric choice":      `{"model":"m","answers":{"decision":{"type":"choice","choice":7,"probabilities":{"refund":1,"exchange":0,"other":0},"confidence":0.9}},` + usage + `}`,
		"no type":             `{"model":"m","answers":{"decision":{"choice":"refund","probabilities":{"refund":1,"exchange":0,"other":0},"confidence":0.9}},` + usage + `}`,
		"answers is a list":   `{"model":"m","answers":[],` + usage + `}`,
		"decision is a list":  `{"model":"m","answers":{"decision":[]},` + usage + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: body})
			n := openrouterNode(t, srv)
			rec := &aitest.Usage{}
			in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), rec)
			in.Params["model"] = "typesafe/jev-1.14"
			_, err := n.Execute(context.Background(), in)
			ce := callErr(t, err)
			assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
			assert.False(t, ce.Retryable, "not repeated")
			assert.Len(t, srv.Requests(), 1)
			assert.EqualValues(t, 9, rec.Total("token_in"), "the tokens were spent")
			assert.EqualValues(t, 1, rec.Total("token_out"))
		})
	}
}

// Metadata of a Decisions answer of another type (a model or a provider that is not a string, a usage that
// is not an object) neither hides the tokens of the answer nor makes the node repeat the call: a usable
// decision is taken, an unusable one is one invalid request, and the reported tokens are recorded either way.
func TestOpenRouterMalformedMetadataKeepsTheUsageAndIsNotRepeated(t *testing.T) {
	answer := `"answers":{"decision":{"type":"choice","choice":"refund","probabilities":{"refund":0.9,"exchange":0.05,"other":0.05},"confidence":0.9}}`
	usage := `"usage":{"input_tokens":9,"output_tokens":1}`
	for name, tc := range map[string]struct {
		body    string
		ok      bool
		in, out int64
	}{
		"model is a number":       {`{"model":7,"provider":"TypeSafe",` + answer + `,` + usage + `}`, true, 9, 1},
		"provider is an object":   {`{"model":"m","provider":{"name":"x"},` + answer + `,` + usage + `}`, true, 9, 1},
		"metadata is null":        {`{"model":null,"provider":null,` + answer + `,` + usage + `}`, true, 9, 1},
		"usage is a list":         {`{"model":"m",` + answer + `,"usage":[9,1]}`, true, 0, 0},
		"usage figure is text":    {`{"model":"m",` + answer + `,"usage":{"input_tokens":"nine","output_tokens":1}}`, true, 0, 1},
		"bad metadata and answer": {`{"model":false,"provider":[],"answers":"none",` + usage + `}`, false, 9, 1},
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: tc.body})
			n := openrouterNode(t, srv)
			rec := &aitest.Usage{}
			in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), rec)
			in.Params["model"] = "typesafe/jev-1.14"
			res, err := n.Execute(context.Background(), in)
			if tc.ok {
				require.NoError(t, err)
				assert.Equal(t, "refund", res.Outcome)
				if !strings.Contains(tc.body, `"model":"m"`) {
					assert.Equal(t, "typesafe/jev-1.14", res.Output["model"], "a model that is not a string is the one asked for")
				}
			} else {
				ce := callErr(t, err)
				assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
				assert.False(t, ce.Retryable, "not repeated")
			}
			assert.Len(t, srv.Requests(), 1, "exactly one request")
			assert.Equal(t, tc.in, rec.Total("token_in"))
			assert.Equal(t, tc.out, rec.Total("token_out"))
		})
	}
}

// A call whose params name no model (a test of a node before it is saved) uses the default of the provider
// of its connection.
func TestMissingModelFallsBackToTheProvidersDefault(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: decision("other", map[string]float64{"refund": 0.1, "exchange": 0.1, "other": 0.8})})
	n := openaiNode(t, srv)
	_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, nil), nil))
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o-mini-2024-07-18", srv.Requests()[0].JSON(t)["model"])
}

// OpenRouter takes a pinned TypeSafe Jev version; a chat model there, or a Jev version on OpenAI, is a
// refused call before any request.
func TestModelMustFitTheProviderOfTheConnection(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: typesafe("refund", map[string]float64{"refund": 1, "exchange": 0, "other": 0}, nil)})
	n := openrouterNode(t, srv)
	for provider, model := range map[string]string{
		"openrouter": "gpt-4o-mini-2024-07-18", "openai": "typesafe/jev-1.14",
	} {
		in := aitest.Input(jev.Kind, srv, creds(provider), base(t, n, nil), nil)
		in.Params["model"] = model
		_, err := n.Execute(context.Background(), in)
		assert.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code, provider+" "+model)
	}
	assert.Empty(t, srv.Requests())
}

func TestOpenRouterAnswersOutsideTheChoicesAreOneInvalidRequest(t *testing.T) {
	good := map[string]float64{"refund": 0.5, "exchange": 0.3, "other": 0.2}
	for name, body := range map[string]string{
		"unknown choice":      typesafe("complaint", good, f(0.9)),
		"empty choice":        typesafe("", good, f(0.9)),
		"missing probability": typesafe("refund", map[string]float64{"refund": 0.5, "exchange": 0.5}, f(0.9)),
		"extra probability":   typesafe("refund", map[string]float64{"refund": 0.4, "exchange": 0.2, "other": 0.2, "x": 0.2}, f(0.9)),
		"over one":            typesafe("refund", map[string]float64{"refund": 1.01, "exchange": 0, "other": 0}, f(0.9)),
		"negative":            typesafe("refund", map[string]float64{"refund": 1, "exchange": -0.1, "other": 0.1}, f(0.9)),
		"does not sum":        typesafe("refund", map[string]float64{"refund": 0.5, "exchange": 0.1, "other": 0.1}, f(0.9)),
		"all zero":            typesafe("refund", map[string]float64{"refund": 0, "exchange": 0, "other": 0}, f(0.9)),
		"confidence over one": typesafe("refund", good, f(1.2)),
		"confidence negative": typesafe("refund", good, f(-0.1)),
		"no probabilities":    typesafe("refund", nil, f(0.9)),
		"no decision":         `{"model":"typesafe/jev-1.14","answers":{},"usage":{"input_tokens":9,"output_tokens":1}}`,
		"wrong type":          `{"model":"typesafe/jev-1.14","answers":{"decision":{"type":"score","score":3}},"usage":{"input_tokens":9,"output_tokens":1}}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: body})
			n := openrouterNode(t, srv)
			usage := &aitest.Usage{}
			in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), usage)
			in.Params["model"] = "typesafe/jev-1.14"
			_, err := n.Execute(context.Background(), in)
			ce := callErr(t, err)
			assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
			assert.False(t, ce.Retryable)
			assert.Len(t, srv.Requests(), 1, "exactly one request")
		})
	}
}

func TestOpenRouterFailures(t *testing.T) {
	cases := []struct {
		name      string
		reply     aitest.Reply
		code      string
		retryable bool
		after     time.Duration
	}{
		{"unauthorized", aitest.Reply{Status: 401, Body: `{"error":{"code":401,"message":"Missing Authentication header"}}`}, catalog.CallUnauthorized, false, 0},
		{"no credit", aitest.Reply{Status: 402, Body: `{"error":{"code":402,"message":"Insufficient credits"}}`}, catalog.CallUnavailable, false, 0},
		{"bad request", aitest.Reply{Status: 400, Body: `{"error":{"code":400,"message":"Invalid request parameters"}}`}, catalog.CallInvalidRequest, false, 0},
		{"too large", aitest.Reply{Status: 413, Body: `{"error":{"code":413,"message":"Request payload too large"}}`}, catalog.CallInvalidRequest, false, 0},
		{"rate limited", aitest.Reply{Status: 429, Body: `{"error":{"code":429,"message":"Rate limit exceeded"}}`}, catalog.CallRateLimited, true, 0},
		{"unavailable keeps the delay", aitest.Reply{Status: 503, Header: map[string]string{"Retry-After": "16"}, Body: `{}`}, catalog.CallUnavailable, true, 16 * time.Second},
		{"timed out", aitest.Reply{Status: 524, Body: `{"error":{"code":524,"message":"Request timed out"}}`}, catalog.CallUnavailable, true, 0},
		{"overloaded", aitest.Reply{Status: 529, Body: `{}`}, catalog.CallUnavailable, true, 0},
		{"refusal", aitest.Reply{Body: `{"model":"typesafe/jev-1.14","answers":{"decision":{"type":"refusal"}},"usage":{"input_tokens":9,"output_tokens":1}}`}, catalog.CallRejected, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := aitest.New(t, tc.reply)
			n := openrouterNode(t, srv)
			usage := &aitest.Usage{}
			in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), usage)
			in.Params["model"] = "typesafe/jev-1.14"
			_, err := n.Execute(context.Background(), in)
			ce := callErr(t, err)
			assert.Equal(t, tc.code, ce.Code)
			assert.Equal(t, tc.retryable, ce.Retryable)
			assert.Equal(t, tc.after, ce.RetryAfter)
			assert.Len(t, srv.Requests(), 1)
			if tc.name == "refusal" {
				assert.EqualValues(t, 9, usage.Total("token_in"), "the tokens of a refusal were spent")
			} else {
				assert.Empty(t, usage.Entries)
			}
		})
	}
}

func TestRetryAfterOf429IsGivenToTheEngine(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Status: 429, Header: map[string]string{"Retry-After": "3"}, Body: `{}`})
	n := openrouterNode(t, srv)
	in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), nil)
	in.Params["model"] = "typesafe/jev-1.14"
	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, 3*time.Second, res.RetryAfter)
}

func TestTheEngineTakesTheOutcomeOfTheAnswer(t *testing.T) {
	// The outcome the engine derives from the output (graph.Interpreter.CallOutcome) is the node's own.
	v := aitest.View(t, jev.New())
	doc := aitest.Graph("ai.jev", `{"input":"{'title': event.title}","choices":`+choicesJSON+`,"min_confidence":0.9,"model":"typesafe/jev-1.14"}`,
		[]string{"refund", "exchange", "other", "low_confidence", "failed"})
	in := aitest.Interpreter(t, v, doc)

	out, st := in.CallOutcome("n1", map[string]any{"choice": "refund", "confidence": 0.7})
	require.Nil(t, st)
	assert.Equal(t, "low_confidence", out)
	out, st = in.CallOutcome("n1", map[string]any{"choice": "exchange", "confidence": 0.95})
	require.Nil(t, st)
	assert.Equal(t, "exchange", out)
	_, st = in.CallOutcome("n1", map[string]any{"choice": "nope", "confidence": 0.95})
	assert.NotNil(t, st, "a choice outside the choices is no outcome")
}

// --- the openai provider: a chat model in strict structured output ---

func TestOpenAIProvider(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: decision("other", map[string]float64{"refund": 0.1, "exchange": 0.1, "other": 0.8})})
	n := openaiNode(t, srv)
	usage := &aitest.Usage{}
	res, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, map[string]any{"prompt": "Decide for {{ event.title }}"}), usage))
	require.NoError(t, err)
	assert.Equal(t, "openai", res.Output["provider"])
	assert.Equal(t, "other", res.Outcome)
	assert.InDelta(t, 0.8, res.Output["confidence"], 1e-9, "the probability of the choice")
	assert.EqualValues(t, 120, usage.Total("token_in"))

	req := srv.Requests()[0]
	assert.Equal(t, "api.openai.com", req.Host())
	body := req.JSON(t)
	assert.Equal(t, "gpt-4o-mini-2024-07-18", body["model"])
	assert.EqualValues(t, 0, body["temperature"])
	assert.Contains(t, body, "max_completion_tokens")
	assert.NotContains(t, body, "usage")
	assert.NotContains(t, body, "provider")
	msgs := body["messages"].([]any)
	require.Len(t, msgs, 2)
	system := msgs[0].(map[string]any)["content"].(string)
	for _, c := range []string{"- refund: The customer asks for money back.", "- exchange:", "- other: Anything else."} {
		assert.Contains(t, system, c)
	}
	user := msgs[1].(map[string]any)["content"].(string)
	assert.True(t, strings.HasPrefix(user, "Decide for {{ event.title }}\n\nInput (JSON):\n"), "the engine renders templates; the test passes one as is")
	// The input is serialized with sorted keys, whatever its values.
	assert.Contains(t, user, "{\n  \"count\": 2,\n  \"labels\": [\n    \"shoes\",\n    \"urgent\"\n  ],\n  \"meta\": {\n    \"a\": \"1\",\n    \"b\": \"2\"\n  },\n  \"title\": \"Wrong size\",\n  \"urgent\": true\n}")

	js := body["response_format"].(map[string]any)["json_schema"].(map[string]any)
	assert.Equal(t, true, js["strict"])
	schema := js["schema"].(map[string]any)
	assert.Equal(t, []any{"refund", "exchange", "other"}, schema["properties"].(map[string]any)["choice"].(map[string]any)["enum"])
	probs := schema["properties"].(map[string]any)["probabilities"].(map[string]any)
	assert.Equal(t, false, probs["additionalProperties"])
	assert.Equal(t, []any{"refund", "exchange", "other"}, probs["required"])
}

func TestOpenAIOutcomes(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: decision("refund", map[string]float64{"refund": 0.7, "exchange": 0.2, "other": 0.1})})
	n := openaiNode(t, srv)
	res, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, map[string]any{"min_confidence": 0.9}), nil))
	require.NoError(t, err)
	assert.Equal(t, "low_confidence", res.Outcome)
	assert.Equal(t, "refund", res.Output["choice"])
}

func TestReasoningModelsGetNoTemperature(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: decision("other", map[string]float64{"refund": 0, "exchange": 0, "other": 1})})
	n := openaiNode(t, srv)
	_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, map[string]any{"model": "o3-2025-04-16"}), nil))
	require.NoError(t, err)
	assert.NotContains(t, srv.Requests()[0].JSON(t), "temperature")
}

func TestOpenAIAnswersOutsideTheChoicesAreOneInvalidRequest(t *testing.T) {
	good := map[string]float64{"refund": 0.5, "exchange": 0.3, "other": 0.2}
	for name, body := range map[string]string{
		"unknown choice":      decision("complaint", good),
		"empty choice":        decision("", good),
		"missing probability": decision("refund", map[string]float64{"refund": 0.5, "exchange": 0.5}),
		"extra probability":   decision("refund", map[string]float64{"refund": 0.4, "exchange": 0.2, "other": 0.2, "x": 0.2}),
		"above one":           decision("refund", map[string]float64{"refund": 1.01, "exchange": 0, "other": 0}),
		"percentages":         decision("refund", map[string]float64{"refund": 80, "exchange": 10, "other": 10}),
		"negative":            decision("refund", map[string]float64{"refund": 1, "exchange": -0.1, "other": 0.1}),
		"does not sum":        decision("refund", map[string]float64{"refund": 0.2, "exchange": 0.1, "other": 0.1}),
		"all zero":            decision("refund", map[string]float64{"refund": 0, "exchange": 0, "other": 0}),
		"not json":            aitest.Chat("refund", 10, 1),
		"not an object":       aitest.Chat(`["refund"]`, 10, 1),
		"extra field":         aitest.Chat(`{"choice":"refund","probabilities":{"refund":1,"exchange":0,"other":0},"why":"x"}`, 10, 1),
		"probabilities text":  aitest.Chat(`{"choice":"refund","probabilities":{"refund":"high","exchange":0,"other":0}}`, 10, 1),
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: body})
			n := openaiNode(t, srv)
			usage := &aitest.Usage{}
			_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, nil), usage))
			ce := callErr(t, err)
			assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
			assert.False(t, ce.Retryable)
			assert.Len(t, srv.Requests(), 1, "exactly one request")
			assert.Positive(t, usage.Total("token_in"), "the tokens were spent")
		})
	}
}

// Malformed choices of a Chat Completions answer (the openai provider) are one invalid request, not repeated,
// and the tokens the answer reports are recorded.
func TestOpenAIMalformedChoicesKeepTheUsageAndAreNotRepeated(t *testing.T) {
	usage := `"usage":{"prompt_tokens":10,"completion_tokens":2}`
	for name, body := range map[string]string{
		"choices is an object":   `{` + usage + `,"choices":{"a":1}}`,
		"choices is text":        `{` + usage + `,"choices":"refund"}`,
		"choices is null":        `{` + usage + `,"choices":null}`,
		"no choices":             `{` + usage + `,"choices":[]}`,
		"choice is a list":       `{` + usage + `,"choices":[[]]}`,
		"message is a list":      `{` + usage + `,"choices":[{"finish_reason":"stop","message":[]}]}`,
		"finish reason a number": `{` + usage + `,"choices":[{"finish_reason":1,"message":{"content":"{}"}}]}`,
		"content is a number":    `{` + usage + `,"choices":[{"finish_reason":"stop","message":{"content":5}}]}`,
		"model is not a string":  `{"model":{},` + usage + `,"choices":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: body})
			n := openaiNode(t, srv)
			rec := &aitest.Usage{}
			_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, nil), rec))
			ce := callErr(t, err)
			assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
			assert.False(t, ce.Retryable, "not repeated")
			assert.Len(t, srv.Requests(), 1, "exactly one request")
			assert.EqualValues(t, 10, rec.Total("token_in"), "the tokens were spent")
			assert.EqualValues(t, 2, rec.Total("token_out"))
		})
	}
}

func TestOpenAIFailures(t *testing.T) {
	cases := []struct {
		name      string
		reply     aitest.Reply
		code      string
		retryable bool
	}{
		{"unauthorized", aitest.Reply{Status: 401, Body: aitest.APIError("bad key", "", "401")}, catalog.CallUnauthorized, false},
		{"rate limited", aitest.Reply{Status: 429, Body: aitest.APIError("slow", "", "429")}, catalog.CallRateLimited, true},
		{"unavailable", aitest.Reply{Status: 502, Body: `bad gateway`}, catalog.CallUnavailable, true},
		{"context", aitest.Reply{Status: 400, Body: aitest.APIError("too long", "invalid_request_error", "context_length_exceeded")}, catalog.CallInvalidRequest, false},
		{"refusal", aitest.Reply{Body: `{"choices":[{"finish_reason":"stop","message":{"content":null,"refusal":"no"}}],"usage":{"prompt_tokens":4,"completion_tokens":1}}`}, catalog.CallRejected, false},
		{"cut off", aitest.Reply{Body: `{"choices":[{"finish_reason":"length","message":{"content":"{\"choice\":"}}],"usage":{"prompt_tokens":4,"completion_tokens":4000}}`}, catalog.CallInvalidRequest, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := aitest.New(t, tc.reply)
			n := openaiNode(t, srv)
			_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, nil), &aitest.Usage{}))
			ce := callErr(t, err)
			assert.Equal(t, tc.code, ce.Code)
			assert.Equal(t, tc.retryable, ce.Retryable)
			assert.Len(t, srv.Requests(), 1)
		})
	}
}

func TestBadCallsAreRefusedBeforeAnyRequest(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: typesafe("refund", map[string]float64{"refund": 1, "exchange": 0, "other": 0}, nil)})
	n := openrouterNode(t, srv)
	run := func(p map[string]any, c map[string]string) error {
		p["model"] = "typesafe/jev-1.14"
		_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, c, p, nil))
		return err
	}

	notMap := base(t, n, nil)
	notMap["input"] = "just a string"
	ce := callErr(t, run(notMap, creds("openrouter")))
	assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
	assert.Contains(t, ce.Message, "map")

	list := base(t, n, nil)
	list["input"] = []any{"a"}
	assert.Equal(t, catalog.CallInvalidRequest, callErr(t, run(list, creds("openrouter"))).Code)

	blank := base(t, n, nil)
	blank["prompt"] = " \n"
	assert.Contains(t, callErr(t, run(blank, creds("openrouter"))).Message, "prompt")

	assert.Equal(t, catalog.CallUnauthorized, callErr(t, run(base(t, n, nil), map[string]string{"provider": "openrouter", "api_key": ""})).Code)
	assert.Equal(t, catalog.CallInvalidRequest, callErr(t, run(base(t, n, nil), map[string]string{"provider": "anthropic"})).Code)

	reserved := base(t, n, nil)
	reserved["choices"] = []any{
		map[string]any{"value": "failed", "description": "x"}, map[string]any{"value": "b", "description": "y"},
	}
	assert.Equal(t, catalog.CallInvalidRequest, callErr(t, run(reserved, creds("openrouter"))).Code)
	assert.Empty(t, srv.Requests())
}

func TestUnauthorizedMarksTheConnection(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Status: 401, Body: `{}`})
	set := &statuses{}
	n := openrouterNode(t, srv, jev.WithConnectionStatus(set))
	in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), nil)
	in.Params["model"] = "typesafe/jev-1.14"
	_, err := n.Execute(context.Background(), in)
	assert.Equal(t, catalog.CallUnauthorized, callErr(t, err).Code)
	assert.Equal(t, []string{in.Connection.ID}, set.ids)
}

type statuses struct{ ids []string }

func (s *statuses) SetStatus(_ context.Context, _, id uuid.UUID, st connectors.ConnectionStatus, _ string) error {
	if st == connectors.StatusNeedsReauth {
		s.ids = append(s.ids, id.String())
	}
	return nil
}

// --- graph ---

func TestGraphAcceptsAMixedInputAndTypesTheChoice(t *testing.T) {
	v := aitest.View(t, jev.New())
	outcomes := []string{"refund", "exchange", "other", "low_confidence", "failed"}
	node := func(input string) string {
		return aitest.Graph("ai.jev", `{"input":"`+input+`","choices":`+choicesJSON+`,"model":"gpt-4o-2024-08-06"}`, outcomes)
	}
	// {title: string, labels: list<string>}: a list beside a string is not a type error.
	assert.Empty(t, aitest.Validate(t, v, node(`{'title': event.title, 'labels': event.labels}`)))
	assert.Empty(t, aitest.Validate(t, v, node(`{'title': event.title, 'labels': event.labels, 'count': event.count, 'urgent': event.urgent, 'meta': event.meta, 'nested': {'a': [1, 2]}}`)))
	assert.Empty(t, aitest.Validate(t, v, node(`{'text': 'plain'}`)))

	// The input is a map.
	assert.NotEmpty(t, aitest.Validate(t, v, node(`event.title`)))
	assert.NotEmpty(t, aitest.Validate(t, v, node(`event.labels`)))
	// and reads only what the event has, and no secrets.
	assert.NotEmpty(t, aitest.Validate(t, v, node(`{'x': event.nope}`)))
	assert.NotEmpty(t, aitest.Validate(t, v, node(`{'x': secrets.key}`)))

	// The outcomes are the choices, low_confidence and failed: a graph that connects only some is refused.
	errs := aitest.Validate(t, v, aitest.Graph("ai.jev", `{"input":"{'a': 1}","choices":`+choicesJSON+`}`, []string{"refund"}))
	assert.NotEmpty(t, errs)
	// An outcome that is not a choice is refused.
	errs = aitest.Validate(t, v, aitest.Graph("ai.jev", `{"input":"{'a': 1}","choices":`+choicesJSON+`}`, append([]string{"complaint"}, outcomes...)))
	assert.NotEmpty(t, errs)
}

func TestGraphRefusesReservedAndBadChoices(t *testing.T) {
	v := aitest.View(t, jev.New())
	for _, reserved := range []string{"low_confidence", "failed"} {
		doc := aitest.Graph("ai.jev",
			`{"input":"{'a': 1}","choices":[{"value":"`+reserved+`","description":"x"},{"value":"b","description":"y"}]}`,
			[]string{"b", "low_confidence", "failed"})
		errs := aitest.Validate(t, v, doc)
		require.NotEmpty(t, errs, reserved)
		assert.Contains(t, errs[0].Path, "choices[0]")
	}
	// An alias is not saved.
	errs := aitest.Validate(t, v, aitest.Graph("ai.jev", `{"input":"{'a': 1}","model":"gpt-4o-latest","choices":`+choicesJSON+`}`,
		[]string{"refund", "exchange", "other", "low_confidence", "failed"}))
	require.Len(t, errs, 1)
	assert.Equal(t, "nodes[0].params.model", errs[0].Path)
}

func TestGraphTypesTheOutput(t *testing.T) {
	v := aitest.View(t, jev.New())
	doc := func(expression string) string {
		return aitest.Doc(
			`{"id":"n1","type":"ai.jev","params":{"input":"{'a': 1}","choices":`+choicesJSON+`,"model":"typesafe/jev-1.14"}},{"id":"check","type":"branch","params":{"expression":"`+expression+`"}}`,
			`{"from":{"node":"t","outcome":"fired"},"to":"n1"},`+
				`{"from":{"node":"n1","outcome":"refund"},"to":"check"},{"from":{"node":"n1","outcome":"exchange"},"to":"ignore"},{"from":{"node":"n1","outcome":"other"},"to":"ignore"},`+
				`{"from":{"node":"n1","outcome":"low_confidence"},"to":"ignore"},{"from":{"node":"n1","outcome":"failed"},"to":"ignore"},`+
				`{"from":{"node":"check","outcome":"true"},"to":"ignore"},{"from":{"node":"check","outcome":"false"},"to":"ignore"}`)
	}
	assert.Empty(t, aitest.Validate(t, v, doc(`nodes.n1.output.choice == 'refund'`)))
	assert.Empty(t, aitest.Validate(t, v, doc(`nodes.n1.output.confidence > 0.8`)))
	assert.Empty(t, aitest.Validate(t, v, doc(`nodes.n1.output.probabilities['refund'] > 0.5`)))
	assert.Empty(t, aitest.Validate(t, v, doc(`nodes.n1.output.usage.output_tokens < 100`)))
	assert.NotEmpty(t, aitest.Validate(t, v, doc(`nodes.n1.output.confidence == 'high'`)))
	assert.NotEmpty(t, aitest.Validate(t, v, doc(`nodes.n1.output.nope`)))
}

// --- decision keys ---

func TestDecisionKey(t *testing.T) {
	n, _ := newNode(t)
	dn := n.(engine.DecisionNode)
	key := func(extra map[string]any, mutate func(map[string]any)) string {
		p := base(t, n, extra)
		if mutate != nil {
			mutate(p)
		}
		k, ok := dn.DecisionKey(catalog.CallInput{Params: p, Credentials: catalog.Credentials{"provider": "openrouter"}})
		require.True(t, ok)
		return k
	}
	k := key(nil, nil)
	assert.Equal(t, k, key(nil, nil))
	assert.Equal(t, k, key(map[string]any{"model": "typesafe/jev-1.14", "min_confidence": 0.6}, nil), "defaults are the same call")
	for name, other := range map[string]string{
		"prompt":         key(map[string]any{"prompt": "other rules"}, nil),
		"model":          key(map[string]any{"model": "typesafe/jev-1.13"}, nil),
		"min confidence": key(map[string]any{"min_confidence": 0.9}, nil),
		"input":          key(nil, func(p map[string]any) { p["input"] = map[string]any{"title": "other"} }),
		"input value":    key(nil, func(p map[string]any) { p["input"].(map[string]any)["count"] = json.Number("3") }),
		"choices": key(nil, func(p map[string]any) {
			p["choices"].([]any)[0].(map[string]any)["description"] = "changed"
		}),
	} {
		assert.NotEqual(t, k, other, name)
	}
	// A decision of one connection or provider is not another's.
	withConn := func(provider, conn string) string {
		p := base(t, n, nil)
		p["model"] = "typesafe/jev-1.14" // the same model: only the connection and the provider differ
		k, ok := dn.DecisionKey(catalog.CallInput{Params: p, Credentials: catalog.Credentials{"provider": provider}, Connection: &catalog.ConnectionInfo{ID: conn}})
		require.True(t, ok)
		return k
	}
	assert.NotEqual(t, withConn("openrouter", "c1"), withConn("openrouter", "c2"))
	assert.NotEqual(t, withConn("openrouter", "c1"), withConn("openai", "c1"))
	// The order of the keys of the input does not matter.
	assert.Equal(t,
		key(nil, func(p map[string]any) { p["input"] = map[string]any{"a": "1", "b": []any{"x", "y"}} }),
		key(nil, func(p map[string]any) { p["input"] = map[string]any{"b": []any{"x", "y"}, "a": "1"} }))

	_, ok := dn.DecisionKey(catalog.CallInput{Params: map[string]any{"input": "not a map"}})
	assert.False(t, ok)
}

// --- the connector ---

func TestConnectorTestOpenRouter(t *testing.T) {
	srv := aitest.New(t)
	srv.Handler = func(r aitest.Request) aitest.Reply {
		switch r.Path {
		case "/api/v1/auth/key":
			return aitest.Reply{Body: `{"data":{"label":"sk-or-v1-abc...xyz","usage":0}}`}
		case "/api/v1/models":
			return aitest.Reply{Body: `{"data":[{"id":"openai/gpt-4o"}]}`}
		}
		return aitest.Reply{Status: 404, Body: `{}`}
	}
	c := jev.New(jev.WithTransport(srv.Transport()))
	id, err := c.Test(context.Background(), connectors.Credentials{"provider": "openrouter", "api_key": "sk-or-v1-secret"})
	require.NoError(t, err)
	assert.Equal(t, "sk-or-v1-abc...xyz", id.Display)
	paths := []string{}
	for _, r := range srv.Requests() {
		paths = append(paths, r.Path)
		assert.Equal(t, "openrouter.ai", r.Host())
		assert.Equal(t, "Bearer sk-or-v1-secret", r.Header.Get("Authorization"))
	}
	assert.Equal(t, []string{"/api/v1/auth/key", "/api/v1/models"}, paths)

	bad := aitest.New(t, aitest.Reply{Status: 401, Body: aitest.APIError("No auth credentials found sk-or-v1-secret", "", "401")})
	c = jev.New(jev.WithTransport(bad.Transport()))
	_, err = c.Test(context.Background(), connectors.Credentials{"provider": "openrouter", "api_key": "sk-or-v1-secret"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "sk-or-v1-secret")

	// A refusal that keeps the provider's detail: the detail stays, the key in it does not.
	detail := aitest.New(t, aitest.Reply{Status: 400, Body: aitest.APIError("harmless-marker for sk-or-v1-secret", "", "400")})
	c = jev.New(jev.WithTransport(detail.Transport()))
	_, err = c.Test(context.Background(), connectors.Credentials{"provider": "openrouter", "api_key": "sk-or-v1-secret"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "harmless-marker")
	assert.NotContains(t, err.Error(), "sk-or-v1-secret")

	// A 200 that carries no key data authenticated nothing.
	empty := aitest.New(t)
	empty.Handler = func(r aitest.Request) aitest.Reply {
		if strings.HasSuffix(r.Path, "/auth/key") {
			return aitest.Reply{Body: `{}`}
		}
		return aitest.Reply{Body: `{"data":[{"id":"m"}]}`}
	}
	c = jev.New(jev.WithTransport(empty.Transport()))
	_, err = c.Test(context.Background(), connectors.Credentials{"provider": "openrouter", "api_key": "sk-or-v1-secret"})
	require.Error(t, err)
}

func TestConnectorTestOpenAI(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Header: map[string]string{"openai-organization": "org-acme"}, Body: `{"object":"list","data":[{"id":"gpt-4o"}]}`})
	c := jev.New(jev.WithTransport(srv.Transport()))
	id, err := c.Test(context.Background(), connectors.Credentials{"provider": "openai", "api_key": "sk-1"})
	require.NoError(t, err)
	assert.Equal(t, "org-acme", id.Display)
	require.Len(t, srv.Requests(), 1)
	assert.Equal(t, "api.openai.com", srv.Requests()[0].Host())
	assert.Equal(t, "/v1/models", srv.Requests()[0].Path)

	_, err = c.Test(context.Background(), connectors.Credentials{"provider": "gemini", "api_key": "k"})
	require.Error(t, err)
	_, err = c.Test(context.Background(), connectors.Credentials{"provider": "openai"})
	require.Error(t, err)
}

func TestPlatformCredentials(t *testing.T) {
	assert.Empty(t, jev.PlatformCredentials(nil))
	assert.Empty(t, jev.PlatformCredentials(&jevconfig.Config{Provider: "openai"}))
	got := jev.PlatformCredentials(&jevconfig.Config{APIKey: "sk-or-1"})
	assert.Equal(t, connectors.Credentials{"provider": "openrouter", "api_key": "sk-or-1"}, got["jev"].Credentials)
	got = jev.PlatformCredentials(&jevconfig.Config{Provider: "openai", APIKey: "sk-1"})
	assert.Equal(t, connectors.Credentials{"provider": "openai", "api_key": "sk-1"}, got["jev"].Credentials)
}

func TestCredentialsSchemaValidates(t *testing.T) {
	d := jev.New().Descriptor()
	require.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"provider": "openrouter", "api_key": "k"}))
	require.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"provider": "openai", "api_key": "k"}))
	require.Error(t, connectors.ValidateCredentials(d, connectors.Credentials{"provider": "gemini", "api_key": "k"}))
	require.Error(t, connectors.ValidateCredentials(d, connectors.Credentials{"api_key": "k"}))
	assert.Equal(t, map[string]string{"provider": "openai"}, connectors.RedactedCredentials(d, connectors.Credentials{"provider": "openai", "api_key": "k"}))
}

// A model namespaced for another provider is refused before the call: nothing translates it for OpenAI.
func TestOpenAIRefusesNamespacedModelsBeforeTheCall(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: decision("other", map[string]float64{"refund": 0.1, "exchange": 0.1, "other": 0.8})})
	n := openaiNode(t, srv)
	for _, model := range []string{"anthropic/claude-3.5-sonnet", "google/gemini-2.0-flash-001", "openai/gpt-4o-mini-2024-07-18"} {
		in := aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, map[string]any{"model": model}), nil)
		_, err := n.Execute(context.Background(), in)
		require.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code, model)
	}
	require.Empty(t, srv.Requests())
}

// The fake's handler may ask the server for its requests.
func TestFakeHandlerMayReadTheRequests(t *testing.T) {
	srv := aitest.New(t)
	srv.Handler = func(aitest.Request) aitest.Reply {
		return aitest.Reply{Body: aitest.Chat(fmt.Sprint(len(srv.Requests())), 1, 1)}
	}
	n := openaiNode(t, srv)
	in := aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, nil), nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = n.Execute(context.Background(), in)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the handler deadlocked on the server's lock")
	}
	require.Len(t, srv.Requests(), 1)
}
