package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/google/uuid"
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
		"opts": map[string]any{"type": "object", "required": []any{"units"}, "properties": map[string]any{
			"units": map[string]any{"type": "string", "enum": []any{"metric", "imperial"}},
			"lang":  map[string]any{"type": "string"},
			"limit": map[string]any{"type": "object", "properties": map[string]any{"max": map[string]any{"type": "integer"}}},
		}},
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
	in, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)
	var out json.RawMessage
	if tool.OutputSchema != nil {
		out, err = json.Marshal(tool.OutputSchema)
		require.NoError(t, err)
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
	require.Equal(t, "fake-mcp 1.2.3 (1 tool)", id.Display)
	require.Equal(t, connectors.IdentityServer, id.Kind)
	require.Equal(t, id, connectors.SafeIdentity(id, func(string) bool { return true }, connectors.Credentials{KeyToken: "tok-1"}), "the page keeps it")
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
			"opts.units": map[string]any{"cel": "metric"}, // rendered: the expression's value
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

	answer = mcptest.Result{Text: "ok, but no structure"}
	_, err = c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.Equal(t, catalog.CallRejected, callCode(err))
	require.Contains(t, err.Error(), "structuredContent")
}

func TestDroppedAnswerAfterToolsCallIsOutcomeUnknownWithoutASecondCall(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(func(map[string]any) mcptest.Result {
		return mcptest.Result{Text: "done", Structured: map[string]any{"temp": 1}}
	})
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

	// 429 before the tool was sent: the engine may repeat the call after Retry-After.
	srv.SetRetryAfter(7)
	srv.Fail("initialize", mcptest.FaultRateLimited)
	res, err := c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.NoError(t, err)
	require.Equal(t, 7*time.Second, res.RetryAfter)
	require.Empty(t, srv.Calls())

	srv.SetRetryAfter(0)
	srv.Fail("initialize", mcptest.FaultRateLimited)
	_, err = c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.Equal(t, catalog.CallRateLimited, callCode(err))

	// 429 to the tools/call itself: the request was sent, so nothing asks for a repeat.
	srv.SetRetryAfter(7)
	srv.Fail("tools/call", mcptest.FaultRateLimited)
	res, err = c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.Equal(t, catalog.CallRejected, callCode(err), "%v", err)
	require.Zero(t, res.RetryAfter)
	require.Equal(t, 1, srv.Requests("tools/call"))

	srv.Fail("tools/call", mcptest.FaultOversized)
	_, err = c.Node().Execute(context.Background(), callInput(id, creds, params))
	require.Equal(t, catalog.CallInvalidRequest, callCode(err), "%v", err)
}

