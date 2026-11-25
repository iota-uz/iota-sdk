package openai_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/openai"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/openaiconfig"
)

const answerSchema = `{"type":"object","properties":{"summary":{"type":"string"},"score":{"type":"integer"},"tags":{"type":"array","items":{"type":"string"}},"note":{"type":"string"},"detail":{"type":"object","properties":{"x":{"type":"integer"},"y":{"type":"string"}},"required":["x"]}},"required":["summary","score"]}`

func schemaParam(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(answerSchema), &m))
	return m
}

func newNode(t *testing.T, opts ...openai.Option) (catalog.CallNode, connectors.Connector) {
	t.Helper()
	c := openai.New(opts...)
	require.Len(t, c.Nodes(), 1)
	n, ok := c.Nodes()[0].(catalog.CallNode)
	require.True(t, ok)
	return n, c
}

// parsed is the params of a call as the engine passes them: parsed by the node's spec (defaults applied).
func parsed(t *testing.T, n catalog.CallNode, raw map[string]any) map[string]any {
	t.Helper()
	p, issues := n.Spec().Params.Parse(raw)
	require.Empty(t, issues)
	return p
}

func callCode(t *testing.T, err error) *catalog.CallError {
	t.Helper()
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce, "want a CallError")
	return ce
}

func TestNodeSpecIsACatalogEntry(t *testing.T) {
	n, c := newNode(t)
	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterConnector(c))
	assert.Equal(t, catalog.KindOpenAI, c.Kind())
	assert.Equal(t, []catalog.ConnectorKind{openai.Kind}, n.Requires())
	assert.True(t, n.Idempotent())
	spec := n.Spec()
	assert.Equal(t, "ai.llm", spec.Type)
	assert.Equal(t, graph.BehaviorCall, spec.Behavior)
	assert.Equal(t, graph.DryRunExecute, spec.DryRun)
	assert.Equal(t, []string{"done", "failed"}, spec.Outcomes)
	assert.Equal(t, connectors.PlatformOptIn, c.Descriptor().Platform)
	assert.Equal(t, connectors.CategoryAI, c.Descriptor().Category)
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
	assert.Empty(t, parse(`{"prompt":"hi"}`))
	assert.Empty(t, parse(`{"prompt":"hi","model":"gpt-4o-2024-08-06","temperature":0.2,"max_tokens":32000,"timeout_ms":120000}`))
	assert.Empty(t, parse(`{"prompt":"hi","timeout_ms":1000}`))
	assert.Empty(t, parse(`{"prompt":"hi","format":"json","schema":`+answerSchema+`}`))

	for name, raw := range map[string]string{
		"no prompt":            `{}`,
		"empty prompt":         `{"prompt":""}`,
		"alias":                `{"prompt":"hi","model":"gpt-4o-latest"}`,
		"bare latest":          `{"prompt":"hi","model":"latest"}`,
		"router":               `{"prompt":"hi","model":"openrouter/auto"}`,
		"not an id":            `{"prompt":"hi","model":"gpt 4o"}`,
		"json without schema":  `{"prompt":"hi","format":"json"}`,
		"schema with text":     `{"prompt":"hi","schema":` + answerSchema + `}`,
		"schema out of subset": `{"prompt":"hi","format":"json","schema":{"type":"object","properties":{"a":{"type":"string","pattern":"x"}}}}`,
		"max tokens":           `{"prompt":"hi","max_tokens":32001}`,
		"zero tokens":          `{"prompt":"hi","max_tokens":0}`,
		"temperature":          `{"prompt":"hi","temperature":2.1}`,
		"timeout too short":    `{"prompt":"hi","timeout_ms":0}`,
		"timeout ms short":     `{"prompt":"hi","timeout_ms":999}`,
		"timeout too long":     `{"prompt":"hi","timeout_ms":300001}`,
		"timeout string":       `{"prompt":"hi","timeout":"60s"}`,
		"alias snapshotless":   `{"prompt":"hi","model":"gpt-4o-mini"}`,
		"openrouter rolling":   `{"prompt":"hi","model":"openai/gpt-4o"}`,
		"too many images":      `{"prompt":"hi","model":"gpt-4o-2024-08-06","images":["1","2","3","4","5","6","7","8","9"]}`,
		"format":               `{"prompt":"hi","format":"xml"}`,
		"unknown param":        `{"prompt":"hi","tools":[]}`,
	} {
		assert.NotEmpty(t, parse(raw), name)
	}
}

func TestModelAliasesAndVision(t *testing.T) {
	for _, alias := range []string{"gpt-4o-latest", "chatgpt-4o-latest", "latest", "claude-latest", "gpt-5-latest", "openrouter/auto", "openai/gpt-4o:latest"} {
		require.Error(t, aichat.CheckModel(alias), alias)
	}
	for _, exact := range []string{"gpt-4o-2024-08-06", "gpt-4o-mini-2024-07-18", "o3-2025-04-16", "anthropic/claude-3.5-sonnet", "meta-llama/llama-3-8b-instruct:free"} {
		require.NoError(t, aichat.CheckModel(exact), exact)
	}
	for model, want := range map[string]bool{
		"gpt-4o": true, "gpt-4o-mini-2024-07-18": true, "gpt-4.1": true, "gpt-5": true, "o3": true, "o4-mini": true,
		"openai/gpt-4o": true, "anthropic/claude-3.5-sonnet": true, "google/gemini-2.0-flash-001": true,
		"gpt-3.5-turbo-0125": false, "gpt-4": false, "o1-mini": false, "o3-mini": false, "text-embedding-3-small": false,
		"meta-llama/llama-3-8b-instruct": false, "anthropic/claude-2.1": false,
	} {
		assert.Equal(t, want, aichat.VisionModel(model), model)
	}
}

func TestImagesNeedAModelWithVision(t *testing.T) {
	n, _ := newNode(t)
	_, issues := n.Spec().Params.Parse(map[string]any{"prompt": "describe", "model": "gpt-3.5-turbo-0125", "images": "event.upload_ids"})
	require.Len(t, issues, 1)
	assert.Equal(t, openai.ErrorModelNoVision, issues[0].Code)
	assert.Equal(t, []any{"images"}, issues[0].Path)

	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "describe", "model": "gpt-4o-2024-08-06", "images": "event.upload_ids"})
	assert.Empty(t, issues)
	// An empty images param is no images.
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "describe", "model": "gpt-3.5-turbo-0125", "images": "[]"})
	assert.Empty(t, issues)

	// The default model is checked when the node names none.
	n2, _ := newNode(t, openai.WithDefaultModel("gpt-3.5-turbo-0125"))
	_, issues = n2.Spec().Params.Parse(map[string]any{"prompt": "describe", "images": "event.upload_ids"})
	require.Len(t, issues, 1)
	assert.Equal(t, openai.ErrorModelNoVision, issues[0].Code)
}

