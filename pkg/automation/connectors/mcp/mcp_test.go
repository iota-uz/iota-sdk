package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/mcp/mcptest"
)

var weatherInput = map[string]any{
	"type":     "object",
	"required": []any{"city"},
	"properties": map[string]any{
		"city": map[string]any{"type": "string"},
		"days": map[string]any{"type": "integer"},
		"opts": map[string]any{"type": "object", "properties": map[string]any{"units": map[string]any{"type": "string", "enum": []any{"metric", "imperial"}}}},
	},
	"additionalProperties": false,
}

var weatherOutput = map[string]any{
	"type":       "object",
	"required":   []any{"temp"},
	"properties": map[string]any{"temp": map[string]any{"type": "number"}},
}

func weatherTool(handle func(map[string]any) mcptest.Result) mcptest.Tool {
	return mcptest.Tool{Name: "weather", Title: "Weather", InputSchema: weatherInput, OutputSchema: weatherOutput, ReadOnly: true, Handle: handle}
}

func pinOf(t *testing.T, tool mcptest.Tool) map[string]any {
	t.Helper()
	in, _ := json.Marshal(tool.InputSchema)
	var out json.RawMessage
	if tool.OutputSchema != nil {
		out, _ = json.Marshal(tool.OutputSchema)
	}
	p, err := PinOf(Tool{Name: tool.Name, InputSchema: in, OutputSchema: out, ReadOnly: tool.ReadOnly})
	require.NoError(t, err)
	v, err := pinValue(p)
	require.NoError(t, err)
	return v.(map[string]any)
}

func TestTestReportsServerIdentityAndChecksTheToken(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	srv.RequireToken("tok-1")
	c := testConnector(t, srv, newMemStore(), nil)

	id, err := c.Test(context.Background(), connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthBearer, KeyToken: "tok-1"})
	require.NoError(t, err)
	require.Equal(t, "fake-mcp 1.2.3 (1 tools)", id.Display)
	require.Equal(t, 1, srv.Requests("tools/list"))

	_, err = c.Test(context.Background(), connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthBearer, KeyToken: "wrong"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")
	require.NotContains(t, err.Error(), "wrong")
}

func TestExecuteRendersArgumentsByThePin(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(func(args map[string]any) mcptest.Result {
		return mcptest.Result{Text: "sunny", Structured: map[string]any{"temp": 21.5}, Image: true}
	})
	srv.SetTool(tool)
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthNone}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)

	res, err := c.Node().Execute(context.Background(), callInput(id, creds, map[string]any{
		"tool": "weather", "tool_schema": pinOf(t, tool),
		"arguments": map[string]any{
			"city":       map[string]any{"template": "Tashkent"},
			"days":       map[string]any{"template": "3"},
			"opts.units": map[string]any{"cel": "metric"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, OutcomeDone, res.Outcome)
	require.Equal(t, "sunny", res.Output["text"])
	require.Equal(t, map[string]any{"temp": json.Number("21.5")}, res.Output["structured"])
	calls := srv.Calls()
	require.Len(t, calls, 1)
	require.Equal(t, map[string]any{"city": "Tashkent", "days": float64(3), "opts": map[string]any{"units": "metric"}}, calls[0].Arguments)
}

func TestExecuteRejectsToolErrorsAndOutputOutsideThePin(t *testing.T) {
	srv := mcptest.New(t)
	answer := mcptest.Result{IsError: true, Text: "city is unknown"}
	tool := weatherTool(func(map[string]any) mcptest.Result { return answer })
	srv.SetTool(tool)
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)
	params := map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool), "arguments": map[string]any{"city": map[string]any{"value": "Nowhere"}}}

	_, err := c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.Equal(t, catalog.CallRejected, callCode(err))
	require.Contains(t, err.Error(), "city is unknown")

	answer = mcptest.Result{Text: "ok", Structured: map[string]any{"temp": "warm"}}
	_, err = c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.Equal(t, catalog.CallRejected, callCode(err))
	require.Contains(t, err.Error(), "pinned output schema")
}

func TestDroppedAnswerAfterToolsCallIsOutcomeUnknownWithoutASecondCall(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(func(map[string]any) mcptest.Result { return mcptest.Result{Text: "done"} })
	srv.SetTool(tool)
	srv.Fail("tools/call", mcptest.FaultDrop)
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, err := c.Node().Execute(ctx, callInput(id, creds, map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool),
		"arguments": map[string]any{"city": map[string]any{"value": "Tashkent"}}}))
	require.Equal(t, catalog.CallOutcomeUnknown, callCode(err), "%v", err)
	require.Len(t, srv.Calls(), 1, "the tool ran once")
	require.Equal(t, 1, srv.Requests("tools/call"), "no second tools/call")
	require.False(t, c.Node().Idempotent())
}

