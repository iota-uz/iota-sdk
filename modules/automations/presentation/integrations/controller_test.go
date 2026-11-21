package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

const secret = "123456:AAH-very-secret-bot-token"

type bot struct{}

func (bot) Kind() catalog.ConnectorKind       { return catalog.KindTelegramBot }
func (bot) Nodes() []catalog.NodeType         { return nil }
func (bot) Triggers() []catalog.TriggerSource { return nil }
func (bot) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind: catalog.KindTelegramBot, Title: map[string]string{"en": "Telegram bot", "ru": "Телеграм-бот"}, Auth: connectors.AuthAPIKey,
		Category: connectors.CategoryMessaging, Description: map[string]string{"en": "Messages"},
		Credentials: json.RawMessage(`{"type":"object","required":["token"],"properties":{"token":{"type":"string","x-secret":true,"x-label":{"en":"Bot token","ru":"Токен бота"}},"chat":{"type":"string"}}}`),
	}
}
func (bot) Test(_ context.Context, c connectors.Credentials) (connectors.Identity, error) {
	if c["token"] != secret {
		return connectors.Identity{}, fmt.Errorf("token %s refused", c["token"])
	}
	return connectors.Identity{Display: "@acme_bot", Kind: connectors.IdentityHandle}, nil
}

// drive is an OAuth2 connector; oauth is its provider.
type drive struct{}

func (drive) Kind() catalog.ConnectorKind       { return "drive" }
func (drive) Nodes() []catalog.NodeType         { return nil }
func (drive) Triggers() []catalog.TriggerSource { return nil }
func (drive) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{Kind: "drive", Title: map[string]string{"en": "Drive"}, Auth: connectors.AuthOAuth2,
		OAuth2: &connectors.OAuth2Config{AuthURL: "https://provider.test/auth", TokenURL: "https://provider.test/token"}}
}
func (drive) Test(context.Context, connectors.Credentials) (connectors.Identity, error) {
	return connectors.Identity{Display: "docs@acme.test", Kind: connectors.IdentityEmail}, nil
}

type oauth struct {
	err   error
	state string
}

func (o *oauth) Begin(_ context.Context, _ uuid.UUID, _ catalog.ConnectorKind, _ string) (string, string, error) {
	o.state = uuid.NewString()
	return "https://provider.test/auth?state=" + o.state, o.state, nil
}
func (o *oauth) Exchange(context.Context, uuid.UUID, string, string) (connectors.OAuth2Grant, error) {
	if o.err != nil {
		return connectors.OAuth2Grant{}, o.err
	}
	return connectors.OAuth2Grant{Kind: "drive", Identity: connectors.Identity{Display: "docs@acme.test", Kind: connectors.IdentityEmail}, Credentials: connectors.Credentials{"access_token": "at"}}, nil
}
func (o *oauth) Complete(context.Context, uuid.UUID, string, string) (connectors.Connection, error) {
	panic("not used")
}
func (o *oauth) Token(context.Context, uuid.UUID, uuid.UUID) (string, error) { return "at", nil }

type fixture struct {
	oauth  *oauth
	userID int64
	t      *testing.T
	ctrl   *Controller
	tenant uuid.UUID
	other  uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_integrations_rpc",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{3}, 32)})
	require.NoError(t, err)
	dir := connectors.NewDirectory(nil)
	require.NoError(t, dir.Register(bot{}))
	require.NoError(t, dir.Register(drive{}))
	store := connectors.NewStore(scope.Pool, keys, nil, nil, nil)
	fake := &oauth{}
	svc := services.NewIntegrationsService(services.IntegrationsDeps{
		Directory: dir, Store: store, States: persistence.NewOAuthStates(scope.Pool), Checks: persistence.NewConnectionChecks(scope.Pool),
		Requests: persistence.NewRequests(scope.Pool), OAuth: fake,
	})
	var userID int64
	require.NoError(t, scope.Pool.QueryRow(context.Background(), `INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		VALUES ($1, 'user', 'Ann', 'Admin', $2, 'en') RETURNING id`, scope.Tenant.ID, "admin-"+uuid.NewString()+"@example.com").Scan(&userID))
	other := uuid.New()
	_, err = scope.Pool.Exec(context.Background(), `INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`,
		other, "t-"+other.String()[:8], other.String()[:8]+".test")
	require.NoError(t, err)
	ctrl, err := NewController(Config{Service: svc, Configured: true})
	require.NoError(t, err)
	return &fixture{t: t, ctrl: ctrl, tenant: scope.Tenant.ID, other: other, oauth: fake, userID: userID}
}

