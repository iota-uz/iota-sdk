package openai_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aitest"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/openai"
)

func completeCall(srv *aitest.Server, args string, u *aitest.Usage) connectors.CodeCall {
	if u == nil {
		u = &aitest.Usage{}
	}
	in := aitest.Input(openai.Kind, srv, nil, nil, u)
	return connectors.CodeCall{Operation: "complete", Args: json.RawMessage(args), Input: in}
}

func codeRefusal(t *testing.T, err error) *connectors.CodeOperationError {
	t.Helper()
	var ce *connectors.CodeOperationError
	require.ErrorAs(t, err, &ce)
	return ce
}

func TestCodeAccessRegistersInADirectory(t *testing.T) {
	_, c := newNode(t)
	require.NoError(t, connectors.NewDirectory(nil).Register(c))
	access := c.(connectors.CodeAccessor).CodeAccess()
	assert.Equal(t, []string{"complete"}, access.OperationNames())
	assert.Empty(t, access.Methods)
}

func TestCodeCompleteText(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("Hello there", 12, 3)})
	_, c := newNode(t, openai.WithTransport(srv.Transport()), openai.WithDefaultModel("gpt-4o-mini-2024-07-18"))
	usage := &aitest.Usage{}
	out, err := c.(connectors.CodeOperator).CallCodeOperation(context.Background(),
		completeCall(srv, `{"prompt":"Say hi","system":"Be brief","temperature":0.2,"max_tokens":50}`, usage))
	require.NoError(t, err)
	res := out.(map[string]any)
	assert.Equal(t, "Hello there", res["text"])
	assert.Equal(t, "test-model-2025-01-01", res["model"])
	assert.Equal(t, map[string]any{"input_tokens": int64(12), "output_tokens": int64(3)}, res["usage"])
	assert.EqualValues(t, 12, usage.Total("token_in"))
	assert.EqualValues(t, 3, usage.Total("token_out"))

	reqs := srv.Requests()
	require.Len(t, reqs, 1)
	body := reqs[0].JSON(t)
	assert.Equal(t, "gpt-4o-mini-2024-07-18", body["model"], "the default model")
	assert.Equal(t, "Bearer sk-test-key-123456", reqs[0].Header.Get("Authorization"))
	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "sk-test-key")
}

func TestCodeCompleteJSON(t *testing.T) {
	t.Run("a schema", func(t *testing.T) {
		srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(`{"summary":"ok","score":3}`, 20, 8)})
		_, c := newNode(t, openai.WithTransport(srv.Transport()))
		usage := &aitest.Usage{}
		out, err := c.(connectors.CodeOperator).CallCodeOperation(context.Background(),
			completeCall(srv, `{"prompt":"rate it","model":"gpt-4o-2024-08-06","json":`+answerSchema+`}`, usage))
		require.NoError(t, err)
		res := out.(map[string]any)
		assert.NotContains(t, res, "text")
		raw, _ := json.Marshal(res["json"])
		assert.JSONEq(t, `{"summary":"ok","score":3}`, string(raw))
		assert.EqualValues(t, 20, usage.Total("token_in"))
		body := srv.Requests()[0].JSON(t)
		rf, _ := body["response_format"].(map[string]any)
		assert.Equal(t, "json_schema", rf["type"])
	})
	t.Run("json true", func(t *testing.T) {
		srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("```json\n{\"a\":[1,2]}\n```", 5, 5)})
		_, c := newNode(t, openai.WithTransport(srv.Transport()))
		out, err := c.(connectors.CodeOperator).CallCodeOperation(context.Background(), completeCall(srv, `{"prompt":"list","json":true}`, nil))
		require.NoError(t, err)
		raw, _ := json.Marshal(out.(map[string]any)["json"])
		assert.JSONEq(t, `{"a":[1,2]}`, string(raw))
		assert.Nil(t, srv.Requests()[0].JSON(t)["response_format"])
	})
	t.Run("an answer outside the schema", func(t *testing.T) {
		srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(`{"summary":"ok"}`, 20, 8)})
		_, c := newNode(t, openai.WithTransport(srv.Transport()))
		usage := &aitest.Usage{}
		_, err := c.(connectors.CodeOperator).CallCodeOperation(context.Background(),
			completeCall(srv, `{"prompt":"rate it","model":"gpt-4o-2024-08-06","json":`+answerSchema+`}`, usage))
		assert.Equal(t, "invalid_response", codeRefusal(t, err).Code)
		assert.EqualValues(t, 20, usage.Total("token_in"), "the spent tokens are recorded anyway")
	})
}

