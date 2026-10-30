package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type greetIn struct {
	Name string `json:"name"`
	Mood string `json:"mood,omitempty"`
}

type greetOut struct {
	Greeting string `json:"greeting"`
}

type recordingAuditor struct{ entries []mcp.AuditEntry }

func (a *recordingAuditor) Record(_ context.Context, e mcp.AuditEntry) {
	a.entries = append(a.entries, e)
}

func newEngine(t *testing.T, audit *recordingAuditor) *mcp.Engine {
	t.Helper()
	reg := mcp.NewRegistry()
	require.NoError(t, reg.Register(
		mcp.MustNew(mcp.Spec{Name: "greet", Description: "Greet someone."},
			func(_ context.Context, in greetIn) (greetOut, error) {
				switch in.Name {
				case "conflict":
					return greetOut{}, serrors.New(serrors.Conflict, "row version 3 in table secrets")
				case "crash":
					return greetOut{}, errors.New("pq: password authentication failed for user admin")
				}
				return greetOut{Greeting: "hello " + in.Name}, nil
			}),
		mcp.MustNew(mcp.Spec{Name: "secret", Policy: mcp.Policy{Permissions: []string{"Secret.Read"}}},
			func(_ context.Context, _ struct{}) (greetOut, error) { return greetOut{}, nil }),
		mcp.MustNew(mcp.Spec{Name: "host_only", HostOnly: true},
			func(_ context.Context, _ struct{}) (greetOut, error) { return greetOut{Greeting: "host"}, nil }),
	))
	authz := mcp.AuthorizerFunc(func(ctx context.Context, _ mcp.Principal, policy mcp.Policy) (context.Context, error) {
		if len(policy.Permissions) > 0 {
			return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryRequestAccess, "missing permission "+policy.Permissions[0])
		}
		return ctx, nil
	})
	engine, err := mcp.NewEngine(reg, authz, mcp.WithAuditor(audit))
	require.NoError(t, err)
	return engine
}