func TestRateLimitsAndOversizedAnswers(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(nil)
	srv.SetTool(tool)
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)
	params := map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool), "arguments": map[string]any{"city": map[string]any{"value": "x"}}}

	srv.SetRetryAfter(7)
	srv.Fail("tools/call", mcptest.FaultRateLimited)
	res, err := c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.NoError(t, err)
	require.Equal(t, 7*time.Second, res.RetryAfter)
	require.Empty(t, srv.Calls())

	srv.SetRetryAfter(0)
	srv.Fail("initialize", mcptest.FaultRateLimited)
	_, err = c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.Equal(t, catalog.CallRateLimited, callCode(err))

	srv.Fail("tools/call", mcptest.FaultOversized)
	_, err = c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.Equal(t, catalog.CallInvalidRequest, callCode(err), "%v", err)
}

func TestPrivateAddressesAreRefused(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	_, port := srv.Host()
	names := map[string]string{"metadata.example.test": "169.254.169.254", "intranet.example.test": "10.0.0.8"}
	st := newMemStore()
	c := testConnector(t, srv, st, names)
	for host := range names {
		creds := connectors.Credentials{KeyServerURL: "https://" + host + ":" + port + "/mcp"}
		_, err := c.Test(context.Background(), creds)
		require.Error(t, err, host)
		require.Contains(t, err.Error(), "unavailable")

		id := st.add(tenant, creds)
		_, err = c.Node().Execute(context.Background(), callInput(id, creds, map[string]any{"tool": "weather", "tool_schema": pinOf(t, weatherTool(nil)),
			"arguments": map[string]any{"city": map[string]any{"value": "x"}}}))
		require.Equal(t, catalog.CallUnavailable, callCode(err), host)
		require.Contains(t, err.Error(), "not allowed")
	}
	require.Zero(t, srv.Requests("initialize"))
}

func TestRedirectFromPublicToPrivateIsRefused(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	_, port := srv.Host()
	names := map[string]string{"public.example.test": "127.0.0.1", "private.example.test": "10.1.2.3"}
	c := testConnector(t, srv, newMemStore(), names)
	creds := connectors.Credentials{KeyServerURL: "https://public.example.test:" + port + "/mcp"}

	// The public name reaches the fake: the policy lets it through.
	_, err := c.Test(context.Background(), creds)
	require.NoError(t, err)

	srv.SetRedirect("https://private.example.test:" + port + "/mcp")
	srv.Fail("initialize", mcptest.FaultRedirect)
	_, err = c.Test(context.Background(), creds)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unavailable")
	require.Equal(t, 2, srv.Requests("initialize"), "the redirect was not followed to the private address")

	srv.Fail("initialize", mcptest.FaultRedirect)
	cf, err := readConfig(creds)
	require.NoError(t, err)
	_, _, err = c.list(context.Background(), cf, "")
	require.Equal(t, catalog.CallUnavailable, callCode(err))
	require.Contains(t, err.Error(), "not allowed")
}

func TestAuthorizationIsDroppedOnCrossOriginRedirect(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	_, port := srv.Host()
	names := map[string]string{"a.example.test": "127.0.0.1", "b.example.test": "127.0.0.1"}
	c := testConnector(t, srv, newMemStore(), names)
	srv.RequireToken("tok-9")
	srv.SetRedirect("https://b.example.test:" + port + "/mcp")
	srv.Fail("initialize", mcptest.FaultRedirect)
	_, err := c.Test(context.Background(), connectors.Credentials{KeyServerURL: "https://a.example.test:" + port + "/mcp", KeyAuth: AuthBearer, KeyToken: "tok-9"})
	require.Error(t, err, "the token is not sent to another origin, so the redirected request is refused")
	require.Contains(t, err.Error(), "unauthorized")
}

