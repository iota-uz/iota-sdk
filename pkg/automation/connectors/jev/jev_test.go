package jev_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
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
	require.True(t, errors.As(err, &ce), "want a CallError, got %v", err)
	return ce
}

func decision(choice string, probs map[string]float64) string {
	b, _ := json.Marshal(map[string]any{"choice": choice, "probabilities": probs})
	return aitest.Chat(string(b), 120, 18)
}

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
	assert.Empty(t, parse(`{"input":"{}","choices":`+two+`,"min_confidence":0.9,"model":"gpt-4o-2024-08-06","timeout":"90s"}`))

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
		"alias":              `{"input":"{}","choices":` + two + `,"model":"gpt-4o-latest"}`,
		"timeout":            `{"input":"{}","choices":` + two + `,"timeout":"10m"}`,
		"empty prompt":       `{"input":"{}","choices":` + two + `,"prompt":""}`,
		"unknown":            `{"input":"{}","choices":` + two + `,"images":"x"}`,
	} {
		assert.NotEmpty(t, parse(raw), name)
	}
}

func TestDefaultsAreAppliedAndPinned(t *testing.T) {
	n, _ := newNode(t, jev.WithDefaultModel("openai/gpt-4o-mini-2024-07-18"))
	p := parsed(t, n, map[string]any{"input": "{}", "choices": choices(t)})
	assert.Equal(t, json.Number("0.6"), p["min_confidence"], "min_confidence defaults to 0.6")
	assert.Equal(t, jev.DefaultPrompt, p["prompt"])
	assert.Contains(t, jev.DefaultPrompt, "When in doubt, choose the safest option")
	assert.Equal(t, "60s", p["timeout"])

	cn := n.(catalog.CompletedNode)
	raw := map[string]any{"input": "{}", "choices": choices(t)}
	done := cn.CompleteParams(raw)
	assert.Equal(t, "openai/gpt-4o-mini-2024-07-18", done["model"])
	assert.Equal(t, jev.DefaultPrompt, done["prompt"])
	assert.NotContains(t, raw, "model")
	assert.NotContains(t, raw, "prompt")
	assert.Equal(t, done, cn.CompleteParams(done))
	kept := cn.CompleteParams(map[string]any{"model": "gpt-4o", "prompt": "my rules"})
	assert.Equal(t, "gpt-4o", kept["model"])
	assert.Equal(t, "my rules", kept["prompt"])
	assert.Equal(t, "", cn.CompleteParams(map[string]any{"prompt": ""})["prompt"], "an empty prompt is refused, not replaced")
}

func TestDefaultModelOfAConfiguration(t *testing.T) {
	assert.Equal(t, "openai/gpt-4o-mini", jev.DefaultModel(nil))
	assert.Equal(t, "openai/gpt-4o-mini", jev.DefaultModel(&jevconfig.Config{}))
	assert.Equal(t, "gpt-4o-mini", jev.DefaultModel(&jevconfig.Config{Provider: "openai"}))
	assert.Equal(t, "anthropic/claude-3.5-haiku", jev.DefaultModel(&jevconfig.Config{Provider: "openai", DefaultModel: " anthropic/claude-3.5-haiku "}))
}