func TestCompleteParamsPinsTheDefaultModel(t *testing.T) {
	n, _ := newNode(t, openai.WithDefaultModel("gpt-4o-mini-2024-07-18"))
	cn := n.(catalog.CompletedNode)

	raw := map[string]any{"prompt": "hi"}
	done := cn.CompleteParams(raw)
	assert.Equal(t, "gpt-4o-mini-2024-07-18", done["model"])
	assert.NotContains(t, raw, "model", "the argument is not changed")
	assert.Equal(t, done, cn.CompleteParams(done), "idempotent")

	assert.Equal(t, "gpt-4o-2024-08-06", cn.CompleteParams(map[string]any{"prompt": "hi", "model": "gpt-4o-2024-08-06"})["model"])
	assert.Equal(t, "gpt-4o-mini-2024-07-18", cn.CompleteParams(map[string]any{"prompt": "hi", "model": ""})["model"])
	assert.Equal(t, "gpt-4o-mini-2024-07-18", cn.CompleteParams(map[string]any{"prompt": "hi", "model": nil})["model"])
	assert.Equal(t, 5, cn.CompleteParams(map[string]any{"model": 5})["model"], "a value of another type is left to the check")
	assert.NotNil(t, cn.CompleteParams(nil))

	// A default that is an alias does not pass the check of the node it was written into.
	bad, _ := newNode(t, openai.WithDefaultModel("gpt-4o-latest"))
	_, issues := bad.Spec().Params.Parse(bad.(catalog.CompletedNode).CompleteParams(map[string]any{"prompt": "hi"}))
	assert.NotEmpty(t, issues)
}

func TestTextCall(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("Hello there", 12, 3)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()), openai.WithDefaultModel("gpt-4o-mini-2024-07-18"))
	usage := &aitest.Usage{}
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "Say hello", "system": "Be brief", "temperature": 0.5}), usage)

	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "done", res.Outcome)
	assert.Equal(t, "Hello there", res.Output["text"])
	assert.NotContains(t, res.Output, "json")
	assert.Equal(t, "test-model-2025-01-01", res.Output["model"])
	assert.Equal(t, map[string]any{"input_tokens": int64(12), "output_tokens": int64(3), "cost_usd": nil}, res.Output["usage"])
	assert.IsType(t, int64(0), res.Output["latency_ms"])

	reqs := srv.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "/chat/completions", reqs[0].Path)
	assert.Equal(t, "Bearer sk-test-key-123456", reqs[0].Header.Get("Authorization"))
	body := reqs[0].JSON(t)
	assert.Equal(t, "gpt-4o-mini-2024-07-18", body["model"], "the default model of the connector")
	assert.EqualValues(t, 2000, body["max_tokens"])
	assert.InDelta(t, 0.5, body["temperature"], 1e-9)
	assert.NotContains(t, body, "response_format")
	assert.Equal(t, []any{
		map[string]any{"role": "system", "content": "Be brief"},
		map[string]any{"role": "user", "content": "Say hello"},
	}, body["messages"])

	// Usage: two entries on the connection of the call, the tenant's own here.
	assert.EqualValues(t, 12, usage.Total("token_in"))
	assert.EqualValues(t, 3, usage.Total("token_out"))
	for _, e := range usage.Entries {
		assert.Equal(t, in.TenantID, e.TenantID)
		assert.Equal(t, openai.Kind, e.Kind)
		assert.Equal(t, in.Connection.ID, e.ConnectionID)
		require.NotNil(t, e.RunID)
		assert.Equal(t, in.RunID, *e.RunID)
	}
}

func TestOpenAIsOwnEndpointTakesMaxCompletionTokens(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("ok", 1, 1)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	in := aitest.Input(openai.Kind, srv, map[string]string{"base_url": "https://api.openai.com/v1"}, parsed(t, n, map[string]any{"prompt": "x"}), nil)
	_, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	body := srv.Requests()[0].JSON(t)
	assert.EqualValues(t, 2000, body["max_completion_tokens"])
	assert.NotContains(t, body, "max_tokens")
}

func TestPlatformKeyIsUsedAndAccounted(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("ok", 5, 2)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	usage := &aitest.Usage{}
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), usage)
	in.Connection = &catalog.ConnectionInfo{ID: catalog.PlatformConnectionID(openai.Kind), Kind: openai.Kind, Name: "platform", Platform: true}
	_, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, usage.Entries, 2)
	for _, e := range usage.Entries {
		assert.Equal(t, "platform:openai", e.ConnectionID)
	}
}

func TestJSONCall(t *testing.T) {
	answer := `{"summary":"fine","score":4,"tags":["a","b"],"note":null}`
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(answer, 20, 9)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x", "format": "json", "schema": schemaParam(t)}), &aitest.Usage{})

	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "done", res.Outcome)
	assert.NotContains(t, res.Output, "text")
	// note was optional and came back null: it is left out, as the declared output has it.
	raw, err := json.Marshal(res.Output["json"])
	require.NoError(t, err)
	assert.JSONEq(t, `{"summary":"fine","score":4,"tags":["a","b"]}`, string(raw))

	body := srv.Requests()[0].JSON(t)
	rf := body["response_format"].(map[string]any)
	assert.Equal(t, "json_schema", rf["type"])
	js := rf["json_schema"].(map[string]any)
	assert.Equal(t, true, js["strict"])
	strict := js["schema"].(map[string]any)
	assert.Equal(t, false, strict["additionalProperties"])
	assert.ElementsMatch(t, []any{"detail", "note", "score", "summary", "tags"}, strict["required"], "strict mode requires every property")
	note := strict["properties"].(map[string]any)["note"].(map[string]any)
	assert.Equal(t, []any{"string", "null"}, note["type"], "an optional property is nullable")
}

func TestJSONCallOfAnArray(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(`{"value":["x","y"]}`, 1, 1)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	schema := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x", "format": "json", "schema": schema}), nil)
	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, []any{"x", "y"}, res.Output["json"])
	js := srv.Requests()[0].JSON(t)["response_format"].(map[string]any)["json_schema"].(map[string]any)["schema"].(map[string]any)
	assert.Equal(t, "object", js["type"], "strict mode wants an object at the root")
}