func TestOAuthSignInRegistersOnceAndRefreshesUnderTheLock(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(func(map[string]any) mcptest.Result { return mcptest.Result{Text: "ok"} })
	srv.SetTool(tool)
	srv.EnableOAuth()
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthOAuth2}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)
	ctx := context.Background()
	const redirect = "https://erp.example.com/settings/integrations/oauth/callback"

	require.True(t, c.NeedsAuthorization(creds))
	_, err := c.Test(ctx, creds)
	require.Error(t, err)

	signIn := func() connectors.Credentials {
		authURL, state, err := c.BeginAuthorization(ctx, connectors.AuthorizationStart{TenantID: tenant, ConnectionID: id, RedirectURL: redirect})
		require.NoError(t, err)
		u, _ := url.Parse(authURL)
		require.Equal(t, srv.URL, u.Query().Get("resource"))
		require.Equal(t, "S256", u.Query().Get("code_challenge_method"))
		require.Equal(t, "tools", u.Query().Get("scope"))
		code := approve(t, srv, authURL)
		out, err := c.CompleteAuthorization(ctx, connectors.AuthorizationReturn{TenantID: tenant, ConnectionID: id, State: state, Code: code})
		require.NoError(t, err)
		return out
	}
	got := signIn()
	require.NotEmpty(t, got[KeyAccessToken])
	require.NotEmpty(t, got[KeyClientID])
	require.Equal(t, "true", got[KeyRegistered])
	require.False(t, c.NeedsAuthorization(got))
	require.Equal(t, 1, srv.Registrations())
	ident, err := c.Test(ctx, got)
	require.NoError(t, err)
	require.Contains(t, ident.Display, "fake-mcp")

	// A second sign-in reuses the registered client.
	got = signIn()
	require.Equal(t, 1, srv.Registrations())

	// A state is bound to its connection and does not open for another purpose.
	_, err = c.CompleteAuthorization(ctx, connectors.AuthorizationReturn{TenantID: tenant, ConnectionID: id, State: "forged.state", Code: "x"})
	require.ErrorIs(t, err, ErrState)

	// An access token expiring within 15 minutes is refreshed once, even by concurrent calls.
	st.setCreds(id, func(c connectors.Credentials) { c[KeyExpiresAt] = time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339) })
	_, cur, _ := st.Get(ctx, tenant, id)
	params := map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool), "arguments": map[string]any{"city": map[string]any{"value": "x"}}}
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.Node().Execute(ctx, callInput(id, cur, params))
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, srv.RefreshCalls())
	_, after, _ := st.Get(ctx, tenant, id)
	require.NotEqual(t, cur[KeyAccessToken], after[KeyAccessToken])

	// A token the server forgot is refreshed once on 401; a refused refresh needs a new sign-in.
	srv.Revoke(after[KeyAccessToken])
	srv.RefuseRefresh()
	_, err = c.Node().Execute(ctx, callInput(id, after, params))
	require.Equal(t, catalog.CallUnauthorized, callCode(err), "%v", err)
	require.Equal(t, connectors.StatusNeedsReauth, st.status(id))
	require.NotContains(t, err.Error(), after[KeyAccessToken])
}

// approve follows the fake's authorization endpoint and returns the code of its redirect.
func approve(t *testing.T, srv *mcptest.Server, authURL string) string {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: srv.TLS()}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(authURL)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusFound, res.StatusCode)
	loc, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	return loc.Query().Get("code")
}

func TestToolsAreCachedPerConnectionVersion(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	st := newMemStore()
	id := st.add(tenant, connectors.Credentials{KeyServerURL: srv.URL})
	c := testConnector(t, srv, st, nil)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		tools, err := c.Tools(ctx, tenant, id)
		require.NoError(t, err)
		require.Len(t, tools, 1)
		require.True(t, tools[0].ReadOnly)
		require.Equal(t, "Weather", tools[0].Title)
	}
	require.Equal(t, 1, srv.Requests("tools/list"))
	st.setCreds(id, func(connectors.Credentials) {})
	_, err := c.Tools(ctx, tenant, id)
	require.NoError(t, err)
	require.Equal(t, 2, srv.Requests("tools/list"))
	_, err = c.Tools(ctx, uuidOther, id)
	require.ErrorIs(t, err, connectors.ErrNotFound)
}

var uuidOther = [16]byte{9}

