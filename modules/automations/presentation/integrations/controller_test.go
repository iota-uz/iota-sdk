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
	"strings"
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
	return connectors.Identity{Display: "@bot"}, nil
}

type fixture struct {
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
	store := connectors.NewStore(scope.Pool, keys, nil, nil, nil)
	svc := services.NewIntegrationsService(services.IntegrationsDeps{
		Directory: dir, Store: store, States: persistence.NewOAuthStates(scope.Pool), Checks: persistence.NewConnectionChecks(scope.Pool),
	})
	other := uuid.New()
	_, err = scope.Pool.Exec(context.Background(), `INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`,
		other, "t-"+other.String()[:8], other.String()[:8]+".test")
	require.NoError(t, err)
	ctrl, err := NewController(Config{Service: svc, Configured: true})
	require.NoError(t, err)
	return &fixture{t: t, ctrl: ctrl, tenant: scope.Tenant.ID, other: other}
}

func testUser(tenant uuid.UUID, perms ...permission.Permission) user.User {
	r := role.New("r", role.WithID(1), role.WithPermissions(perms), role.WithTenantID(tenant), role.WithCreatedAt(time.Now()), role.WithUpdatedAt(time.Now()))
	email, _ := internet.NewEmail("admin@example.com")
	return user.New("Ann", "Admin", email, "", user.WithID(7), user.WithTenantID(tenant), user.WithRoles([]role.Role{r}))
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
	require.Len(t, list.Connectors, 1)
	assert.Equal(t, CategoryMessaging, list.Connectors[0].Category)
	assert.True(t, list.CanManage)
	require.Len(t, list.Connectors[0].Connections, 1)
	assert.Equal(t, "@bot", *list.Connectors[0].Connections[0].Identity)
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

func TestOAuthCallbackRedirects(t *testing.T) {
	f := newFixture(t)
	admin := testUser(f.tenant, permissions.ConnectionsManage)
	get := func(q url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/settings/integrations/oauth/callback?"+q.Encode(), nil)
		req = req.WithContext(composables.WithUser(req.Context(), admin))
		rec := httptest.NewRecorder()
		f.ctrl.oauthCallback(rec, req)
		return rec
	}
	rec := get(url.Values{"state": {"forged"}, "code": {"c"}})
	assert.Equal(t, http.StatusFound, rec.Code)
	// Without an OAuth2 helper the service reports it is not configured.
	assert.True(t, strings.HasPrefix(rec.Header().Get("Location"), "/settings/integrations?oauth_error="), rec.Header().Get("Location"))
	rec = get(url.Values{"state": {"forged"}, "error": {"access_denied"}})
	assert.Equal(t, "/settings/integrations?oauth_error=denied", rec.Header().Get("Location"))
}