func TestAnswerOutsideTheSchemaIsOneInvalidRequest(t *testing.T) {
	for name, answer := range map[string]string{
		"missing field": `{"summary":"fine"}`,
		"wrong type":    `{"summary":"fine","score":"high"}`,
		"extra field":   `{"summary":"fine","score":1,"extra":true}`,
		"not json":      `Sure! {"summary":"fine","score":1}`,
		"trailing text": `{"summary":"fine","score":1} thanks`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(answer, 20, 9)})
			n, _ := newNode(t, openai.WithTransport(srv.Transport()))
			usage := &aitest.Usage{}
			in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x", "format": "json", "schema": schemaParam(t)}), usage)
			_, err := n.Execute(context.Background(), in)
			ce := callCode(t, err)
			assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
			assert.False(t, ce.Retryable, "the engine does not repeat a client refusal")
			assert.Len(t, srv.Requests(), 1, "exactly one request")
			assert.EqualValues(t, 20, usage.Total("token_in"), "the tokens were spent")
		})
	}
}

func TestCutOffJSONIsInvalid(t *testing.T) {
	body := `{"model":"m","choices":[{"finish_reason":"length","message":{"content":"{\"summary\":\"fi"}}],"usage":{"prompt_tokens":3,"completion_tokens":2000}}`
	srv := aitest.New(t, aitest.Reply{Body: body})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x", "format": "json", "schema": schemaParam(t)}), nil)
	_, err := n.Execute(context.Background(), in)
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, err).Code)
}

func TestProviderFailures(t *testing.T) {
	cases := []struct {
		name      string
		reply     aitest.Reply
		code      string
		retryable bool
		message   string
	}{
		{"unauthorized", aitest.Reply{Status: 401, Body: aitest.APIError("Incorrect API key provided: sk-test-key-123456", "invalid_request_error", "invalid_api_key")}, catalog.CallUnauthorized, false, ""},
		{"forbidden", aitest.Reply{Status: 403, Body: `{}`}, catalog.CallUnauthorized, false, ""},
		{"rate limited", aitest.Reply{Status: 429, Body: aitest.APIError("slow down", "rate_limit_exceeded", "rate_limit_exceeded")}, catalog.CallRateLimited, true, "slow down"},
		{"quota", aitest.Reply{Status: 429, Body: aitest.APIError("You exceeded your current quota", "insufficient_quota", "insufficient_quota")}, catalog.CallUnavailable, false, "quota"},
		{"server error", aitest.Reply{Status: 503, Body: `upstream down`}, catalog.CallUnavailable, true, ""},
		{"bad gateway", aitest.Reply{Status: 502, Body: ``}, catalog.CallUnavailable, true, ""},
		{"context", aitest.Reply{Status: 400, Body: aitest.APIError("maximum context length is 128000", "invalid_request_error", "context_length_exceeded")}, catalog.CallInvalidRequest, false, "context"},
		{"no such model", aitest.Reply{Status: 404, Body: aitest.APIError("The model does not exist", "invalid_request_error", "model_not_found")}, catalog.CallInvalidRequest, false, "does not exist"},
		{"moderation", aitest.Reply{Status: 400, Body: aitest.APIError("blocked", "invalid_request_error", "content_policy_violation")}, catalog.CallRejected, false, ""},
		{"refusal", aitest.Reply{Body: `{"choices":[{"finish_reason":"stop","message":{"content":null,"refusal":"I can't help with that"}}],"usage":{"prompt_tokens":7,"completion_tokens":5}}`}, catalog.CallRejected, false, "can't help"},
		{"content filter", aitest.Reply{Body: `{"choices":[{"finish_reason":"content_filter","message":{"content":""}}],"usage":{"prompt_tokens":7,"completion_tokens":0}}`}, catalog.CallRejected, false, ""},
		{"no choices", aitest.Reply{Body: `{"choices":[]}`}, catalog.CallInvalidRequest, false, ""},
		{"not json", aitest.Reply{Body: `<html>`}, catalog.CallUnavailable, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := aitest.New(t, tc.reply)
			n, _ := newNode(t, openai.WithTransport(srv.Transport()))
			in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), &aitest.Usage{})
			res, err := n.Execute(context.Background(), in)
			if tc.reply.Status == 429 && tc.reply.Header["Retry-After"] != "" {
				require.NoError(t, err)
				assert.Positive(t, res.RetryAfter)
				return
			}
			ce := callCode(t, err)
			assert.Equal(t, tc.code, ce.Code)
			assert.Equal(t, tc.retryable, ce.Retryable)
			assert.Contains(t, ce.Message, tc.message)
			assert.NotContains(t, ce.Message, "sk-test-key-123456", "the key is never in a message")
			assert.Len(t, srv.Requests(), 1)
		})
	}
}

func TestRetryAfterIsGivenToTheEngine(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Status: 429, Header: map[string]string{"Retry-After": "7"}, Body: aitest.APIError("slow", "rate_limit_exceeded", "")})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	usage := &aitest.Usage{}
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), usage)
	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, 7*time.Second, res.RetryAfter)
	assert.Empty(t, res.Outcome)
	assert.Empty(t, usage.Entries, "a 429 spent nothing")
}

func TestRefusalIsStillAccounted(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: `{"choices":[{"finish_reason":"stop","message":{"content":null,"refusal":"no"}}],"usage":{"prompt_tokens":7,"completion_tokens":5}}`})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	usage := &aitest.Usage{}
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), usage)
	_, err := n.Execute(context.Background(), in)
	assert.Equal(t, catalog.CallRejected, callCode(t, err).Code)
	assert.EqualValues(t, 7, usage.Total("token_in"))
	assert.EqualValues(t, 5, usage.Total("token_out"))
}

func TestEveryRepeatIsAccounted(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("ok", 10, 4)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	usage := &aitest.Usage{}
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), usage)
	for attempt := 1; attempt <= 2; attempt++ {
		in.Attempt = attempt
		_, err := n.Execute(context.Background(), in)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 20, usage.Total("token_in"))
	assert.EqualValues(t, 8, usage.Total("token_out"))
	assert.Equal(t, 2, usage.Count("token_in"))
}

func TestUsageFailureDoesNotFailTheCall(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("ok", 10, 4)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), &aitest.Usage{Err: errors.New("db is down")})
	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "done", res.Outcome)
}

func TestCostIsReportedWhenTheProviderGivesIt(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: `{"model":"m","choices":[{"finish_reason":"stop","message":{"content":"ok"}}],"usage":{"prompt_tokens":3,"completion_tokens":1,"cost":0.00042}}`})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), nil)
	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.InDelta(t, 0.00042, res.Output["usage"].(map[string]any)["cost_usd"], 1e-12)
}