func TestARedirectOfASentToolsCallIsNotFollowed(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(func(map[string]any) mcptest.Result {
		return mcptest.Result{Text: "done", Structured: map[string]any{"temp": 1}}
	})
	srv.SetTool(tool)
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)
	srv.SetRedirect(srv.URL) // a 307 would make net/http send the same tools/call again
	srv.Fail("tools/call", mcptest.FaultRedirect)
	_, err := c.Node().Execute(context.Background(), callInput(id, creds, map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool),
		"arguments": map[string]any{"city": map[string]any{"value": "x"}}}))
	require.Equal(t, catalog.CallOutcomeUnknown, callCode(err), "%v", err)
	require.Equal(t, 1, srv.Requests("tools/call"), "no second tools/call")
	require.Empty(t, srv.Calls())
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
	tool := weatherTool(func(map[string]any) mcptest.Result {
		return mcptest.Result{Text: "ok", Structured: map[string]any{"temp": 1}}
	})
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
	_ = signIn()
	require.Equal(t, 1, srv.Registrations())

	// A state is bound to its connection and does not open for another purpose.
	_, err = c.CompleteAuthorization(ctx, connectors.AuthorizationReturn{TenantID: tenant, ConnectionID: id, State: "forged.state", Code: "x"})
	require.ErrorIs(t, err, ErrState)

	// An access token expiring within 15 minutes is refreshed once, even by concurrent calls.
	st.setCreds(id, func(c connectors.Credentials) {
		c[KeyExpiresAt] = time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339)
	})
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
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, authURL, nil)
	require.NoError(t, err)
	res, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
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

	// Nested: an object literal is validated whole; a dotted key into an object needs its siblings
	// the schema requires; a nested literal of the wrong type is refused.
	res = prepare(map[string]any{"tool": "weather", "tool_schema": pinned, "arguments": map[string]any{
		"city": map[string]any{"value": "x"}, "opts": map[string]any{"value": map[string]any{"lang": "en"}},
	}})
	require.Len(t, res.Errors, 1)
	require.Equal(t, "arguments.opts.value", res.Errors[0].Path)
	require.Contains(t, res.Errors[0].Message, "units")
	res = prepare(map[string]any{"tool": "weather", "tool_schema": pinned, "arguments": map[string]any{
		"city": map[string]any{"value": "x"}, "opts.lang": map[string]any{"value": "en"}, "opts.limit": map[string]any{"value": map[string]any{"max": "ten"}},
	}})
	codes = map[string]string{}
	for _, e := range res.Errors {
		codes[e.Path] = e.Code
	}
	require.Equal(t, map[string]string{"arguments": CodeArgumentMissing, "arguments.opts.limit.value": CodeArgumentType}, codes)
	res = prepare(map[string]any{"tool": "weather", "tool_schema": pinned, "arguments": map[string]any{
		"city": map[string]any{"value": "x"}, "opts.units": map[string]any{"template": "metric"}, "opts.limit.max": map[string]any{"cel": "'a'"},
	}})
	require.Len(t, res.Errors, 1)
	require.Equal(t, "arguments.opts.limit.max.cel", res.Errors[0].Path)

	// The server changes the input schema: a warning, the pin stays.
	changed := tool
	changed.InputSchema = map[string]any{"type": "object", "required": []any{"city", "country"}, "properties": map[string]any{"city": map[string]any{"type": "string"}, "country": map[string]any{"type": "string"}}}
	srv.SetTool(changed)
	c.cache.drop(tenant, id)
	res = prepare(map[string]any{"tool": "weather", "tool_schema": pinned, "arguments": map[string]any{"city": map[string]any{"value": "Tashkent"}}})
	require.Empty(t, res.Errors)
	require.Equal(t, pinned["hash"], res.Params["tool_schema"].(map[string]any)["hash"], "the pinned schema stays")
	require.NotEqual(t, true, res.Params["tool_schema"].(map[string]any)["read_only"], "a changed tool is no longer run in a dry run")
	require.Len(t, res.Warnings, 1)
	require.Equal(t, CodeSchemaChanged, res.Warnings[0].Code)
	require.Equal(t, "tool_schema", res.Warnings[0].Path)

	// read_only comes from the server, never from the graph: a forged true on a tool that is not
	// read-only (or a pin whose schema the server changed) is set back to false.
	srv.SetTool(mcptest.Tool{Name: "write", InputSchema: map[string]any{"type": "object"}})
	c.cache.drop(tenant, id)
	res = prepare(map[string]any{"tool": "write"})
	writePin := res.Params["tool_schema"].(map[string]any)
	require.NotEqual(t, true, writePin["read_only"])
	writePin["read_only"] = true
	res = prepare(map[string]any{"tool": "write", "tool_schema": writePin})
	require.NotEqual(t, true, res.Params["tool_schema"].(map[string]any)["read_only"], "a forged read_only is set back")
	res, err := c.Node().Prepare(ctx, catalog.PrepareInput{TenantID: tenant, NodeID: "n1", Params: map[string]any{"tool": "write", "tool_schema": writePin}, Connection: conn})
	require.NoError(t, err)
	require.Nil(t, res.Params, "a stored version being enabled keeps its params")

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