func TestEngine_Call(t *testing.T) {
	authed := mcp.WithPrincipal(context.Background(), mcp.Principal{TenantID: uuid.New(), UserID: 1, Scopes: mcp.AllScopes()})
	cases := []struct {
		name     string
		ctx      context.Context
		tool     string
		input    string
		wantJSON string
		code     mcp.Code
		recovery mcp.Recovery
		issues   []string
	}{
		{name: "success", ctx: authed, tool: "greet", input: `{"name":"ann"}`, wantJSON: `{"ok":true,"data":{"greeting":"hello ann"}}`},
		{name: "unauthenticated", ctx: context.Background(), tool: "greet", input: `{"name":"ann"}`,
			code: mcp.CodeForbidden, recovery: mcp.RecoveryReauthenticate},
		{name: "unknown tool", ctx: authed, tool: "nope", input: `{}`, code: mcp.CodeNotFound, recovery: mcp.RecoveryFixInput},
		{name: "policy denied", ctx: authed, tool: "secret", input: `{}`, code: mcp.CodeForbidden, recovery: mcp.RecoveryRequestAccess},
		{name: "unknown argument rejected", ctx: authed, tool: "greet", input: `{"name":"ann","admin":true}`,
			code: mcp.CodeInvalidInput, recovery: mcp.RecoveryFixInput, issues: []string{""}},
		{name: "wrong type", ctx: authed, tool: "greet", input: `{"name":5}`,
			code: mcp.CodeInvalidInput, recovery: mcp.RecoveryFixInput, issues: []string{"/name"}},
		{name: "missing required", ctx: authed, tool: "greet", input: `{"mood":"ok"}`,
			code: mcp.CodeInvalidInput, recovery: mcp.RecoveryFixInput, issues: []string{""}},
		{name: "domain conflict keeps diagnostics private", ctx: authed, tool: "greet", input: `{"name":"conflict"}`,
			code: mcp.CodeConflict, recovery: mcp.RecoveryReadThenPrepare},
		{name: "internal error keeps diagnostics private", ctx: authed, tool: "greet", input: `{"name":"crash"}`,
			code: mcp.CodeDependencyUnavailable, recovery: mcp.RecoveryRetryLater},
		{name: "host-only tool hidden from nested callers", ctx: mcp.WithNestedCaller(authed), tool: "host_only", input: `{}`,
			code: mcp.CodeNotFound, recovery: mcp.RecoveryFixInput},
		{name: "host-only tool callable by the host", ctx: authed, tool: "host_only", input: `{}`,
			wantJSON: `{"ok":true,"data":{"greeting":"host"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := newEngine(t, &recordingAuditor{}).Call(tc.ctx, tc.tool, json.RawMessage(tc.input))
			raw, err := json.Marshal(res)
			require.NoError(t, err)
			if tc.wantJSON != "" {
				assert.JSONEq(t, tc.wantJSON, string(raw))
				return
			}
			require.False(t, res.OK, string(raw))
			assert.Equal(t, tc.code, res.Error.Code)
			assert.Equal(t, tc.recovery, res.Error.Recovery)
			assert.NotContains(t, res.Error.Message, "password")
			assert.NotContains(t, res.Error.Message, "secrets")
			paths := make([]string, 0, len(res.Error.Issues))
			for _, iss := range res.Error.Issues {
				paths = append(paths, iss.Path)
			}
			for _, want := range tc.issues {
				assert.Contains(t, paths, want)
			}
			assert.Contains(t, string(raw), `"issues":[`)
		})
	}
}

func TestEngine_AuditOmitsPayloads(t *testing.T) {
	audit := &recordingAuditor{}
	tenant := uuid.New()
	ctx := mcp.WithRequestID(mcp.WithPrincipal(context.Background(),
		mcp.Principal{TenantID: tenant, UserID: 9, ClientID: "claude", Scopes: mcp.AllScopes()}), "req-1")
	engine := newEngine(t, audit)

	engine.Call(ctx, "greet", json.RawMessage(`{"name":"ann"}`))
	engine.Call(mcp.WithNestedCaller(ctx), "secret", json.RawMessage(`{}`))

	require.Len(t, audit.entries, 2)
	assert.Equal(t, mcp.AuditEntry{RequestID: "req-1", Tool: "greet", Subject: "user:9", TenantID: tenant.String(),
		ClientID: "claude", Duration: audit.entries[0].Duration}, audit.entries[0])
	assert.True(t, audit.entries[1].Nested)
	assert.Equal(t, mcp.CodeForbidden, audit.entries[1].Outcome)
}

func TestRegistry_Register(t *testing.T) {
	ok := mcp.MustNew(mcp.Spec{Name: "ok"}, func(_ context.Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	cases := []struct {
		name  string
		tools []mcp.Tool
		code  serrors.Code
	}{
		{name: "duplicate", tools: []mcp.Tool{ok, ok}, code: serrors.AlreadyExists},
		{name: "not a python identifier", tools: []mcp.Tool{
			mcp.MustNew(mcp.Spec{Name: "bad-name"}, func(_ context.Context, _ struct{}) (struct{}, error) { return struct{}{}, nil }),
		}, code: serrors.Invalid},
		{name: "non-object input", tools: []mcp.Tool{
			mcp.MustNew(mcp.Spec{Name: "scalar"}, func(_ context.Context, _ string) (struct{}, error) { return struct{}{}, nil }),
		}, code: serrors.Invalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mcp.NewRegistry().Register(tc.tools...)
			require.Error(t, err)
			assert.Equal(t, tc.code, serrors.CodeOf(err))
		})
	}
}

func TestPrincipal_ScopesNarrowAccess(t *testing.T) {
	cases := []struct {
		name     string
		scopes   mcp.Scopes
		required []string
		missing  []string
	}{
		{name: "first-party session", scopes: mcp.AllScopes(), required: []string{"mcp:write"}},
		{name: "granted", scopes: mcp.GrantedScopes("mcp:read", "mcp:write"), required: []string{"mcp:write"}},
		{name: "not granted", scopes: mcp.GrantedScopes("mcp:read"), required: []string{"mcp:write"}, missing: []string{"mcp:write"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.missing, tc.scopes.Missing(tc.required))
		})
	}
}