func TestBadCallsAreRefusedBeforeAnyRequest(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("ok", 1, 1)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	good := parsed(t, n, map[string]any{"prompt": "x"})

	blank := map[string]any{}
	for k, v := range good {
		blank[k] = v
	}
	blank["prompt"] = "  \n "
	_, err := n.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, blank, nil))
	ce := callCode(t, err)
	assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
	assert.Contains(t, ce.Message, "prompt")

	in := aitest.Input(openai.Kind, srv, map[string]string{"api_key": ""}, good, nil)
	_, err = n.Execute(context.Background(), in)
	assert.Equal(t, catalog.CallUnauthorized, callCode(t, err).Code)

	for _, base := range []string{"http://api.example.com/v1", "https://user:pw@api.example.com/v1", "https://api.example.com/v1?x=1"} {
		in = aitest.Input(openai.Kind, srv, map[string]string{"base_url": base}, good, nil)
		_, err = n.Execute(context.Background(), in)
		assert.Equal(t, catalog.CallInvalidRequest, callCode(t, err).Code, base)
	}
	assert.Empty(t, srv.Requests())
}

func TestUnauthorizedMarksTheConnection(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Status: 401, Body: `{}`})
	set := &statusLog{}
	n, _ := newNode(t, openai.WithTransport(srv.Transport()), openai.WithConnectionStatus(set))
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), nil)
	_, err := n.Execute(context.Background(), in)
	assert.Equal(t, catalog.CallUnauthorized, callCode(t, err).Code)
	require.Len(t, set.calls, 1)
	assert.Equal(t, connectors.StatusNeedsReauth, set.calls[0].status)
	assert.Equal(t, in.Connection.ID, set.calls[0].id.String())

	// The platform key is not a connection of the tenant.
	in.Connection = &catalog.ConnectionInfo{ID: "platform:openai", Kind: openai.Kind, Platform: true}
	_, _ = n.Execute(context.Background(), in)
	assert.Len(t, set.calls, 1)
}

type statusCall struct {
	id     uuid.UUID
	status connectors.ConnectionStatus
}
type statusLog struct{ calls []statusCall }

func (s *statusLog) SetStatus(_ context.Context, _, id uuid.UUID, st connectors.ConnectionStatus, _ string) error {
	s.calls = append(s.calls, statusCall{id, st})
	return nil
}

func TestTenantEndpointIsGuarded(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("ok", 1, 1)})
	// No transport override: a tenant's base_url on a loopback address is not connected to.
	n, _ := newNode(t)
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), nil)
	_, err := n.Execute(context.Background(), in)
	ce := callCode(t, err)
	assert.Equal(t, catalog.CallUnavailable, ce.Code)
	assert.Empty(t, srv.Requests())
}

func TestDeadlineEndsTheCall(t *testing.T) {
	srv := aitest.New(t)
	srv.Handler = func(aitest.Request) aitest.Reply {
		time.Sleep(300 * time.Millisecond)
		return aitest.Reply{Body: aitest.Chat("late", 1, 1)}
	}
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := n.Execute(ctx, in)
	ce := callCode(t, err)
	assert.Equal(t, catalog.CallUnavailable, ce.Code)
	assert.True(t, ce.Retryable)
}

// --- images ---

type images map[string]openai.Image

func (m images) Image(_ context.Context, _ uuid.UUID, id string, limit int64) (openai.Image, error) {
	img, ok := m[id]
	if !ok {
		return openai.Image{}, openai.ErrImageNotFound
	}
	if int64(len(img.Data)) > limit {
		return openai.Image{}, openai.ErrImageTooLarge
	}
	return img, nil
}

var jpeg = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")

var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")

func TestImagesReachTheProvider(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("a cat", 100, 3)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()), openai.WithImages(images{
		"11": {Data: png}, "12": {MediaType: "image/jpeg; charset=binary", Data: jpeg},
	}))
	params := parsed(t, n, map[string]any{"prompt": "what is it?", "model": "gpt-4o-2024-08-06", "images": "event.images"})
	params["images"] = []any{"11", json.Number("12")} // the engine renders the expression to its value
	in := aitest.Input(openai.Kind, srv, nil, params, nil)

	res, err := n.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "a cat", res.Output["text"])

	body := srv.Requests()[0].JSON(t)
	msgs := body["messages"].([]any)
	require.Len(t, msgs, 1)
	parts := msgs[0].(map[string]any)["content"].([]any)
	require.Len(t, parts, 3)
	assert.Equal(t, map[string]any{"type": "text", "text": "what is it?"}, parts[0])
	first := parts[1].(map[string]any)
	assert.Equal(t, "image_url", first["type"])
	assert.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(png), first["image_url"].(map[string]any)["url"])
	second := parts[2].(map[string]any)["image_url"].(map[string]any)["url"].(string)
	assert.True(t, strings.HasPrefix(second, "data:image/jpeg;base64,"), second)
}

func TestImageFailures(t *testing.T) {
	big := make([]byte, openai.MaxImageBytes+1)
	copy(big, png)
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("ok", 1, 1)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()), openai.WithImages(images{"1": {Data: png}, "big": {Data: big}, "text": {Data: []byte("plain text")}, "mislabeled": {MediaType: "image/jpeg", Data: png}}))

	run := func(model string, ids ...any) error {
		params := parsed(t, n, map[string]any{"prompt": "x", "model": model})
		params["images"] = ids
		_, err := n.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, params, nil))
		return err
	}
	assert.Contains(t, callCode(t, run("gpt-4o-2024-08-06", "missing")).Message, "not found")
	assert.Contains(t, callCode(t, run("gpt-4o-2024-08-06", "big")).Message, "5 MB")
	assert.Contains(t, callCode(t, run("gpt-4o-2024-08-06", "text")).Message, "PNG, JPEG")
	assert.Contains(t, callCode(t, run("gpt-4o-2024-08-06", "mislabeled")).Message, "labelled image/jpeg but is image/png")
	assert.Contains(t, callCode(t, run("gpt-4o-2024-08-06", 1, 2, 3, 4, 5, 6, 7, 8, 9)).Message, "at most 8")
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run("gpt-3.5-turbo-0125", "1")).Code, "a model without vision, whatever was saved")
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run("gpt-4o-2024-08-06", true)).Code)
	assert.Empty(t, srv.Requests())

	// Without a source the node has no images to give.
	n0, _ := newNode(t, openai.WithTransport(srv.Transport()))
	params := parsed(t, n0, map[string]any{"prompt": "x", "model": "gpt-4o-2024-08-06"})
	params["images"] = []any{"1"}
	_, err := n0.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, params, nil))
	assert.Contains(t, callCode(t, err).Message, "not available")

	// A failing source is the engine's to repeat.
	nf, _ := newNode(t, openai.WithTransport(srv.Transport()), openai.WithImages(failingImages{}))
	params = parsed(t, nf, map[string]any{"prompt": "x", "model": "gpt-4o-2024-08-06"})
	params["images"] = []any{"1"}
	_, err = nf.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, params, nil))
	ce := callCode(t, err)
	assert.Equal(t, catalog.CallUnavailable, ce.Code)
	assert.True(t, ce.Retryable)
}