func TestServerEchoesOfSecretsAreScrubbed(t *testing.T) {
	srv := mcptest.New(t)
	const token = "tok-SECRET-123"
	tool := weatherTool(func(map[string]any) mcptest.Result {
		return mcptest.Result{Text: "your token is " + token, Structured: map[string]any{"temp": 1, "echo": token}}
	})
	tool.Title = "Weather for " + token
	srv.SetTool(tool)
	srv.SetTool(mcptest.Tool{Name: "leak_" + token, InputSchema: map[string]any{"type": "object"}})
	srv.RequireToken(token)
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthBearer, KeyToken: token}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)
	ctx := context.Background()

	tools, err := c.Tools(ctx, tenant, id)
	require.NoError(t, err)
	raw, err := json.Marshal(tools)
	require.NoError(t, err)
	require.NotContains(t, string(raw), token)
	require.Len(t, tools, 1, "a tool whose name carries the token is left out")

	res, err := c.Node().Execute(ctx, callInput(id, creds, map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool),
		"arguments": map[string]any{"city": map[string]any{"value": "x"}}}))
	require.NoError(t, err)
	out, err := json.Marshal(res.Output)
	require.NoError(t, err)
	require.NotContains(t, string(out), token)
	require.Contains(t, string(out), "***")
}

func TestTokenRequestsDoNotFollowRedirects(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	srv.EnableOAuth()
	st := newMemStore()
	id := st.add(tenant, connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthOAuth2})
	c := testConnector(t, srv, st, nil)
	ctx := context.Background()
	authURL, state, err := c.BeginAuthorization(ctx, connectors.AuthorizationStart{TenantID: tenant, ConnectionID: id, RedirectURL: "https://erp.example.com/cb"})
	require.NoError(t, err)
	code := approve(t, srv, authURL)
	srv.MoveTokenEndpoint()
	_, err = c.CompleteAuthorization(ctx, connectors.AuthorizationReturn{TenantID: tenant, ConnectionID: id, State: state, Code: code})
	require.ErrorIs(t, err, ErrExchange)
	require.Zero(t, srv.Elsewhere(), "the code and its verifier are not sent to the redirect")
}

func TestRefreshDoesNotLendATokenOfNewSettingsToTheOldServer(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(nil)
	srv.SetTool(tool)
	st := newMemStore()
	read := connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthOAuth2, KeyAccessToken: "old-access", KeyRefreshToken: "r",
		KeyResource: srv.URL, KeyIssuer: srv.Origin(), KeyTokenEndpoint: srv.Origin() + "/token", KeyClientID: "c1",
		KeyExpiresAt: time.Now().Add(time.Minute).UTC().Format(time.RFC3339)}
	id := st.add(tenant, read)
	// The administrator points the connection at another server and signs in there before the lock.
	st.setCreds(id, func(c connectors.Credentials) {
		c[KeyServerURL], c[KeyResource], c[KeyAccessToken] = "https://other.example.com/mcp", "https://other.example.com/mcp", "new-access"
		c[KeyExpiresAt] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	})
	c := testConnector(t, srv, st, nil)
	_, err := c.Node().Execute(context.Background(), callInput(id, read, map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool),
		"arguments": map[string]any{"city": map[string]any{"value": "x"}}}))
	require.Equal(t, catalog.CallUnavailable, callCode(err), "%v", err)
	require.Zero(t, srv.Requests("initialize"), "nothing is sent to the old server")
}

func TestAHangingSessionEndDoesNotHoldTheAnswer(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(func(map[string]any) mcptest.Result {
		return mcptest.Result{Text: "ok", Structured: map[string]any{"temp": 1}}
	})
	srv.SetTool(tool)
	srv.HangDelete()
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)
	started := time.Now()
	res, err := c.Node().Execute(context.Background(), callInput(id, creds, map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool),
		"arguments": map[string]any{"city": map[string]any{"value": "x"}}}))
	require.NoError(t, err)
	require.Equal(t, OutcomeDone, res.Outcome)
	require.Less(t, time.Since(started), 3*time.Second)
}

