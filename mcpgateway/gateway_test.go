package mcpgateway_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/mcpgateway"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/chattools"
)

const (
	issuer = "https://erp.test"
	scope  = "erp:read"
)

type quoteIn struct {
	Customer string `json:"customer"`
	Amount   string `json:"amount" jsonschema:"decimal string"`
}

type quoteOut struct {
	Total string `json:"total"`
}

// app is the example application: it registers one tool once and connects
// the same engine to both the MCP gateway and the built-in chat.
type app struct {
	engine  *mcp.Engine
	tenant  uuid.UUID
	key     *rsa.PrivateKey
	mu      sync.Mutex
	blocked map[string]bool
	server  *httptest.Server
	oauth   mcpgateway.OAuth
}

func newApp(t *testing.T) *app {
	t.Helper()
	reg := mcp.NewRegistry()
	require.NoError(t, reg.Register(mcp.MustNew(
		mcp.Spec{Name: "quote", Description: "Price a quote.", Annotations: mcp.Annotations{ReadOnly: true},
			Policy: mcp.Policy{Scopes: []string{scope}}},
		func(_ context.Context, in quoteIn) (quoteOut, error) {
			if in.Customer == "ghost" {
				return quoteOut{}, mcp.Fail(mcp.CodeNotFound, mcp.RecoveryFixInput, "unknown customer")
			}
			return quoteOut{Total: in.Amount + " UZS"}, nil
		})))
	engine, err := mcp.NewEngine(reg, mcp.AuthorizerFunc(func(ctx context.Context, p mcp.Principal, policy mcp.Policy) (context.Context, error) {
		if missing := p.Scopes.Missing(policy.Scopes); len(missing) > 0 {
			return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "missing scope")
		}
		return ctx, nil
	}))
	require.NoError(t, err)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	a := &app{engine: engine, tenant: uuid.New(), key: key, blocked: map[string]bool{}}

	server, err := mcpgateway.NewServer(engine, &sdk.Implementation{Name: "test-erp", Version: "v1"}, nil)
	require.NoError(t, err)
	mux := http.NewServeMux()
	a.server = httptest.NewServer(mux)
	t.Cleanup(a.server.Close)
	a.oauth = mcpgateway.OAuth{
		Resource: a.server.URL + "/mcp",
		Issuer:   issuer,
		KeySet: func(context.Context) (*jose.JSONWebKeySet, error) {
			return &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}}, nil
		},
		Accounts: mcpgateway.AccountResolverFunc(func(_ context.Context, subject string) (mcp.Principal, error) {
			a.mu.Lock()
			defer a.mu.Unlock()
			if a.blocked[subject] {
				return mcp.Principal{}, assert.AnError
			}
			return mcp.Principal{TenantID: a.tenant, UserID: 42}, nil
		}),
		ScopesSupported: []string{scope},
	}
	mux.Handle("/mcp", a.oauth.Handler(server, nil))
	mux.Handle(a.oauth.MetadataPath(), a.oauth.MetadataHandler())
	return a
}

func (a *app) token(t *testing.T, mutate func(jwt.MapClaims)) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss": issuer, "sub": "42", "aud": []string{a.oauth.Resource},
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "scope": []string{scope},
	}
	if mutate != nil {
		mutate(claims)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "k1"
	signed, err := tok.SignedString(a.key)
	require.NoError(t, err)
	return signed
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func (a *app) connect(t *testing.T, token string) *sdk.ClientSession {
	t.Helper()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "client", Version: "v1"}, nil).Connect(context.Background(),
		&sdk.StreamableClientTransport{Endpoint: a.oauth.Resource, HTTPClient: &http.Client{Transport: bearer{token}}}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// chatContext is what the chat module has after its own session middleware.
func (a *app) chatContext() context.Context {
	return mcp.WithPrincipal(composables.WithTenantID(context.Background(), a.tenant),
		mcp.Principal{TenantID: a.tenant, UserID: 42, Scopes: mcp.AllScopes()})
}

func TestGateway_ChatAndMCPShareOneContract(t *testing.T) {
	a := newApp(t)
	cs := a.connect(t, a.token(t, nil))
	chat, err := chattools.Tools(a.engine)
	require.NoError(t, err)
	require.Len(t, chat, 1)

	listed, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, listed.Tools, 1)
	mcpSchema, err := json.Marshal(listed.Tools[0].InputSchema)
	require.NoError(t, err)
	chatSchema, err := json.Marshal(chat[0].Parameters())
	require.NoError(t, err)
	assert.JSONEq(t, string(mcpSchema), string(chatSchema), "schemas drifted between adapters")
	assert.Equal(t, chat[0].Name(), listed.Tools[0].Name)
	assert.True(t, listed.Tools[0].Annotations.ReadOnlyHint)
	assert.NotNil(t, listed.Tools[0].OutputSchema)

	cases := []struct {
		name    string
		args    string
		isError bool
	}{
		{name: "success", args: `{"customer":"acme","amount":"10.50"}`},
		{name: "domain failure", args: `{"customer":"ghost","amount":"1"}`, isError: true},
		{name: "unknown argument", args: `{"customer":"acme","amount":"1","discount":"99"}`, isError: true},
		{name: "missing argument", args: `{"customer":"acme"}`, isError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "quote", Arguments: json.RawMessage(tc.args)})
			require.NoError(t, err, "tool failures are not protocol errors")
			assert.Equal(t, tc.isError, res.IsError)
			viaMCP, err := json.Marshal(res.StructuredContent)
			require.NoError(t, err)

			viaChat, err := chat[0].Call(a.chatContext(), tc.args)
			require.NoError(t, err)
			assert.JSONEq(t, viaChat, string(viaMCP), "results drifted between adapters")
		})
	}

	_, err = cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "nope", Arguments: json.RawMessage(`{}`)})
	assert.Error(t, err, "an unknown tool stays a protocol error")
}