type failingImages struct{}

func (failingImages) Image(context.Context, uuid.UUID, string, int64) (openai.Image, error) {
	return openai.Image{}, errors.New("storage is down")
}

// --- decision keys ---

func TestDecisionKeyFollowsPromptModelAndParams(t *testing.T) {
	n, _ := newNode(t, openai.WithDefaultModel("gpt-4o-mini-2024-07-18"))
	dn := n.(engine.DecisionNode)
	key := func(raw map[string]any, images ...string) string {
		p := parsed(t, n, raw)
		if len(images) > 0 {
			list := make([]any, len(images))
			for i, id := range images {
				list[i] = id
			}
			p["images"] = list
		}
		k, ok := dn.DecisionKey(catalog.CallInput{Params: p})
		require.True(t, ok)
		return k
	}
	base := key(map[string]any{"prompt": "classify this"})
	assert.Equal(t, base, key(map[string]any{"prompt": "classify this"}), "stable")
	assert.Equal(t, base, key(map[string]any{"prompt": "classify this", "model": "gpt-4o-mini-2024-07-18", "max_tokens": 2000}), "defaults are the same call")

	for name, other := range map[string]string{
		"prompt":      key(map[string]any{"prompt": "classify that"}),
		"system":      key(map[string]any{"prompt": "classify this", "system": "be strict"}),
		"model":       key(map[string]any{"prompt": "classify this", "model": "gpt-4o-2024-08-06"}),
		"max tokens":  key(map[string]any{"prompt": "classify this", "max_tokens": 100}),
		"temperature": key(map[string]any{"prompt": "classify this", "temperature": 0.3}),
		"format":      key(map[string]any{"prompt": "classify this", "format": "json", "schema": schemaParam(t)}),
		"images":      key(map[string]any{"prompt": "classify this", "model": "gpt-4o-2024-08-06"}, "7"),
	} {
		assert.NotEqual(t, base, other, name)
	}
	a := key(map[string]any{"prompt": "x", "model": "gpt-4o-2024-08-06"}, "7")
	b := key(map[string]any{"prompt": "x", "model": "gpt-4o-2024-08-06"}, "8")
	assert.NotEqual(t, a, b)

	_, ok := dn.DecisionKey(catalog.CallInput{Params: map[string]any{"model": "gpt-4o-latest", "prompt": "x"}})
	assert.False(t, ok, "a call that would be refused is not cached")
}

// --- the connector ---

func TestConnectorTest(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Header: map[string]string{"openai-organization": "org-acme", "openai-project": "proj_support"}, Body: `{"object":"list","data":[{"id":"gpt-4o-2024-08-06"}]}`})
	c := openai.New(openai.WithTransport(srv.Transport()))
	id, err := c.Test(context.Background(), connectors.Credentials{"api_key": "sk-test-key-123456", "base_url": srv.URL})
	require.NoError(t, err)
	assert.Equal(t, "proj_support", id.Display)
	assert.Equal(t, "org-acme", id.External)
	reqs := srv.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "GET", reqs[0].Method)
	assert.Equal(t, "/models", reqs[0].Path)
	assert.Equal(t, "Bearer sk-test-key-123456", reqs[0].Header.Get("Authorization"))

	bad := aitest.New(t, aitest.Reply{Status: 401, Body: aitest.APIError("Incorrect API key provided: sk-test-key-123456", "invalid_request_error", "invalid_api_key")})
	c = openai.New(openai.WithTransport(bad.Transport()))
	_, err = c.Test(context.Background(), connectors.Credentials{"api_key": "sk-test-key-123456", "base_url": bad.URL})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "sk-test-key-123456")

	_, err = c.Test(context.Background(), connectors.Credentials{"api_key": ""})
	require.Error(t, err)
	_, err = c.Test(context.Background(), connectors.Credentials{"api_key": "k", "base_url": "http://insecure.example.com"})
	require.Error(t, err)

	garbage := aitest.New(t, aitest.Reply{Body: `{"unexpected":true}`})
	c = openai.New(openai.WithTransport(garbage.Transport()))
	_, err = c.Test(context.Background(), connectors.Credentials{"api_key": "k", "base_url": garbage.URL})
	require.Error(t, err)
}

func TestPlatformCredentials(t *testing.T) {
	assert.Empty(t, openai.PlatformCredentials(nil))
	assert.Empty(t, openai.PlatformCredentials(&openaiconfig.Config{DefaultModel: "gpt-4o-2024-08-06"}))
	got := openai.PlatformCredentials(&openaiconfig.Config{APIKey: "sk-platform", BaseURL: "https://llm.example.com/v1"})
	require.Contains(t, got, catalog.ConnectorKind("openai"))
	assert.Equal(t, connectors.Credentials{"api_key": "sk-platform", "base_url": "https://llm.example.com/v1"}, got["openai"].Credentials)
	got = openai.PlatformCredentials(&openaiconfig.Config{APIKey: "sk-platform"})
	assert.Equal(t, connectors.Credentials{"api_key": "sk-platform"}, got["openai"].Credentials)
}

func TestCredentialsSchemaValidates(t *testing.T) {
	d := openai.New().Descriptor()
	require.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"api_key": "sk-1"}))
	require.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"api_key": "sk-1", "base_url": "https://llm.example.com/v1"}))
	require.Error(t, connectors.ValidateCredentials(d, connectors.Credentials{"api_key": "sk-1", "base_url": "http://llm.example.com"}))
	require.Error(t, connectors.ValidateCredentials(d, connectors.Credentials{"base_url": "https://x.example.com"}))
	assert.Equal(t, map[string]string{"base_url": "https://llm.example.com/v1"},
		connectors.RedactedCredentials(d, connectors.Credentials{"api_key": "sk-1", "base_url": "https://llm.example.com/v1"}))
}