func TestDecisionAndOutcomes(t *testing.T) {
	probs := map[string]float64{"refund": 0.7, "exchange": 0.2, "other": 0.1}
	srv := aitest.New(t, aitest.Reply{Body: decision("refund", probs)})
	n, _ := newNode(t, jev.WithTransport(srv.Transport()), jev.WithDefaultModel("openai/gpt-4o-mini"))
	usage := &aitest.Usage{}

	in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), usage)
	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "refund", res.Outcome, "0.7 is over the default 0.6")
	assert.Equal(t, "refund", res.Output["choice"])
	assert.InDelta(t, 0.7, res.Output["confidence"], 1e-9)
	assert.Equal(t, "openrouter", res.Output["provider"])
	assert.Equal(t, "test-model-2025-01-01", res.Output["model"])
	got := res.Output["probabilities"].(map[string]any)
	assert.InDelta(t, 0.7, got["refund"], 1e-9)
	assert.Len(t, got, 3)
	assert.Equal(t, map[string]any{"input_tokens": int64(120), "output_tokens": int64(18), "cost_usd": nil}, res.Output["usage"])

	assert.EqualValues(t, 120, usage.Total("token_in"))
	assert.EqualValues(t, 18, usage.Total("token_out"))
	for _, e := range usage.Entries {
		assert.Equal(t, jev.Kind, e.Kind)
		assert.Equal(t, in.Connection.ID, e.ConnectionID)
	}

	// MinConfidence 0.9 and a confidence of 0.7: low_confidence, and the choice is still in the output.
	p := base(t, n, map[string]any{"min_confidence": 0.9})
	res, err = n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), p, nil))
	require.NoError(t, err)
	assert.Equal(t, "low_confidence", res.Outcome)
	assert.Equal(t, "refund", res.Output["choice"])
	assert.InDelta(t, 0.7, res.Output["confidence"], 1e-9)

	// At the floor exactly the choice is taken.
	p = base(t, n, map[string]any{"min_confidence": 0.7})
	res, err = n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), p, nil))
	require.NoError(t, err)
	assert.Equal(t, "refund", res.Outcome)
}

func TestTheEngineTakesTheOutcomeOfTheAnswer(t *testing.T) {
	// The outcome the engine derives from the output (graph.Interpreter.CallOutcome) is the node's own.
	v := aitest.View(t, jev.New())
	doc := aitest.Graph("ai.jev", `{"input":"{'title': event.title}","choices":`+choicesJSON+`,"min_confidence":0.9,"model":"gpt-4o"}`,
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

func TestRequestShape(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: decision("other", map[string]float64{"refund": 0.1, "exchange": 0.1, "other": 0.8})})
	n, _ := newNode(t, jev.WithTransport(srv.Transport()))
	_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, map[string]any{"prompt": "Decide for {{ event.title }}"}), nil))
	require.NoError(t, err)

	req := srv.Requests()[0]
	assert.Equal(t, "openrouter.ai", req.Host())
	assert.Equal(t, "/api/v1/chat/completions", req.Path)
	body := req.JSON(t)
	assert.Equal(t, "openai/gpt-4o-mini", body["model"])
	assert.EqualValues(t, 0, body["temperature"])
	assert.Equal(t, map[string]any{"include": true}, body["usage"], "OpenRouter reports the cost")
	assert.Equal(t, map[string]any{"require_parameters": true}, body["provider"])
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

func TestOpenAIProvider(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: decision("other", map[string]float64{"refund": 0.1, "exchange": 0.1, "other": 0.8})})
	n, _ := newNode(t, jev.WithTransport(srv.Transport()))
	res, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, nil), nil))
	require.NoError(t, err)
	assert.Equal(t, "openai", res.Output["provider"])
	req := srv.Requests()[0]
	assert.Equal(t, "api.openai.com", req.Host())
	body := req.JSON(t)
	assert.Equal(t, "gpt-4o-mini", body["model"], "the vendor prefix of an OpenRouter id is dropped")
	assert.Contains(t, body, "max_completion_tokens")
	assert.NotContains(t, body, "usage", "OpenAI takes no usage option")
	assert.NotContains(t, body, "provider")
}

func TestReasoningModelsGetNoTemperature(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: decision("other", map[string]float64{"refund": 0, "exchange": 0, "other": 1})})
	n, _ := newNode(t, jev.WithTransport(srv.Transport()))
	_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openai"), base(t, n, map[string]any{"model": "o3-2025-04-16"}), nil))
	require.NoError(t, err)
	assert.NotContains(t, srv.Requests()[0].JSON(t), "temperature")
}

func TestProbabilitiesAreNormalized(t *testing.T) {
	// Percentages, or numbers that do not sum to 1, become probabilities of one.
	srv := aitest.New(t, aitest.Reply{Body: decision("exchange", map[string]float64{"refund": 10, "exchange": 80, "other": 10})})
	n, _ := newNode(t, jev.WithTransport(srv.Transport()))
	res, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), nil))
	require.NoError(t, err)
	assert.Equal(t, "exchange", res.Outcome)
	var sum float64
	for _, v := range res.Output["probabilities"].(map[string]any) {
		sum += v.(float64)
	}
	assert.InDelta(t, 1, sum, 1e-9)
	assert.InDelta(t, 0.8, res.Output["confidence"], 1e-9)
}

