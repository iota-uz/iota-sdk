package testharness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRPCClient_DoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Contains(t, r.Header.Get("Cookie"), "granite_sid=token")

		var req rpcRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "bichat.ping", req.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     req.ID,
			"result": map[string]any{"ok": true},
		})
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		ServerURL:       srv.URL,
		RPCEndpointPath: "/rpc",
		CookieName:      "granite_sid",
		SessionToken:    "token",
	}
	client := NewRPCClient(cfg).WithHTTPClient(srv.Client())

	var out struct {
		OK bool `json:"ok"`
	}
	err := client.Do(context.Background(), "bichat.ping", map[string]any{}, &out)
	require.NoError(t, err)
	require.True(t, out.OK)
}

func TestRPCClient_MethodError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": r.Header.Get("X-Request-Id"),
			"error": map[string]any{
				"code":    "forbidden",
				"message": "permission denied",
			},
		})
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		ServerURL:       srv.URL,
		RPCEndpointPath: "/rpc",
	}
	client := NewRPCClient(cfg).WithHTTPClient(srv.Client())

	err := client.Do(context.Background(), "bichat.session.get", map[string]any{"id": uuid.NewString()}, nil)
	require.Error(t, err)

	var rpcErr *RPCMethodError
	require.ErrorAs(t, err, &rpcErr)
	require.Equal(t, "forbidden", rpcErr.Code)
	require.True(t, isForbidden(err))
}

func TestRPCClient_CreateSessionAndGetSession(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		w.Header().Set("Content-Type", "application/json")

		switch req.Method {
		case "bichat.session.create":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": req.ID,
				"result": map[string]any{
					"session": map[string]any{"id": sessionID.String()},
				},
			})
		case "bichat.session.get":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": req.ID,
				"result": map[string]any{
					"session": map[string]any{"id": sessionID.String()},
					"turns": []map[string]any{
						{
							"id": "turn-1",
							"assistantTurn": map[string]any{
								"id":      "assistant-1",
								"role":    "assistant",
								"content": "answer",
								"toolCalls": []map[string]any{
									{
										"name":      "sql_query",
										"arguments": `{"q":"select 1"}`,
									},
								},
								"debug": map[string]any{
									"usage": map[string]any{
										"promptTokens":     10,
										"completionTokens": 6,
										"totalTokens":      16,
										"cost":             0.002,
									},
								},
							},
						},
					},
				},
			})
		default:
			require.FailNow(t, "unexpected method", req.Method)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		ServerURL:       srv.URL,
		RPCEndpointPath: "/rpc",
	}
	client := NewRPCClient(cfg).WithHTTPClient(srv.Client())

	gotSessionID, err := client.CreateSession(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, sessionID, gotSessionID)

	session, err := client.GetSession(context.Background(), sessionID)
	require.NoError(t, err)
	require.Len(t, session.Turns, 1)
	require.NotNil(t, session.Turns[0].AssistantTurn)
	require.Equal(t, "answer", session.Turns[0].AssistantTurn.Content)
	require.Len(t, session.Turns[0].AssistantTurn.ToolCalls, 1)
	require.Equal(t, "sql_query", session.Turns[0].AssistantTurn.ToolCalls[0].Name)
	require.NotNil(t, session.Turns[0].AssistantTurn.Debug)
	require.NotNil(t, session.Turns[0].AssistantTurn.Debug.Usage)

	usage := session.Turns[0].AssistantTurn.Debug.Usage.ToDebugUsage()
	require.NotNil(t, usage)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 6, usage.CompletionTokens)
}