func TestRollingIdsAreNotPinned(t *testing.T) {
	for _, rolling := range []string{"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-5", "o3", "o4-mini", "chatgpt-4o", "openai/gpt-4o-mini"} {
		err := aichat.CheckModel(rolling)
		require.Error(t, err, rolling)
		assert.Contains(t, err.Error(), "rolling", rolling)
	}
	for _, pinned := range []string{
		"gpt-4o-2024-08-06", "gpt-4o-mini-2024-07-18", "gpt-4.1-2025-04-14", "gpt-5-2025-08-07", "o3-2025-04-16", "gpt-4-0613",
		"openai/gpt-4o-mini-2024-07-18", "anthropic/claude-3.5-sonnet", "meta-llama/llama-3-8b-instruct", "typesafe/jev-1.14", "my-company-model",
	} {
		require.NoError(t, aichat.CheckModel(pinned), pinned)
	}
	require.Error(t, aichat.CheckModel("~typesafe/jev-latest"))
	assert.Equal(t, "gpt-4o-mini-2024-07-18", openaiconfig.DefaultModel, "the shipped default is pinned")
	require.NoError(t, aichat.CheckModel(openaiconfig.DefaultModel))
}

func TestVisionIsAVerifiedList(t *testing.T) {
	for model, want := range map[string]bool{
		"gpt-4o-2024-08-06": true, "gpt-4o-mini-2024-07-18": true, "gpt-4.1-2025-04-14": true, "gpt-5-2025-08-07": true, "o3-2025-04-16": true,
		"gpt-5.4-mini-2026-03-17": true, "gpt-5-mini-2025-08-07": true, "gpt-5-nano-2025-08-07": true, "gpt-5.1-2025-11-13": true, "gpt-4.1-mini-2025-04-14": true, "o3-pro-2025-06-10": true,
		"gpt-5-chat-latest": false, "gpt-5-codex": false,
		"gpt-4o-mini-transcribe-2025-03-20": false, "gpt-4o-audio-preview-2025-06-03": false, "gpt-4o-realtime-preview-2025-06-03": false,
		"gpt-4o-mini-tts-2025-03-20": false, "gpt-4o-search-preview-2025-03-11": false, "gpt-4o-mini-search-preview": false,
		"gpt-3.5-turbo-0125": false, "o3-mini-2025-01-31": false, "o1-mini-2024-09-12": false, "text-embedding-3-small": false,
		"openai/gpt-4o-2024-08-06": true, "anthropic/claude-3.5-sonnet": true, "anthropic/claude-sonnet-4": true, "anthropic/claude-3-haiku": true,
		"google/gemini-2.0-flash-001": true, "google/gemini-embedding-001": false, "meta-llama/llama-3-8b-instruct": false, "my-company-model": false,
	} {
		assert.Equal(t, want, aichat.VisionModel(model), model)
	}
	n, _ := newNode(t)
	_, issues := n.Spec().Params.Parse(map[string]any{"prompt": "x", "model": "gpt-4o-mini-transcribe-2025-03-20", "images": []any{"1"}})
	require.Len(t, issues, 1)
	assert.Equal(t, openai.ErrorModelNoVision, issues[0].Code)
	// An application with capability metadata of its own replaces the list.
	n2, _ := newNode(t, openai.WithVision(func(m string) bool { return m == "my-company-model" }))
	_, issues = n2.Spec().Params.Parse(map[string]any{"prompt": "x", "model": "my-company-model", "images": []any{"1"}})
	assert.Empty(t, issues)
}

func TestImagesAreAListOrAnExpression(t *testing.T) {
	n, _ := newNode(t)
	_, issues := n.Spec().Params.Parse(map[string]any{"prompt": "x", "model": "gpt-4o-2024-08-06", "images": []any{"11", "12"}})
	assert.Empty(t, issues, "a literal list of upload ids")
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "x", "model": "gpt-4o-2024-08-06", "images": "event.upload_ids"})
	assert.Empty(t, issues, "a CEL expression")
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "x", "model": "gpt-3.5-turbo-0125", "images": []any{"11"}})
	require.Len(t, issues, 1)
	assert.Equal(t, openai.ErrorModelNoVision, issues[0].Code)
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "x", "model": "gpt-3.5-turbo-0125", "images": []any{}})
	assert.Empty(t, issues, "an empty list is no images")
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "x", "model": "gpt-4o-2024-08-06", "images": []any{1}})
	assert.NotEmpty(t, issues, "ids are strings")

	// The literal list is what the node gets, and the provider gets the pictures.
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("a cat", 100, 3)})
	n, _ = newNode(t, openai.WithTransport(srv.Transport()), openai.WithImages(images{"11": {Data: png}}))
	p := parsed(t, n, map[string]any{"prompt": "what?", "model": "gpt-4o-2024-08-06", "images": []any{"11"}})
	_, err := n.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, p, nil))
	require.NoError(t, err)
	parts := srv.Requests()[0].JSON(t)["messages"].([]any)[0].(map[string]any)["content"].([]any)
	assert.Len(t, parts, 2)
}

func TestTimeoutMsIsTheEnginesDeadline(t *testing.T) {
	n, _ := newNode(t)
	p := parsed(t, n, map[string]any{"prompt": "x"})
	assert.Equal(t, json.Number("60000"), p["timeout_ms"])
	assert.NotContains(t, string(n.Spec().Params.JSONSchema()), "\"timeout\":")
}

func TestServerErrorKeepsRetryAfter(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Status: 503, Header: map[string]string{"Retry-After": "16"}, Body: `{}`})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	res, err := n.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), nil))
	require.Error(t, err)
	assert.Zero(t, res.RetryAfter)
	ce := callCode(t, err)
	assert.Equal(t, catalog.CallUnavailable, ce.Code, "the code survives the repeats")
	assert.True(t, ce.Retryable)
	assert.Equal(t, 16*time.Second, ce.RetryAfter, "the engine waits at least that long")
}