func TestAnswersOutsideTheChoicesAreOneInvalidRequest(t *testing.T) {
	good := map[string]float64{"refund": 0.5, "exchange": 0.3, "other": 0.2}
	for name, body := range map[string]string{
		"unknown choice":      decision("complaint", good),
		"empty choice":        decision("", good),
		"missing probability": decision("refund", map[string]float64{"refund": 0.5, "exchange": 0.5}),
		"extra probability":   decision("refund", map[string]float64{"refund": 0.5, "exchange": 0.2, "other": 0.2, "x": 0.1}),
		"negative":            decision("refund", map[string]float64{"refund": 1.5, "exchange": -0.3, "other": -0.2}),
		"all zero":            decision("refund", map[string]float64{"refund": 0, "exchange": 0, "other": 0}),
		"not json":            aitest.Chat("refund", 10, 1),
		"not an object":       aitest.Chat(`["refund"]`, 10, 1),
		"extra field":         aitest.Chat(`{"choice":"refund","probabilities":{"refund":1,"exchange":0,"other":0},"why":"x"}`, 10, 1),
		"probabilities text":  aitest.Chat(`{"choice":"refund","probabilities":{"refund":"high","exchange":0,"other":0}}`, 10, 1),
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: body})
			n, _ := newNode(t, jev.WithTransport(srv.Transport()))
			usage := &aitest.Usage{}
			_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), usage))
			ce := callErr(t, err)
			assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
			assert.False(t, ce.Retryable)
			assert.Len(t, srv.Requests(), 1, "exactly one request")
			assert.Positive(t, usage.Total("token_in"), "the tokens were spent")
		})
	}
}

func TestProviderFailures(t *testing.T) {
	cases := []struct {
		name      string
		reply     aitest.Reply
		code      string
		retryable bool
	}{
		{"unauthorized", aitest.Reply{Status: 401, Body: aitest.APIError("No auth credentials found", "", "401")}, catalog.CallUnauthorized, false},
		{"rate limited", aitest.Reply{Status: 429, Body: aitest.APIError("slow", "", "429")}, catalog.CallRateLimited, true},
		{"unavailable", aitest.Reply{Status: 502, Body: `bad gateway`}, catalog.CallUnavailable, true},
		{"no credit", aitest.Reply{Status: 402, Body: aitest.APIError("Insufficient credits", "", "402")}, catalog.CallUnavailable, false},
		{"context", aitest.Reply{Status: 400, Body: aitest.APIError("too long", "invalid_request_error", "context_length_exceeded")}, catalog.CallInvalidRequest, false},
		{"refusal", aitest.Reply{Body: `{"choices":[{"finish_reason":"stop","message":{"content":null,"refusal":"no"}}],"usage":{"prompt_tokens":4,"completion_tokens":1}}`}, catalog.CallRejected, false},
		{"cut off", aitest.Reply{Body: `{"choices":[{"finish_reason":"length","message":{"content":"{\"choice\":"}}],"usage":{"prompt_tokens":4,"completion_tokens":4000}}`}, catalog.CallInvalidRequest, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := aitest.New(t, tc.reply)
			n, _ := newNode(t, jev.WithTransport(srv.Transport()))
			_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), &aitest.Usage{}))
			ce := callErr(t, err)
			assert.Equal(t, tc.code, ce.Code)
			assert.Equal(t, tc.retryable, ce.Retryable)
			assert.Len(t, srv.Requests(), 1)
		})
	}
}

func TestRetryAfterIsGivenToTheEngine(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Status: 429, Header: map[string]string{"Retry-After": "3"}, Body: `{}`})
	n, _ := newNode(t, jev.WithTransport(srv.Transport()))
	usage := &aitest.Usage{}
	res, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), usage))
	require.NoError(t, err)
	assert.EqualValues(t, 3e9, res.RetryAfter)
	assert.Empty(t, usage.Entries)
}