func TestA403AfterToolsCallIsNotSentAgain(t *testing.T) {
	srv := mcptest.New(t)
	tool := weatherTool(func(map[string]any) mcptest.Result {
		return mcptest.Result{Text: "ok", Structured: map[string]any{"temp": 1}}
	})
	srv.SetTool(tool)
	srv.RequireToken("acc-1")
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthOAuth2, KeyAccessToken: "acc-1", KeyRefreshToken: "r", KeyResource: srv.URL,
		KeyIssuer: srv.Origin(), KeyTokenEndpoint: srv.Origin() + "/token", KeyClientID: "c1"}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)
	srv.Fail("tools/call", mcptest.FaultForbidden)
	_, err := c.Node().Execute(context.Background(), callInput(id, creds, map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool),
		"arguments": map[string]any{"city": map[string]any{"value": "x"}}}))
	require.Equal(t, catalog.CallRejected, callCode(err), "%v", err)
	require.Equal(t, 1, srv.Requests("tools/call"), "no refresh and no second tools/call on 403")
	require.Zero(t, srv.RefreshCalls())
	require.Equal(t, connectors.StatusConnected, st.status(id))
}

func TestShortSecretsAreNotEchoed(t *testing.T) {
	srv := mcptest.New(t)
	const token = "k9"
	srv.SetTool(mcptest.Tool{Name: "echo", Title: "Echo " + token, InputSchema: map[string]any{"type": "object"}})
	srv.SetTool(mcptest.Tool{Name: "plain", Title: "Plain", InputSchema: map[string]any{"type": "object", "description": "uses " + token}})
	srv.SetTool(mcptest.Tool{Name: "fine", Title: "Fine", InputSchema: map[string]any{"type": "object"},
		Handle: func(map[string]any) mcptest.Result { return mcptest.Result{Text: "your key is " + token} }})
	srv.RequireToken(token)
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthBearer, KeyToken: token}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)
	tools, err := c.Tools(context.Background(), tenant, id)
	require.NoError(t, err)
	require.Len(t, tools, 1, "tools whose metadata carry the short token are left out")
	require.Equal(t, "fine", tools[0].Name)
	pin, err := PinOf(tools[0])
	require.NoError(t, err)
	pv, err := pinValue(pin)
	require.NoError(t, err)
	res, err := c.Node().Execute(context.Background(), callInput(id, creds, map[string]any{"tool": "fine", "tool_schema": pv}))
	require.NoError(t, err)
	require.NotContains(t, res.Output["text"], token)
}

func TestTheResponseCapAcceptsAnExactFit(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    func([]byte) io.Reader
	}{
		{"whole", func(b []byte) io.Reader { return bytes.NewReader(b) }},
		{"chunked", func(b []byte) io.Reader { return iotest.HalfReader(bytes.NewReader(b)) }},
		{"eof with data", func(b []byte) io.Reader { return iotest.DataErrReader(bytes.NewReader(b)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := newObserver("https://x", "")
			exact := &cappedBody{r: io.NopCloser(tc.r(bytes.Repeat([]byte("a"), MaxResponseBytes))), left: MaxResponseBytes, o: o, method: "tools/call"}
			n, err := io.Copy(io.Discard, exact)
			require.NoError(t, err)
			require.EqualValues(t, MaxResponseBytes, n)
			require.False(t, o.overflowed())
			require.True(t, o.completed("tools/call"))

			o = newObserver("https://x", "")
			over := &cappedBody{r: io.NopCloser(tc.r(bytes.Repeat([]byte("a"), MaxResponseBytes+1))), left: MaxResponseBytes, o: o}
			_, err = io.Copy(io.Discard, over)
			require.ErrorIs(t, err, errTooLarge)
			require.True(t, o.overflowed())
		})
	}
}

// changingStore changes the connection's server while a request is on its way.
type changingStore struct {
	*memStore
	onGet func(n int)
	gets  int
}

func (s *changingStore) Get(ctx context.Context, tenant, id uuid.UUID) (connectors.Connection, connectors.Credentials, error) {
	s.gets++
	if s.onGet != nil {
		s.onGet(s.gets)
	}
	return s.memStore.Get(ctx, tenant, id)
}