func TestStrictEnvelopeIsChecked(t *testing.T) {
	stringSchema := map[string]any{"type": "string"}
	for name, answer := range map[string]string{
		"bare string":     `"hello"`,
		"extra field":     `{"value":"hello","note":"x"}`,
		"wrong key":       `{"text":"hello"}`,
		"not an envelope": `["hello"]`,
		"wrong type":      `{"value":5}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(answer, 5, 1)})
			n, _ := newNode(t, openai.WithTransport(srv.Transport()))
			in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x", "format": "json", "schema": stringSchema}), nil)
			_, err := n.Execute(context.Background(), in)
			assert.Equal(t, catalog.CallInvalidRequest, callCode(t, err).Code)
			assert.Len(t, srv.Requests(), 1)
		})
	}
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(`{"value":"hello"}`, 5, 1)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	res, err := n.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x", "format": "json", "schema": stringSchema}), nil))
	require.NoError(t, err)
	assert.Equal(t, "hello", res.Output["json"])
}

func TestDecisionKeyFollowsTheEndpointAndTheConnection(t *testing.T) {
	n, _ := newNode(t)
	dn := n.(engine.DecisionNode)
	p := parsed(t, n, map[string]any{"prompt": "x"})
	key := func(base, conn string) string {
		in := catalog.CallInput{Params: p, Credentials: catalog.Credentials{"api_key": "k", "base_url": base}, Connection: &catalog.ConnectionInfo{ID: conn}}
		k, ok := dn.DecisionKey(in)
		require.True(t, ok)
		return k
	}
	a := key("https://llm-a.example.com/v1", "c1")
	assert.Equal(t, a, key("https://llm-a.example.com/v1/", "c1"))
	assert.NotEqual(t, a, key("https://llm-b.example.com/v1", "c1"), "another endpoint")
	assert.NotEqual(t, a, key("https://llm-a.example.com/v1", "c2"), "another connection")
	assert.NotEqual(t, key("", "c1"), a, "OpenAI's own endpoint")
	assert.NotContains(t, a, "sk-")
}

// A field the schema does not declare is refused even when it is null: only a declared optional property may
// be the placeholder null of strict mode.
func TestUndeclaredNullFieldIsRefused(t *testing.T) {
	for name, answer := range map[string]string{
		"undeclared null": `{"summary":"fine","score":4,"extra":null}`,
		"nested null":     `{"summary":"fine","score":4,"detail":{"x":1,"extra":null}}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(answer, 20, 9)})
			n, _ := newNode(t, openai.WithTransport(srv.Transport()))
			in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x", "format": "json", "schema": schemaParam(t)}), nil)
			_, err := n.Execute(context.Background(), in)
			assert.Equal(t, catalog.CallInvalidRequest, callCode(t, err).Code)
			assert.Len(t, srv.Requests(), 1)
		})
	}
	// A declared optional property that is null is dropped, as before.
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(`{"summary":"fine","score":4,"tags":null,"note":null}`, 20, 9)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()))
	res, err := n.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x", "format": "json", "schema": schemaParam(t)}), nil))
	require.NoError(t, err)
	raw, err := json.Marshal(res.Output["json"])
	require.NoError(t, err)
	assert.JSONEq(t, `{"summary":"fine","score":4}`, string(raw))
}

// A fraction is not an upload id and is not rounded to one; a whole number is.
func TestFractionalImageIDsAreRefused(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("ok", 1, 1)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()), openai.WithImages(images{"7": {Data: png}}))
	run := func(id any) error {
		p := parsed(t, n, map[string]any{"prompt": "x", "model": "gpt-4o-2024-08-06"})
		p["images"] = []any{id}
		_, err := n.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, p, nil))
		return err
	}
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run(7.9)).Code)
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run(json.Number("7.9"))).Code)
	// A float64 would round these to 7 or 8: the decimal text decides.
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run(json.Number("7.9999999999999999"))).Code)
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run(json.Number("7.0000000000000001"))).Code)
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run(json.Number("1e400"))).Code)
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run(json.Number("1e99999999999"))).Code)
	started := time.Now()
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run(json.Number("1e1000000"))).Code, "an exponent of a million digits is not expanded")
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, run(json.Number("1e-1000000"))).Code)
	assert.Less(t, time.Since(started), time.Second)
	assert.Empty(t, srv.Requests(), "nothing was sent for a fraction")
	for _, whole := range []any{"7", 7, int64(7), 7.0, json.Number("7"), json.Number("7.0"), json.Number("0.7e1")} {
		require.NoError(t, run(whole), "%v", whole)
	}
}

func TestSchemaDepthIsBoundedOnSave(t *testing.T) {
	nested := func(levels int) map[string]any {
		var s any = map[string]any{"type": "string"}
		for i := 0; i < levels; i++ {
			s = map[string]any{"type": "object", "properties": map[string]any{"a": s}, "required": []any{"a"}}
		}
		return s.(map[string]any)
	}
	n, _ := newNode(t)
	_, issues := n.Spec().Params.Parse(map[string]any{"prompt": "x", "format": "json", "schema": nested(10)})
	assert.Empty(t, issues, "ten levels")
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "x", "format": "json", "schema": nested(11)})
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "levels deep")
	// A schema that is not an object is wrapped, and the wrapper is a level.
	arrays := map[string]any{"type": "string"}
	var s any = arrays
	for i := 0; i < 10; i++ {
		s = map[string]any{"type": "array", "items": s}
	}
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "x", "format": "json", "schema": s})
	assert.NotEmpty(t, issues, "ten arrays and the wrapper are eleven")
}

func TestStructuredOutputNeedsASupportingModel(t *testing.T) {
	for model, no := range map[string]bool{
		"gpt-3.5-turbo-0125": true, "gpt-4-turbo-2024-04-09": true, "gpt-4-0613": true, "gpt-4o-2024-05-13": true, "o1-mini-2024-09-12": true,
		"gpt-4o-2024-08-06": false, "gpt-4o-mini-2024-07-18": false, "gpt-4.1-2025-04-14": false, "gpt-5-2025-08-07": false, "o3-2025-04-16": false,
		"my-company-model": false, "meta-llama/llama-3-8b-instruct": false,
	} {
		assert.Equal(t, no, aichat.NoStructuredOutput(model), model)
	}
	n, _ := newNode(t)
	_, issues := n.Spec().Params.Parse(map[string]any{"prompt": "x", "format": "json", "schema": schemaParam(t), "model": "gpt-4-turbo-2024-04-09"})
	require.Len(t, issues, 1)
	assert.Equal(t, openai.ErrorModelNoStructuredOutput, issues[0].Code)
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "x", "model": "gpt-4-turbo-2024-04-09"})
	assert.Empty(t, issues, "text needs no structured output")

	// A call saved before this check is refused before any request.
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(`{}`, 1, 1)})
	nn, _ := newNode(t, openai.WithTransport(srv.Transport()))
	p := parsed(t, nn, map[string]any{"prompt": "x", "format": "json", "schema": schemaParam(t)})
	p["model"] = "gpt-4-turbo-2024-04-09"
	_, err := nn.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, p, nil))
	assert.Equal(t, catalog.CallInvalidRequest, callCode(t, err).Code)
	assert.Empty(t, srv.Requests())
}

