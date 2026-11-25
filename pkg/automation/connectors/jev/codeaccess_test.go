package jev_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aitest"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/jev"
)

func codeCall(srv *aitest.Server, provider, args string, u *aitest.Usage) connectors.CodeCall {
	return connectors.CodeCall{Operation: "complete", Args: json.RawMessage(args), Input: aitest.Input(jev.Kind, srv, creds(provider), nil, u)}
}

func TestCodeAccessRegistersInADirectory(t *testing.T) {
	_, c := newNode(t)
	require.NoError(t, connectors.NewDirectory(nil).Register(c))
	assert.Equal(t, []string{"complete"}, c.(connectors.CodeAccessor).CodeAccess().OperationNames())
}

func TestCodeCompleteByProvider(t *testing.T) {
	for _, tc := range []struct{ provider, host, model string }{
		{"openai", "api.openai.com", jev.DefaultModelOpenAI},
		{"openrouter", "openrouter.ai", jev.DefaultCodeModelOpenRouter},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("done", 9, 4)})
			_, c := newNode(t, jev.WithTransport(srv.Transport()))
			usage := &aitest.Usage{}
			out, err := c.(connectors.CodeOperator).CallCodeOperation(context.Background(), codeCall(srv, tc.provider, `{"prompt":"hi","system":"s"}`, usage))
			require.NoError(t, err)
			res := out.(map[string]any)
			assert.Equal(t, "done", res["text"])
			assert.EqualValues(t, 9, usage.Total("token_in"))
			assert.EqualValues(t, 4, usage.Total("token_out"))
			reqs := srv.Requests()
			require.Len(t, reqs, 1)
			assert.Equal(t, tc.host, reqs[0].Host())
			assert.Equal(t, tc.model, reqs[0].JSON(t)["model"])
			raw, _ := json.Marshal(res)
			assert.NotContains(t, string(raw), "sk-test-key")
		})
	}
}

func TestCodeCompleteJSONAndFailures(t *testing.T) {
	ctx := context.Background()
	t.Run("json", func(t *testing.T) {
		srv := aitest.New(t, aitest.Reply{Body: aitest.Chat(`{"n":2}`, 3, 3)})
		_, c := newNode(t, jev.WithTransport(srv.Transport()))
		out, err := c.(connectors.CodeOperator).CallCodeOperation(ctx, codeCall(srv, "openai", `{"prompt":"x","model":"gpt-4o-2024-08-06","json":{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"]}}`, &aitest.Usage{}))
		require.NoError(t, err)
		raw, _ := json.Marshal(out.(map[string]any)["json"])
		assert.JSONEq(t, `{"n":2}`, string(raw))
	})
	t.Run("rate limit and refused key", func(t *testing.T) {
		srv := aitest.New(t, aitest.Reply{Status: 429, Body: aitest.APIError("slow", "rate_limit", "rate_limit_exceeded")}, aitest.Reply{Status: 401, Body: aitest.APIError("no", "x", "invalid_api_key")})
		_, c := newNode(t, jev.WithTransport(srv.Transport()))
		_, err := c.(connectors.CodeOperator).CallCodeOperation(ctx, codeCall(srv, "openai", `{"prompt":"x"}`, &aitest.Usage{}))
		var ce *connectors.CodeOperationError
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, "rate_limited", ce.Code)
		assert.True(t, ce.Retryable)
		_, err = c.(connectors.CodeOperator).CallCodeOperation(ctx, codeCall(srv, "openai", `{"prompt":"x"}`, &aitest.Usage{}))
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, "provider_refused", ce.Code)
	})
	t.Run("invalid arguments and credentials", func(t *testing.T) {
		srv := aitest.New(t, aitest.Reply{Body: aitest.Chat("x", 1, 1)})
		_, c := newNode(t, jev.WithTransport(srv.Transport()))
		var ce *connectors.CodeOperationError
		_, err := c.(connectors.CodeOperator).CallCodeOperation(ctx, codeCall(srv, "openai", `{"prompt":""}`, &aitest.Usage{}))
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, "invalid_args", ce.Code)
		_, err = c.(connectors.CodeOperator).CallCodeOperation(ctx, codeCall(srv, "nobody", `{"prompt":"x"}`, &aitest.Usage{}))
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, "provider_refused", ce.Code)
		assert.Empty(t, srv.Requests())
	})
}