func TestPreparePinsWarnsAndChecksArguments(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(nil)
	srv.SetTool(tool)
	st := newMemStore()
	id := st.add(tenant, connectors.Credentials{KeyServerURL: srv.URL})
	c := testConnector(t, srv, st, nil)
	ctx := context.Background()
	conn := &catalog.ConnectionInfo{ID: id.String(), Kind: Kind}
	typeOf := func(src string) (string, error) {
		if strings.HasPrefix(src, "'") {
			return "string", nil
		}
		return "int", nil
	}
	prepare := func(params map[string]any) catalog.PrepareResult {
		res, err := c.Node().Prepare(ctx, catalog.PrepareInput{TenantID: tenant, NodeID: "n1", Params: params, Connection: conn, Fill: true, TypeOf: typeOf})
		require.NoError(t, err)
		return res
	}

	// No pin: the server's schema is pinned.
	res := prepare(map[string]any{"tool": "weather", "arguments": map[string]any{"city": map[string]any{"template": "{{ event.city }}"}}})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Warnings)
	pinned := res.Params["tool_schema"].(map[string]any)
	require.Equal(t, pinOf(t, tool)["hash"], pinned["hash"])
	require.Equal(t, true, pinned["read_only"])

	// An unknown tool.
	res = prepare(map[string]any{"tool": "nope"})
	require.Equal(t, CodeToolNotFound, res.Errors[0].Code)
	require.Equal(t, "tool", res.Errors[0].Path)

	// Arguments against the pin: a missing required one, a closed object's unknown key, wrong types.
	res = prepare(map[string]any{"tool": "weather", "tool_schema": pinned, "arguments": map[string]any{
		"days": map[string]any{"cel": "'three'"}, "colour": map[string]any{"value": "red"}, "opts.units": map[string]any{"value": "kelvin"},
	}})
	codes := map[string]string{}
	for _, e := range res.Errors {
		codes[e.Path] = e.Code
	}
	require.Equal(t, map[string]string{
		"arguments": CodeArgumentMissing, "arguments.colour": CodeArgumentUnknown,
		"arguments.days.cel": CodeArgumentType, "arguments.opts.units.value": CodeArgumentType,
	}, codes)

	// The server changes the input schema: a warning, the pin stays.
	changed := tool
	changed.InputSchema = map[string]any{"type": "object", "required": []any{"city", "country"}, "properties": map[string]any{"city": map[string]any{"type": "string"}, "country": map[string]any{"type": "string"}}}
	srv.SetTool(changed)
	c.cache.drop(tenant, id)
	res = prepare(map[string]any{"tool": "weather", "tool_schema": pinned, "arguments": map[string]any{"city": map[string]any{"value": "Tashkent"}}})
	require.Empty(t, res.Errors)
	require.Nil(t, res.Params, "the pin is not replaced")
	require.Len(t, res.Warnings, 1)
	require.Equal(t, CodeSchemaChanged, res.Warnings[0].Code)
	require.Equal(t, "tool_schema", res.Warnings[0].Path)

	// A pin edited by hand is refused.
	forged := map[string]any{"input_schema": map[string]any{"type": "object"}, "hash": pinned["hash"]}
	res = prepare(map[string]any{"tool": "weather", "tool_schema": forged})
	require.Equal(t, CodeSchemaInvalid, res.Errors[0].Code)

	// The server is down: a pinned node is only warned about; one without a pin cannot be saved.
	srv.Fail("initialize", mcptest.FaultRateLimited)
	c.cache.drop(tenant, id)
	res = prepare(map[string]any{"tool": "weather", "tool_schema": pinned, "arguments": map[string]any{"city": map[string]any{"value": "x"}}})
	require.Empty(t, res.Errors)
	require.Equal(t, CodeToolsUnavailable, res.Warnings[0].Code)
	srv.Fail("initialize", mcptest.FaultRateLimited)
	res = prepare(map[string]any{"tool": "weather"})
	require.Equal(t, CodeToolsUnavailable, res.Errors[0].Code)
}

func TestSchemaHashIsCanonical(t *testing.T) {
	a, err := SchemaHash(json.RawMessage(`{"type":"object","properties":{"b":{"type":"string"},"a":{"type":"integer"}}}`), nil)
	require.NoError(t, err)
	b, err := SchemaHash(json.RawMessage(`{ "properties": {"a": {"type": "integer"}, "b": {"type": "string"}}, "type": "object" }`), json.RawMessage(`null`))
	require.NoError(t, err)
	require.Equal(t, a, b)
	require.Len(t, a, 64)
	c, err := SchemaHash(json.RawMessage(`{"type":"object"}`), json.RawMessage(`{"type":"object"}`))
	require.NoError(t, err)
	require.NotEqual(t, a, c)
}

func TestCredentialsSchemaOnlyShowsTheFormFields(t *testing.T) {
	d := New().Descriptor()
	internal := connectors.InternalFields(d)
	require.True(t, internal[KeyAccessToken] && internal[KeyRefreshToken] && internal[KeyIssuer] && internal[KeyExpiresAt])
	for _, k := range []string{KeyServerURL, KeyAuth, KeyToken, KeyClientID, KeyClientSecret, KeyScopes} {
		require.False(t, internal[k], k)
	}
	secret := connectors.SecretFields(d)
	require.True(t, secret[KeyToken] && secret[KeyClientSecret] && secret[KeyAccessToken] && secret[KeyRefreshToken])
	shown := connectors.RedactedCredentials(d, connectors.Credentials{KeyServerURL: "https://x", KeyAuth: AuthOAuth2, KeyIssuer: "https://as", KeyAccessToken: "a"})
	require.Equal(t, map[string]string{KeyServerURL: "https://x", KeyAuth: AuthOAuth2}, shown)
	require.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{KeyServerURL: "https://x/mcp", KeyAuth: AuthNone}))
	require.Error(t, connectors.ValidateCredentials(d, connectors.Credentials{KeyServerURL: "http://x/mcp", KeyAuth: AuthNone}))
}