func TestToolsOfAServerReplacedMidRequestAreNotKept(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	mem := newMemStore()
	id := mem.add(tenant, connectors.Credentials{KeyServerURL: srv.URL})
	st := &changingStore{memStore: mem}
	st.onGet = func(n int) {
		if n == 2 { // after the list, before it is kept
			mem.setCreds(id, func(c connectors.Credentials) { c[KeyServerURL] = "https://other.example.com/mcp" })
		}
	}
	c := New(WithStore(st), withFake(srv, nil))
	_, err := c.Tools(context.Background(), tenant, id)
	require.Equal(t, catalog.CallUnavailable, callCode(err), "%v", err)
	st.onGet = nil
	ok := c.cache.has(cacheKey{tenant, id})
	require.False(t, ok, "nothing is cached for the new settings")
}

func TestALateRefusalDoesNotUndoANewSignIn(t *testing.T) {
	st := newMemStore()
	used := connectors.Credentials{KeyServerURL: "https://a.example.com/mcp", KeyAuth: AuthOAuth2, KeyAccessToken: "old", KeyClientID: "c1"}
	id := st.add(tenant, used)
	c := New(WithStore(st))
	// The administrator signs in again before the refusal of the old token is recorded.
	st.setCreds(id, func(c connectors.Credentials) { c[KeyAccessToken] = "new" })
	c.markReauth(context.Background(), tenant, id, used, "old", "refused")
	require.Equal(t, connectors.StatusConnected, st.status(id))
	c.markReauth(context.Background(), tenant, id, used, "new", "refused")
	require.Equal(t, connectors.StatusNeedsReauth, st.status(id), "the token of the row now is refused")
}

func TestArgumentsAreCheckedWithRefsAndComposition(t *testing.T) {
	input, err := json.Marshal(map[string]any{
		"type": "object",
		"$defs": map[string]any{
			"filter": map[string]any{"type": "object", "required": []any{"field"}, "properties": map[string]any{
				"field": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}},
		},
		"properties": map[string]any{
			"filter": map[string]any{"$ref": "#/$defs/filter"},
			"tags":   map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer"}},
			"mode":   map[string]any{"allOf": []any{map[string]any{"type": "string"}, map[string]any{"enum": []any{"a", "b"}}}},
		},
	})
	require.NoError(t, err)
	str := func(s string) *string { return &s }
	typeOf := func(src string) (string, error) {
		if strings.HasPrefix(src, "'") {
			return "string", nil
		}
		return "int", nil
	}
	codes := func(args map[string]Argument) map[string]string {
		out := map[string]string{}
		for _, i := range checkArguments(input, args, typeOf) {
			out[i.path] = i.code
		}
		return out
	}
	// A dotted key into a $ref'd object still needs its required sibling.
	require.Equal(t, map[string]string{"arguments": CodeArgumentMissing}, codes(map[string]Argument{"filter.limit": {CEL: str("1")}}))
	// additionalProperties as a schema, allOf with an enum, and an expression typed against a $ref.
	require.Equal(t, map[string]string{
		"arguments.tags.value": CodeArgumentType, "arguments.mode.value": CodeArgumentType, "arguments.filter.field.cel": CodeArgumentType,
	}, codes(map[string]Argument{
		"tags": {Value: json.RawMessage(`{"x": "one"}`)}, "mode": {Value: json.RawMessage(`"c"`)}, "filter.field": {CEL: str("2")},
	}))
	require.Empty(t, codes(map[string]Argument{"filter.field": {Template: str("{{ x }}")}, "filter.limit": {CEL: str("1")}, "mode": {Template: str("a")}}))
}