func testUser(tenant uuid.UUID, perms ...permission.Permission) user.User {
	return testUserID(tenant, 7, perms...)
}

func testUserID(tenant uuid.UUID, id int64, perms ...permission.Permission) user.User {
	r := role.New("r", role.WithID(1), role.WithPermissions(perms), role.WithTenantID(tenant), role.WithCreatedAt(time.Now()), role.WithUpdatedAt(time.Now()))
	email, _ := internet.NewEmail("admin@example.com")
	return user.New("Ann", "Admin", email, "", user.WithID(uint(id)), user.WithTenantID(tenant), user.WithRoles([]role.Role{r}))
}

// call posts one JSON-RPC request as u and returns the raw body.
func (f *fixture) call(u user.User, method string, params any) (int, string) {
	f.t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	require.NoError(f.t, err)
	req := httptest.NewRequest(http.MethodPost, "/settings/integrations/rpc", bytes.NewReader(body))
	if u != nil {
		req = req.WithContext(composables.WithUser(req.Context(), u))
	}
	rec := httptest.NewRecorder()
	withRequest(f.ctrl.rpc).ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func result[T any](t *testing.T, body string) T {
	t.Helper()
	var env struct {
		Result T               `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &env), body)
	require.Empty(t, env.Error, body)
	return env.Result
}

func TestRPCNeverReturnsSecrets(t *testing.T) {
	f := newFixture(t)
	admin := testUser(f.tenant, permissions.ConnectionsManage)

	_, body := f.call(admin, MethodCreate, map[string]any{"kind": "telegram_bot", "name": "sales_bot", "display_name": "Sales", "credentials": map[string]any{"token": "wrong-but-secret-token"}})
	failed := result[SaveResponse](t, body)
	assert.Equal(t, ErrTestFailed, failed.Error)
	assert.NotContains(t, body, "wrong-but-secret-token", "a failed Test does not echo the secret")

	_, body = f.call(admin, MethodCreate, map[string]any{"kind": "telegram_bot", "name": "sales_bot", "display_name": "Sales", "credentials": map[string]any{"token": secret, "chat": "-100"}})
	saved := result[SaveResponse](t, body)
	require.True(t, saved.OK, body)
	assert.NotContains(t, body, secret)
	assert.Equal(t, map[string]string{"chat": "-100"}, saved.Connection.Fields)

	_, body = f.call(admin, MethodList, map[string]any{})
	list := result[ListResponse](t, body)
	assert.NotContains(t, body, secret)
	require.Len(t, list.Connectors, 2)
	list.Connectors = list.Connectors[1:] // drive, telegram_bot: by kind
	assert.Equal(t, CategoryMessaging, list.Connectors[0].Category)
	assert.True(t, list.CanManage)
	require.Len(t, list.Connectors[0].Connections, 1)
	assert.Equal(t, "@acme_bot", *list.Connectors[0].Connections[0].Identity)
	schema := list.Connectors[0].CredentialSchema.(map[string]any)
	token := schema["properties"].(map[string]any)["token"].(map[string]any)
	assert.Equal(t, "Bot token", token["title"], "x-label resolved for the locale")
	assert.Equal(t, true, token["x-secret"])

	id := saved.Connection.ID
	for _, m := range []struct {
		method string
		params any
	}{
		{MethodUpdate, map[string]any{"id": id, "version": saved.Connection.Version, "display_name": "Renamed"}},
		{MethodTest, map[string]any{"id": id}},
		{MethodSetDisabled, map[string]any{"id": id, "disabled": true}},
		{MethodUsage, map[string]any{"id": id}},
	} {
		_, body := f.call(admin, m.method, m.params)
		assert.NotContains(t, body, secret, m.method)
		assert.NotContains(t, body, `"error"`, m.method+": "+body)
	}
}

func TestRPCPermissionsAndTenants(t *testing.T) {
	f := newFixture(t)
	admin := testUser(f.tenant, permissions.ConnectionsManage)
	author := testUser(f.tenant, permissions.AutomationsAuthor)
	nobody := testUser(f.tenant)
	stranger := testUser(f.other, permissions.ConnectionsManage)

	_, body := f.call(admin, MethodCreate, map[string]any{"kind": "telegram_bot", "name": "sales_bot", "display_name": "Sales", "credentials": map[string]any{"token": secret}})
	saved := result[SaveResponse](t, body)
	require.True(t, saved.OK, body)
	id := saved.Connection.ID

	// An author lists connections (to pick one in a node) without administrator fields.
	_, body = f.call(author, MethodList, map[string]any{})
	list := result[ListResponse](t, body)
	assert.False(t, list.CanManage)
	list.Connectors = list.Connectors[1:]
	require.Len(t, list.Connectors[0].Connections, 1)
	assert.Nil(t, list.Connectors[0].Connections[0].Fields)

	code, body := f.call(nobody, MethodList, map[string]any{})
	assert.Equal(t, http.StatusForbidden, code)
	assert.Contains(t, body, `"code":"forbidden"`)
	for _, m := range []string{MethodCreate, MethodUpdate, MethodTest, MethodSetDisabled, MethodDelete, MethodUsage, MethodPlatform, MethodOAuthStart} {
		code, body := f.call(author, m, map[string]any{"id": id, "kind": "telegram_bot"})
		assert.Equal(t, http.StatusForbidden, code, m)
		assert.Contains(t, body, `"code":"forbidden"`, m)
	}
	code, _ = f.call(nil, MethodList, map[string]any{})
	assert.Equal(t, http.StatusUnauthorized, code)

	// Another tenant's administrator gets not_found for the id.
	for _, m := range []struct {
		method string
		params any
	}{
		{MethodTest, map[string]any{"id": id}},
		{MethodUpdate, map[string]any{"id": id, "version": 1, "display_name": "x"}},
		{MethodDelete, map[string]any{"id": id}},
		{MethodSetDisabled, map[string]any{"id": id, "disabled": true}},
		{MethodUsage, map[string]any{"id": id}},
	} {
		_, body := f.call(stranger, m.method, m.params)
		assert.Contains(t, body, `"code":"not_found"`, m.method+": "+body)
	}
	_, body = f.call(admin, MethodTest, map[string]any{"id": "not-a-uuid"})
	assert.Contains(t, body, `"code":"not_found"`)

	// Delete removes it; a second delete is not_found.
	_, body = f.call(admin, MethodDelete, map[string]any{"id": id})
	assert.True(t, result[DeleteResponse](t, body).OK)
	_, body = f.call(admin, MethodDelete, map[string]any{"id": id})
	assert.Contains(t, body, `"code":"not_found"`)
}

func TestRPCNameTakenAndInvalidInput(t *testing.T) {
	f := newFixture(t)
	admin := testUser(f.tenant, permissions.ConnectionsManage)
	create := map[string]any{"kind": "telegram_bot", "name": "sales_bot", "display_name": "Sales", "credentials": map[string]any{"token": secret}}
	_, body := f.call(admin, MethodCreate, create)
	require.True(t, result[SaveResponse](t, body).OK)
	_, body = f.call(admin, MethodCreate, create)
	taken := result[SaveResponse](t, body)
	assert.Equal(t, ErrNameTaken, taken.Error)
	assert.Equal(t, map[string]string{"name": "taken"}, taken.Fields)

	_, body = f.call(admin, MethodCreate, map[string]any{"kind": "telegram_bot", "name": "Bad", "credentials": map[string]any{}})
	invalid := result[SaveResponse](t, body)
	assert.Equal(t, ErrInvalidInput, invalid.Error)
	assert.Equal(t, map[string]string{"name": "name", "token": "required"}, invalid.Fields)
	_, body = f.call(admin, MethodCreate, map[string]any{"kind": "nope", "name": "x"})
	assert.Contains(t, body, `"code":"not_found"`)
}

func TestOAuthCallbackKeepsTheFormContext(t *testing.T) {
	f := newFixture(t)
	admin := testUserID(f.tenant, f.userID, permissions.ConnectionsManage)
	start := func(params map[string]any) string {
		t.Helper()
		_, body := f.call(admin, MethodOAuthStart, params)
		require.NotEmpty(t, result[OAuthStartResponse](t, body).AuthorizeURL, body)
		return f.oauth.state
	}
	callback := func(q url.Values) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/settings/integrations/oauth/callback?"+q.Encode(), nil)
		req = req.WithContext(composables.WithUser(req.Context(), admin))
		rec := httptest.NewRecorder()
		f.ctrl.oauthCallback(rec, req)
		require.Equal(t, http.StatusFound, rec.Code)
		loc, err := url.Parse(rec.Header().Get("Location"))
		require.NoError(t, err)
		return loc.Query().Encode()
	}

	// The user refuses at the provider: the create form comes back with its names.
	state := start(map[string]any{"kind": "drive", "name": "company_drive", "display_name": "Company drive"})
	assert.Equal(t, url.Values{"oauth_error": {"denied"}, "kind": {"drive"}, "name": {"company_drive"}, "display_name": {"Company drive"}}.Encode(),
		callback(url.Values{"state": {state}, "error": {"access_denied"}}))

	// The provider's token is refused by the connector.
	f.oauth.err = connectors.ErrOAuth2Refused
	state = start(map[string]any{"kind": "drive", "name": "company_drive"})
	assert.Equal(t, url.Values{"oauth_error": {"refused"}, "kind": {"drive"}, "name": {"company_drive"}}.Encode(),
		callback(url.Values{"state": {state}, "code": {"c"}}))

	// Success creates the connection under the requested name.
	f.oauth.err = nil
	state = start(map[string]any{"kind": "drive", "name": "company_drive"})
	got, err := url.ParseQuery(callback(url.Values{"state": {state}, "code": {"c"}}))
	require.NoError(t, err)
	id := got.Get("connected")
	require.NotEmpty(t, id)
	assert.Equal(t, "drive", got.Get("kind"))

	// A re-authorization that fails returns to the edit form of that connection.
	f.oauth.err = fmt.Errorf("oauth2: server error")
	state = start(map[string]any{"kind": "drive", "name": "company_drive", "connection_id": id})
	assert.Equal(t, url.Values{"oauth_error": {"exchange"}, "kind": {"drive"}, "edit": {id}}.Encode(),
		callback(url.Values{"state": {state}, "code": {"c"}}))
	// … and a successful one too.
	f.oauth.err = nil
	state = start(map[string]any{"kind": "drive", "name": "company_drive", "connection_id": id})
	assert.Equal(t, url.Values{"connected": {id}, "kind": {"drive"}, "edit": {id}}.Encode(), callback(url.Values{"state": {state}, "code": {"c"}}))

	// A state that is not this user's carries no context.
	assert.Equal(t, url.Values{"oauth_error": {"state"}}.Encode(), callback(url.Values{"state": {"forged"}, "code": {"c"}}))
}