func TestCodeCompleteInvalidArgs(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("x", 1, 1)})
	_, c := newNode(t, openai.WithTransport(srv.Transport()))
	for name, args := range map[string]string{
		"no prompt":             `{}`,
		"null":                  `null`,
		"blank prompt":          `{"prompt":"  "}`,
		"temperature":           `{"prompt":"x","temperature":3}`,
		"max tokens":            `{"prompt":"x","max_tokens":0}`,
		"unknown field":         `{"prompt":"x","api_key":"k"}`,
		"bad model":             `{"prompt":"x","model":"a b"}`,
		"json a string":         `{"prompt":"x","json":"yes"}`,
		"bad schema":            `{"prompt":"x","json":{"type":"nope"}}`,
		"old model json":        `{"prompt":"x","model":"gpt-3.5-turbo","json":{"type":"object","properties":{"a":{"type":"string"}}}}`,
		"reasoning temperature": `{"prompt":"x","temperature":0.5}`,
	} {
		_, err := c.(connectors.CodeOperator).CallCodeOperation(context.Background(), completeCall(srv, args, nil))
		assert.Equal(t, "invalid_args", codeRefusal(t, err).Code, name)
	}
	assert.Empty(t, srv.Requests())

	// GPT-5.1 defaults to reasoning effort none and accepts an explicit temperature.
	out, err := c.(connectors.CodeOperator).CallCodeOperation(context.Background(),
		completeCall(srv, `{"prompt":"x","model":"gpt-5.1-2025-11-13","temperature":0.4}`, nil))
	require.NoError(t, err)
	assert.Equal(t, "x", out.(map[string]any)["text"])
	require.Len(t, srv.Requests(), 1)
	assert.InDelta(t, 0.4, srv.Requests()[0].JSON(t)["temperature"], 1e-9)
}

func TestCodeCompleteProviderFailures(t *testing.T) {
	call := func(t *testing.T, reply aitest.Reply, opts ...openai.Option) (*connectors.CodeOperationError, *aitest.Usage) {
		t.Helper()
		srv := aitest.New(t, reply)
		_, c := newNode(t, append([]openai.Option{openai.WithTransport(srv.Transport())}, opts...)...)
		usage := &aitest.Usage{}
		_, err := c.(connectors.CodeOperator).CallCodeOperation(context.Background(), completeCall(srv, `{"prompt":"x"}`, usage))
		return codeRefusal(t, err), usage
	}
	t.Run("rate limit", func(t *testing.T) {
		ce, _ := call(t, aitest.Reply{Status: 429, Header: map[string]string{"Retry-After": "2"}, Body: aitest.APIError("slow down", "rate_limit", "rate_limit_exceeded")})
		assert.Equal(t, "rate_limited", ce.Code)
		assert.True(t, ce.Retryable)
	})
	t.Run("a refused key marks the connection", func(t *testing.T) {
		statuses := &statusLog{}
		ce, _ := call(t, aitest.Reply{Status: 401, Body: aitest.APIError("bad key sk-test-key-123456", "invalid_request_error", "invalid_api_key")}, openai.WithConnectionStatus(statuses))
		assert.Equal(t, "provider_refused", ce.Code)
		assert.False(t, ce.Retryable)
		assert.NotContains(t, ce.Error(), "sk-test-key")
		require.Len(t, statuses.calls, 1)
		assert.Equal(t, connectors.StatusNeedsReauth, statuses.calls[0].status)
	})
	t.Run("an outage is retryable", func(t *testing.T) {
		ce, _ := call(t, aitest.Reply{Status: 503, Body: aitest.APIError("down", "server_error", "")})
		assert.Equal(t, "unavailable", ce.Code)
		assert.True(t, ce.Retryable)
	})
	t.Run("a policy refusal", func(t *testing.T) {
		ce, _ := call(t, aitest.Reply{Status: 400, Body: aitest.APIError("no", "invalid_request_error", "content_policy_violation")})
		assert.Equal(t, "provider_refused", ce.Code)
	})
}

func TestCodeCompleteWithoutAKey(t *testing.T) {
	srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("x", 1, 1)})
	_, c := newNode(t, openai.WithTransport(srv.Transport()))
	call := completeCall(srv, `{"prompt":"x"}`, nil)
	call.Input.Credentials["api_key"] = ""
	_, err := c.(connectors.CodeOperator).CallCodeOperation(context.Background(), call)
	assert.Equal(t, "provider_refused", codeRefusal(t, err).Code)
}