func TestTemplatesAreReadByTheSchemaTheSaveChecked(t *testing.T) {
	input := json.RawMessage(`{"type":"object","$defs":{"n":{"type":"integer"}},"properties":{"count":{"$ref":"#/$defs/n"},"flag":{"allOf":[{"type":"boolean"}]},"name":{"type":"string"}}}`)
	tmpl := func(s string) *string { return &s }
	require.Empty(t, checkArguments(input, map[string]Argument{"count": {Template: tmpl("{{ x }}")}, "flag": {Template: tmpl("{{ y }}")}}, nil))
	args, err := buildArguments(input, map[string]any{
		"count": map[string]any{"template": "2"}, "flag": map[string]any{"template": "true"}, "name": map[string]any{"template": "7"},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"count": json.Number("2"), "flag": true, "name": "7"}, args)
}

func TestABranchChosenByATemplateIsNotJudgedEarly(t *testing.T) {
	input := json.RawMessage(`{"type":"object","properties":{"mode":{"type":"string"},"one":{"type":"string"},"other":{"type":"string"}},
		"if":{"properties":{"mode":{"const":"chosen"}}},"then":{"required":["one"]},"else":{"required":["other"]}}`)
	tmpl := func(s string) *string { return &s }
	require.Empty(t, checkArguments(input, map[string]Argument{"mode": {Template: tmpl("{{ event.mode }}")}, "one": {Value: json.RawMessage(`"x"`)}}, nil))
	// With literals only the branch is known and judged.
	issues := checkArguments(input, map[string]Argument{"mode": {Value: json.RawMessage(`"chosen"`)}}, nil)
	require.Len(t, issues, 1)
	require.Equal(t, CodeArgumentMissing, issues[0].code)
}

func TestToolsListedWithARotatedTokenAreNotKept(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	srv.RequireToken("tok-old")
	srv.RequireToken("tok-new")
	mem := newMemStore()
	id := mem.add(tenant, connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthBearer, KeyToken: "tok-old"})
	st := &changingStore{memStore: mem}
	st.onGet = func(n int) {
		if n == 2 {
			mem.setCreds(id, func(c connectors.Credentials) { c[KeyToken] = "tok-new" })
		}
	}
	c := New(WithStore(st), withFake(srv, nil))
	_, err := c.Tools(context.Background(), tenant, id)
	require.Equal(t, catalog.CallUnavailable, callCode(err), "%v", err)
	ok := c.cache.has(cacheKey{tenant, id})
	require.False(t, ok)
}

func TestAStructuredResultThatLosesItsShapeToTheScrubIsRefused(t *testing.T) {
	srv := mcptest.New(t)
	const token = "k9"
	tool := weatherTool(func(map[string]any) mcptest.Result {
		return mcptest.Result{Text: "ok", Structured: map[string]any{"temp": 1, "note": "key " + token}}
	})
	srv.SetTool(tool)
	srv.RequireToken(token)
	st := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthBearer, KeyToken: token}
	id := st.add(tenant, creds)
	c := testConnector(t, srv, st, nil)
	_, err := c.Node().Execute(context.Background(), callInput(id, creds, map[string]any{"tool": "weather", "tool_schema": pinOf(t, tool),
		"arguments": map[string]any{"city": map[string]any{"value": "x"}}}))
	require.Equal(t, catalog.CallRejected, callCode(err), "%v", err)
	require.NotContains(t, err.Error(), token)
}

func TestACodeForAnotherClientIsNotStored(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	srv.EnableOAuth()
	st := newMemStore()
	id := st.add(tenant, connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthOAuth2})
	c := testConnector(t, srv, st, nil)
	ctx := context.Background()
	authURL, state, err := c.BeginAuthorization(ctx, connectors.AuthorizationStart{TenantID: tenant, ConnectionID: id, RedirectURL: "https://erp.example.com/cb"})
	require.NoError(t, err)
	code := approve(t, srv, authURL)
	// The administrator enters another client while the browser is away.
	st.setCreds(id, func(c connectors.Credentials) { c[KeyClientID] = "admin-client" })
	_, err = c.CompleteAuthorization(ctx, connectors.AuthorizationReturn{TenantID: tenant, ConnectionID: id, State: state, Code: code})
	require.ErrorIs(t, err, ErrState)
	_, now, _ := st.Get(ctx, tenant, id)
	require.Empty(t, now[KeyAccessToken])
}