// A malformed field of an answer does not hide the usage it reports, and is not repeated.
func TestMalformedChatAnswerKeepsItsUsage(t *testing.T) {
	for name, body := range map[string]string{
		"finish reason": `{"usage":{"prompt_tokens":10,"completion_tokens":2},"choices":[{"finish_reason":false,"message":{"content":"x"}}]}`,
		"choices":       `{"usage":{"prompt_tokens":10,"completion_tokens":2},"choices":{"a":1}}`,
		"message":       `{"usage":{"prompt_tokens":10,"completion_tokens":2},"choices":[{"finish_reason":"stop","message":[]}]}`,
		"content":       `{"usage":{"prompt_tokens":10,"completion_tokens":2},"choices":[{"finish_reason":"stop","message":{"content":{"a":` + `1}}}]}`,
		"model":         `{"model":false,"usage":{"prompt_tokens":10,"completion_tokens":2},"choices":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: body})
			n, _ := newNode(t, openai.WithTransport(srv.Transport()))
			usage := &aitest.Usage{}
			in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), usage)
			_, err := n.Execute(context.Background(), in)
			ce := callCode(t, err)
			assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
			assert.False(t, ce.Retryable)
			assert.Len(t, srv.Requests(), 1)
			assert.EqualValues(t, 10, usage.Total("token_in"))
			assert.EqualValues(t, 2, usage.Total("token_out"))
		})
	}
}

type slowUsage struct{ deadline chan time.Duration }

func (u slowUsage) Record(ctx context.Context, _ catalog.UsageEntry) error {
	dl, ok := ctx.Deadline()
	if !ok {
		u.deadline <- -1
		return nil
	}
	u.deadline <- time.Until(dl)
	<-ctx.Done()
	return ctx.Err()
}

// The write of the usage has a bound of its own: a database that stalls does not hold the node.
func TestUsageWriteIsBounded(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("ok", 3, 1)})
	n, _ := newNode(t, openai.WithTransport(srv.Transport()), openai.WithLogger(slog.New(slog.DiscardHandler)))
	rec := slowUsage{deadline: make(chan time.Duration, 4)}
	in := aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), rec)
	ctx, cancel := context.WithCancel(context.Background())
	started := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = n.Execute(ctx, in)
	}()
	d := <-rec.deadline
	cancel() // the node's own context ending does not end the write
	assert.Positive(t, d, "the write has a deadline")
	assert.LessOrEqual(t, d, 6*time.Second)
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("the node did not return: the usage write is not bounded")
	}
	assert.Less(t, time.Since(started), 14*time.Second)
}

func TestEnumValuesAreCountedWithTheirNulls(t *testing.T) {
	values := func(n int) []any {
		out := make([]any, n)
		for i := range out {
			out[i] = fmt.Sprintf("v%d", i)
		}
		return out
	}
	schema := func(optional bool) map[string]any {
		props := map[string]any{}
		required := []any{}
		for i := 0; i < 10; i++ {
			name := fmt.Sprintf("p%d", i)
			props[name] = map[string]any{"type": "string", "enum": values(100)}
			if !optional {
				required = append(required, name)
			}
		}
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	n, _ := newNode(t)
	_, issues := n.Spec().Params.Parse(map[string]any{"prompt": "x", "format": "json", "schema": schema(false)})
	require.Empty(t, issues, "ten required enums of 100 values are exactly 1000")
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "x", "format": "json", "schema": schema(true)})
	require.Len(t, issues, 1, "ten optional ones gain ten nulls: 1010")
	assert.Contains(t, issues[0].Message, "1010")
}

// A property called enum is a property, not the enum keyword: its values count like any other's.
func TestAPropertyCalledEnumDoesNotHideEnumValues(t *testing.T) {
	values := make([]any, 100)
	for i := range values {
		values[i] = fmt.Sprintf("v%d", i)
	}
	props := map[string]any{"enum": map[string]any{"type": "string", "enum": values}}
	for i := 0; i < 9; i++ {
		props[fmt.Sprintf("p%d", i)] = map[string]any{"type": "string", "enum": values}
	}
	n, _ := newNode(t)
	// Ten optional enums, one of them named enum: 1000 values and ten nulls.
	_, issues := n.Spec().Params.Parse(map[string]any{"prompt": "x", "format": "json", "schema": map[string]any{"type": "object", "properties": props}})
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "1010")
	// The same schema with the property required is exactly 1000.
	required := make([]any, 0, len(props))
	for name := range props {
		required = append(required, name)
	}
	_, issues = n.Spec().Params.Parse(map[string]any{"prompt": "x", "format": "json", "schema": map[string]any{"type": "object", "properties": props, "required": required}})
	require.Empty(t, issues)
}

// The cost is output-only: a cost that is not a number, or a counter of another type, does not erase the
// figures that are valid.
func TestMalformedUsageFiguresDoNotEraseTheValidOnes(t *testing.T) {
	for name, usage := range map[string]struct {
		body        string
		in, out     int64
		hasCostNull bool
	}{
		"cost as text":    {`{"prompt_tokens":10,"completion_tokens":2,"cost":"unknown"}`, 10, 2, true},
		"cost as object":  {`{"prompt_tokens":10,"completion_tokens":2,"cost":{"a":1}}`, 10, 2, true},
		"negative cost":   {`{"prompt_tokens":10,"completion_tokens":2,"cost":-1}`, 10, 2, true},
		"counter as text": {`{"prompt_tokens":"ten","completion_tokens":2,"cost":0.5}`, 0, 2, false},
		"fractional":      {`{"prompt_tokens":10.5,"completion_tokens":2}`, 0, 2, true},
	} {
		t.Run(name, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: `{"model":"m","choices":[{"finish_reason":"stop","message":{"content":"ok"}}],"usage":` + usage.body + `}`})
			n, _ := newNode(t, openai.WithTransport(srv.Transport()))
			rec := &aitest.Usage{}
			res, err := n.Execute(context.Background(), aitest.Input(openai.Kind, srv, nil, parsed(t, n, map[string]any{"prompt": "x"}), rec))
			require.NoError(t, err)
			assert.Equal(t, usage.in, rec.Total("token_in"))
			assert.Equal(t, usage.out, rec.Total("token_out"))
			if usage.hasCostNull {
				assert.Nil(t, res.Output["usage"].(map[string]any)["cost_usd"])
			} else {
				assert.InDelta(t, 0.5, res.Output["usage"].(map[string]any)["cost_usd"], 1e-9)
			}
		})
	}
}