func TestBadCallsAreRefusedBeforeAnyRequest(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: decision("refund", map[string]float64{"refund": 1, "exchange": 0, "other": 0})})
	n, _ := newNode(t, jev.WithTransport(srv.Transport()))

	notMap := base(t, n, nil)
	notMap["input"] = "just a string"
	_, err := n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), notMap, nil))
	ce := callErr(t, err)
	assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
	assert.Contains(t, ce.Message, "map")

	list := base(t, n, nil)
	list["input"] = []any{"a"}
	_, err = n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), list, nil))
	assert.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code)

	blank := base(t, n, nil)
	blank["prompt"] = " \n"
	_, err = n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), blank, nil))
	assert.Contains(t, callErr(t, err).Message, "prompt")

	_, err = n.Execute(context.Background(), aitest.Input(jev.Kind, srv, map[string]string{"provider": "openrouter", "api_key": ""}, base(t, n, nil), nil))
	assert.Equal(t, catalog.CallUnauthorized, callErr(t, err).Code)

	_, err = n.Execute(context.Background(), aitest.Input(jev.Kind, srv, map[string]string{"provider": "anthropic"}, base(t, n, nil), nil))
	assert.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code)

	reserved := base(t, n, nil)
	reserved["choices"] = []any{
		map[string]any{"value": "failed", "description": "x"}, map[string]any{"value": "b", "description": "y"},
	}
	_, err = n.Execute(context.Background(), aitest.Input(jev.Kind, srv, creds("openrouter"), reserved, nil))
	assert.Equal(t, catalog.CallInvalidRequest, callErr(t, err).Code)
	assert.Empty(t, srv.Requests())
}

func TestUnauthorizedMarksTheConnection(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Status: 401, Body: `{}`})
	set := &statuses{}
	n, _ := newNode(t, jev.WithTransport(srv.Transport()), jev.WithConnectionStatus(set))
	in := aitest.Input(jev.Kind, srv, creds("openrouter"), base(t, n, nil), nil)
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
		return aitest.Graph("ai.jev", `{"input":"`+input+`","choices":`+choicesJSON+`,"model":"gpt-4o"}`, outcomes)
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
			`{"id":"n1","type":"ai.jev","params":{"input":"{'a': 1}","choices":`+choicesJSON+`,"model":"gpt-4o"}},{"id":"check","type":"branch","params":{"expression":"`+expression+`"}}`,
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
	assert.Equal(t, k, key(map[string]any{"model": "openai/gpt-4o-mini", "min_confidence": 0.6}, nil), "defaults are the same call")
	for name, other := range map[string]string{
		"prompt":         key(map[string]any{"prompt": "other rules"}, nil),
		"model":          key(map[string]any{"model": "openai/gpt-4o"}, nil),
		"min confidence": key(map[string]any{"min_confidence": 0.9}, nil),
		"input":          key(nil, func(p map[string]any) { p["input"] = map[string]any{"title": "other"} }),
		"input value":    key(nil, func(p map[string]any) { p["input"].(map[string]any)["count"] = json.Number("3") }),
		"choices": key(nil, func(p map[string]any) {
			p["choices"].([]any)[0].(map[string]any)["description"] = "changed"
		}),
	} {
		assert.NotEqual(t, k, other, name)
	}
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
	assert.Error(t, err)
	_, err = c.Test(context.Background(), connectors.Credentials{"provider": "openai"})
	assert.Error(t, err)
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
	assert.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"provider": "openrouter", "api_key": "k"}))
	assert.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"provider": "openai", "api_key": "k"}))
	assert.Error(t, connectors.ValidateCredentials(d, connectors.Credentials{"provider": "gemini", "api_key": "k"}))
	assert.Error(t, connectors.ValidateCredentials(d, connectors.Credentials{"api_key": "k"}))
	assert.Equal(t, map[string]string{"provider": "openai"}, connectors.RedactedCredentials(d, connectors.Credentials{"provider": "openai", "api_key": "k"}))
}