func TestGateway_ChatSessionIdentity(t *testing.T) {
	a := newApp(t)
	chat, err := chattools.Tools(a.engine)
	require.NoError(t, err)

	out, err := chat[0].Call(context.Background(), `{"customer":"acme","amount":"1"}`)
	require.NoError(t, err)
	var res mcp.Result
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	require.False(t, res.OK, "no session means no identity")
	assert.Equal(t, mcp.CodeForbidden, res.Error.Code)
}

func TestGateway_ResourceServer(t *testing.T) {
	a := newApp(t)
	cases := []struct {
		name   string
		mutate func(jwt.MapClaims)
	}{
		{name: "foreign issuer", mutate: func(c jwt.MapClaims) { c["iss"] = "https://evil.test" }},
		{name: "other resource", mutate: func(c jwt.MapClaims) { c["aud"] = []string{"https://other.test/mcp"} }},
		{name: "expired", mutate: func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{name: "no expiry", mutate: func(c jwt.MapClaims) { delete(c, "exp") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, header := post(t, a.oauth.Resource, a.token(t, tc.mutate))
			assert.Equal(t, http.StatusUnauthorized, status)
			assert.Contains(t, header, "resource_metadata=")
		})
	}

	t.Run("missing token advertises metadata", func(t *testing.T) {
		status, header := post(t, a.oauth.Resource, "")
		assert.Equal(t, http.StatusUnauthorized, status)
		assert.Contains(t, header, a.server.URL+"/.well-known/oauth-protected-resource/mcp")
	})

	t.Run("protected resource metadata", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, a.server.URL+"/.well-known/oauth-protected-resource/mcp", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		var meta map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&meta))
		assert.Equal(t, a.oauth.Resource, meta["resource"])
		assert.Equal(t, []any{issuer}, meta["authorization_servers"])
	})

	t.Run("scopes narrow access", func(t *testing.T) {
		cs := a.connect(t, a.token(t, func(c jwt.MapClaims) { c["scope"] = "profile" }))
		res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "quote", Arguments: json.RawMessage(`{"customer":"a","amount":"1"}`)})
		require.NoError(t, err)
		require.True(t, res.IsError)
		assert.Contains(t, mustJSON(t, res.StructuredContent), `"code":"forbidden"`)
	})

	t.Run("blocking the account cuts an open connection", func(t *testing.T) {
		cs := a.connect(t, a.token(t, nil))
		_, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "quote", Arguments: json.RawMessage(`{"customer":"a","amount":"1"}`)})
		require.NoError(t, err)

		a.mu.Lock()
		a.blocked["42"] = true
		a.mu.Unlock()
		_, err = cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "quote", Arguments: json.RawMessage(`{"customer":"a","amount":"1"}`)})
		assert.Error(t, err)
	})
}

func TestGateway_StdioProxyMirrorsServer(t *testing.T) {
	a := newApp(t)
	remote := a.connect(t, a.token(t, nil))
	impl := &sdk.Implementation{Name: "stdio", Version: "v1"}
	proxy, err := mcpgateway.Proxy(context.Background(), remote, impl)
	require.NoError(t, err)

	serverSide, clientSide := sdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = proxy.Run(ctx, serverSide) }()
	local, err := sdk.NewClient(impl, nil).Connect(ctx, clientSide, nil)
	require.NoError(t, err)
	defer func() { _ = local.Close() }()

	viaHTTP, err := remote.ListTools(ctx, nil)
	require.NoError(t, err)
	viaStdio, err := local.ListTools(ctx, nil)
	require.NoError(t, err)
	assert.JSONEq(t, mustJSON(t, viaHTTP.Tools), mustJSON(t, viaStdio.Tools))

	args := json.RawMessage(`{"customer":"ghost","amount":"1"}`)
	direct, err := remote.CallTool(ctx, &sdk.CallToolParams{Name: "quote", Arguments: args})
	require.NoError(t, err)
	proxied, err := local.CallTool(ctx, &sdk.CallToolParams{Name: "quote", Arguments: args})
	require.NoError(t, err)
	assert.True(t, proxied.IsError)
	assert.JSONEq(t, mustJSON(t, direct.StructuredContent), mustJSON(t, proxied.StructuredContent))
}

func post(t *testing.T, url, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, resp.Header.Get("WWW-Authenticate")
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}
