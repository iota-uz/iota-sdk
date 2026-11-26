package mcp

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"testing"

	"code.dny.dev/ssrf"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/mcp/mcptest"
)

// The reading of a template is decided by the branch the other templates' readings choose, whatever the
// order of the names: a sorts before mode, and mode read as 1 chooses the branch where a is an integer.
func TestReviewTemplateCoercionUsesFinalBranch(t *testing.T) {
	input := json.RawMessage(`{"type":"object","properties":{"mode":{"type":"integer"}},
		"if":{"properties":{"mode":{"const":1}}},"then":{"properties":{"a":{"type":"integer"}}},"else":{"properties":{"a":{"type":"string"}}}}`)
	args, err := buildArguments(input, map[string]any{"a": map[string]any{"template": "2"}, "mode": map[string]any{"template": "1"}})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"a": json.Number("2"), "mode": json.Number("1")}, args)

	// The other branch keeps the text.
	args, err = buildArguments(input, map[string]any{"a": map[string]any{"template": "2"}, "mode": map[string]any{"template": "5"}})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"a": "2", "mode": json.Number("5")}, args)
}

// A then or else that leads through a $ref to $defs is still a branch: while a template decides the
// condition, the required of the branch the placeholder happens to choose is not reported.
func TestReviewBranchThroughARefIsNotJudgedEarly(t *testing.T) {
	input := json.RawMessage(`{"type":"object","$defs":{"needOne":{"required":["one"]},"needOther":{"required":["other"]}},
		"properties":{"mode":{"type":"string"},"one":{"type":"string"},"other":{"type":"string"}},
		"if":{"properties":{"mode":{"const":"chosen"}}},"then":{"$ref":"#/$defs/needOne"},"else":{"$ref":"#/$defs/needOther"}}`)
	tmpl := func(s string) *string { return &s }
	require.Empty(t, checkArguments(input, map[string]Argument{"mode": {Template: tmpl("{{ event.mode }}")}, "one": {Value: json.RawMessage(`"x"`)}}, nil))
	// With literals only, the branch behind the $ref is judged.
	issues := checkArguments(input, map[string]Argument{"mode": {Value: json.RawMessage(`"chosen"`)}}, nil)
	require.Len(t, issues, 1)
	require.Equal(t, CodeArgumentMissing, issues[0].code)

	// A conditional that lives in $defs and is reached through a $ref.
	nested := json.RawMessage(`{"type":"object","$ref":"#/$defs/cond","$defs":{"cond":{
		"if":{"properties":{"mode":{"const":"chosen"}}},"then":{"required":["one"]},"else":{"required":["other"]}}}}`)
	require.Empty(t, checkArguments(nested, map[string]Argument{"mode": {Template: tmpl("{{ event.mode }}")}, "one": {Value: json.RawMessage(`"x"`)}}, nil))
}

// A condition decided by literals is judged even when another argument is a template: the template
// cannot change which branch applies.
func TestReviewBranchChosenByALiteralIsJudged(t *testing.T) {
	input := json.RawMessage(`{"type":"object","properties":{"mode":{"type":"string"},"one":{"type":"string"},"note":{"type":"string"}},
		"if":{"properties":{"mode":{"const":"chosen"}}},"then":{"required":["one"]}}`)
	tmpl := func(s string) *string { return &s }
	issues := checkArguments(input, map[string]Argument{"mode": {Value: json.RawMessage(`"chosen"`)}, "note": {Template: tmpl("{{ event.note }}")}}, nil)
	require.Len(t, issues, 1, "%v", issues)
	require.Equal(t, CodeArgumentMissing, issues[0].code)
	require.Equal(t, "arguments", issues[0].path)

	// The same with anyOf: alternatives that read only literals.
	alternatives := json.RawMessage(`{"type":"object","properties":{"note":{"type":"string"}},
		"anyOf":[{"required":["a"]},{"required":["b"]}]}`)
	issues = checkArguments(alternatives, map[string]Argument{"note": {Template: tmpl("{{ event.note }}")}}, nil)
	require.NotEmpty(t, issues)
	// An alternative that reads the template's value: not judged.
	dyn := json.RawMessage(`{"type":"object","properties":{"note":{"type":"string"}},
		"anyOf":[{"properties":{"note":{"const":"x"}},"required":["a"]},{"required":["b"]}]}`)
	require.Empty(t, checkArguments(dyn, map[string]Argument{"note": {Template: tmpl("{{ event.note }}")}}, nil))
}

// A tools/list of an OAuth connection is kept for the token that read it: after a new sign-in (another
// user, the same settings) the old list is not answered, even at the same version of the row.
func TestReviewToolsOfAnotherSignInAreNotServed(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	srv.RequireToken("tok-a")
	srv.RequireToken("tok-b")
	mem := newMemStore()
	creds := connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthOAuth2, KeyAccessToken: "tok-a", KeyResource: srv.URL}
	id := mem.add(tenant, creds)
	c := New(WithStore(mem), withFake(srv, nil))
	ctx := context.Background()
	_, err := c.Tools(ctx, tenant, id)
	require.NoError(t, err)
	require.Equal(t, 1, srv.Requests("tools/list"))
	_, err = c.Tools(ctx, tenant, id)
	require.NoError(t, err)
	require.Equal(t, 1, srv.Requests("tools/list"), "the same token reuses the list")

	// The row now holds another user's token at the same version: the list is read again.
	mem.mu.Lock()
	mem.creds[id][KeyAccessToken] = "tok-b"
	mem.mu.Unlock()
	_, err = c.Tools(ctx, tenant, id)
	require.NoError(t, err)
	require.Equal(t, 2, srv.Requests("tools/list"))
}

// A sign-in that lands while tools/list is on its way (the same settings, a new access token) is not
// given the list the old token read.
func TestReviewToolsListedBeforeANewSignInAreNotKept(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(weatherTool(nil))
	srv.RequireToken("tok-a")
	mem := newMemStore()
	id := mem.add(tenant, connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthOAuth2, KeyAccessToken: "tok-a", KeyResource: srv.URL})
	st := &changingStore{memStore: mem}
	st.onGet = func(n int) {
		if n == 2 { // after the list, before it is kept
			mem.setCreds(id, func(c connectors.Credentials) { c[KeyAccessToken] = "tok-b" })
		}
	}
	c := New(WithStore(st), withFake(srv, nil))
	_, err := c.Tools(context.Background(), tenant, id)
	require.Equal(t, catalog.CallUnavailable, callCode(err), "%v", err)
	require.False(t, c.cache.has(cacheKey{tenant, id}))
}

// lockStore runs a hook before each row lock, as a change committed between a read and the lock.
type lockStore struct {
	*memStore
	beforeLock func(n int)
	locks      int
}

func (s *lockStore) UpdateCredentials(ctx context.Context, tenant, id uuid.UUID, fn func(connectors.Connection, connectors.Credentials) (*connectors.CredentialChange, error)) (connectors.Connection, connectors.Credentials, error) {
	s.locks++
	if s.beforeLock != nil {
		s.beforeLock(s.locks)
	}
	return s.memStore.UpdateCredentials(ctx, tenant, id, fn)
}

// withFakes lets the connector reach several fakes (they share the test certificate).
func withFakes(srvs ...*mcptest.Server) Option {
	return func(c *Connector) {
		withFake(srvs[0], nil)(c)
		allowed := map[string]bool{}
		for _, s := range srvs {
			_, p := s.Host()
			port, _ := strconv.Atoi(p)
			c.net.ports = append(c.net.ports, uint16(port))
			allowed[s.Addr] = true
		}
		g := ssrf.New(ssrf.WithPorts(c.net.ports...))
		c.net.guard = func(network, address string) error {
			if allowed[address] {
				return nil
			}
			return g.Safe(network, address, nil)
		}
	}
}

// The administrator points the connection at another server while the sign-in discovers the first
// one: the row lock sees the change and the discovery is made again, so the stored issuer, token
// endpoint and client are the new server's and the browser goes to its authorization endpoint.
func TestReviewSignInDiscoversAgainWhenTheServerChanged(t *testing.T) {
	a, b := mcptest.New(t), mcptest.New(t)
	for _, s := range []*mcptest.Server{a, b} {
		s.SetTool(weatherTool(nil))
		s.EnableOAuth()
	}
	mem := newMemStore()
	id := mem.add(tenant, connectors.Credentials{KeyServerURL: a.URL, KeyAuth: AuthOAuth2})
	st := &lockStore{memStore: mem}
	st.beforeLock = func(n int) {
		if n == 1 {
			mem.setCreds(id, func(c connectors.Credentials) { c[KeyServerURL] = b.URL })
		}
	}
	c := New(WithStore(st), withFakes(a, b))
	authURL, _, err := c.BeginAuthorization(context.Background(), connectors.AuthorizationStart{TenantID: tenant, ConnectionID: id, RedirectURL: "https://erp.example.com/cb"})
	require.NoError(t, err)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	require.Equal(t, b.Origin()+"/authorize", u.Scheme+"://"+u.Host+u.Path)
	require.Equal(t, b.URL, u.Query().Get("resource"))
	_, now, err := mem.Get(context.Background(), tenant, id)
	require.NoError(t, err)
	require.Equal(t, b.Origin(), now[KeyIssuer])
	require.Equal(t, b.Origin()+"/token", now[KeyTokenEndpoint])
	require.Equal(t, 0, a.Registrations(), "no client of the old server is kept")
	require.Equal(t, 1, b.Registrations())

	// A server that keeps changing: the sign-in gives up instead of storing a discovery of another one.
	st.beforeLock = func(int) {
		mem.setCreds(id, func(c connectors.Credentials) {
			if c[KeyServerURL] == a.URL {
				c[KeyServerURL] = b.URL
			} else {
				c[KeyServerURL] = a.URL
			}
		})
	}
	_, _, err = c.BeginAuthorization(context.Background(), connectors.AuthorizationStart{TenantID: tenant, ConnectionID: id, RedirectURL: "https://erp.example.com/cb"})
	require.ErrorIs(t, err, connectors.ErrInvalid)
}
