package services_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The migrator reads ./migrations relative to the working directory.
func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

const (
	goodToken = "123456:AAH-very-secret-bot-token"
	badToken  = "999999:ZZZ-refused-secret-token"
)

// ---- fakes

type node struct{ spec graph.NodeSpec }

func (n node) Spec() graph.NodeSpec { return n.spec }
func (n node) Requires() []catalog.ConnectorKind {
	return []catalog.ConnectorKind{catalog.KindTelegramBot}
}

type telegram struct {
	during     func()                    // runs inside Test, while the provider is being asked
	echo       func(token string) string // the refusal message, from the token (a careless provider)
	identityOf func(token string) string // the identity's display, from the token
	accept     string                    // another token the provider accepts
	tested     atomic.Int32
	policy     connectors.PlatformPolicy
	nodes      []catalog.NodeType
}

func (c *telegram) Kind() catalog.ConnectorKind       { return catalog.KindTelegramBot }
func (c *telegram) Nodes() []catalog.NodeType         { return c.nodes }
func (c *telegram) Triggers() []catalog.TriggerSource { return nil }
func (c *telegram) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind: catalog.KindTelegramBot, Title: map[string]string{"en": "Telegram bot"}, Auth: connectors.AuthAPIKey, Platform: c.policy,
		Category:    connectors.CategoryMessaging,
		Credentials: json.RawMessage(`{"type":"object","required":["token"],"properties":{"token":{"type":"string","x-secret":true},"chat":{"type":"string"}},"additionalProperties":false}`),
	}
}

// Test refuses badToken and, like a careless provider, echoes it in the message.
func (c *telegram) Test(_ context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	c.tested.Add(1)
	if c.during != nil {
		c.during()
	}
	if creds["token"] != goodToken && (c.accept == "" || creds["token"] != c.accept) {
		if c.echo != nil {
			return connectors.Identity{}, errors.New(c.echo(creds["token"]))
		}
		return connectors.Identity{}, fmt.Errorf("401 Unauthorized: token %s is not accepted", creds["token"])
	}
	if c.identityOf != nil {
		return connectors.Identity{Display: c.identityOf(creds["token"]), External: "bot-" + creds["token"]}, nil
	}
	return connectors.Identity{Display: "@sales_bot", External: "bot-1"}, nil
}

func telegramConnector(t *testing.T) *telegram {
	t.Helper()
	p, err := graph.FieldsParams(expr.Fields{{Name: "text", Field: expr.StringField}})
	require.NoError(t, err)
	return &telegram{nodes: []catalog.NodeType{node{spec: graph.NodeSpec{Type: "telegram.send", Title: "Send a Telegram message", Behavior: graph.BehaviorCall, Params: p, Outcomes: []string{"sent"}}}}}
}

type usage struct {
	mu   sync.Mutex
	uses map[uuid.UUID][]connectors.Use
}

func (u *usage) Uses(_ context.Context, _, id uuid.UUID) ([]connectors.Use, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.uses[id], nil
}

// fakeOAuth exchanges any code for a token of its current identity; it saves nothing.
type fakeOAuth struct {
	identity connectors.Identity
	err      error
}

func (f *fakeOAuth) Begin(_ context.Context, _ uuid.UUID, _ catalog.ConnectorKind, redirect string) (string, string, error) {
	state := uuid.NewString()
	return "https://provider.test/auth?state=" + state + "&redirect_uri=" + redirect, state, nil
}

func (f *fakeOAuth) Exchange(context.Context, uuid.UUID, string, string) (connectors.OAuth2Grant, error) {
	if f.err != nil {
		return connectors.OAuth2Grant{}, f.err
	}
	return connectors.OAuth2Grant{Kind: catalog.KindTelegramBot, Identity: f.identity, Credentials: connectors.Credentials{"token": goodToken}}, nil
}

func (f *fakeOAuth) Complete(context.Context, uuid.UUID, string, string) (connectors.Connection, error) {
	panic("the service saves the connection itself")
}

func (f *fakeOAuth) Token(context.Context, uuid.UUID, uuid.UUID) (string, error) { return "", nil }

// ---- environment

type env struct {
	t      *testing.T
	ctx    context.Context
	db     *pgxpool.Pool
	tenant uuid.UUID
	user   int64
	tg     *telegram
	reg    catalog.Registry
	store  connectors.Store
	usage  *usage
	svc    *services.IntegrationsService
	oauth  *fakeOAuth
	now    time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_integrations",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	e := &env{t: t, ctx: context.Background(), db: scope.Pool, tenant: scope.Tenant.ID, usage: &usage{uses: map[uuid.UUID][]connectors.Use{}}, now: time.Now()}
	require.NoError(t, e.db.QueryRow(e.ctx, `INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		VALUES ($1, 'user', 'Ann', 'Admin', $2, 'en') RETURNING id`, e.tenant, "admin-"+uuid.NewString()+"@example.com").Scan(&e.user))
	e.tg = telegramConnector(t)
	e.tg.policy = connectors.PlatformOptIn
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{7}, 32)})
	require.NoError(t, err)
	var store connectors.Store
	e.reg = catalog.NewRegistry(availability{get: func() connectors.Store { return store }})
	dir := connectors.NewDirectory(e.reg)
	require.NoError(t, dir.Register(e.tg))
	platform := connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		catalog.KindTelegramBot: {Credentials: connectors.Credentials{"token": "platform-secret"}},
	})
	grants := connectors.NewPlatformGrants(e.db)
	store = connectors.NewStore(e.db, keys, e.usage, platform, grants)
	e.store = store
	e.oauth = &fakeOAuth{identity: connectors.Identity{Display: "@oauth", External: "acct-1"}}
	e.svc = services.NewIntegrationsService(services.IntegrationsDeps{
		Directory: dir, Store: store, Usage: e.usage, Platform: platform, Grants: grants, OAuth: e.oauth,
		States: persistence.NewOAuthStates(e.db), Checks: persistence.NewConnectionChecks(e.db), Requests: persistence.NewRequests(e.db),
		Now: func() time.Time { return e.now },
	})
	return e
}

type availability struct{ get func() connectors.Store }

func (a availability) Connected(ctx context.Context, tenantID uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	return a.get().Connected(ctx, tenantID)
}

func (e *env) newTenant() uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	_, err := e.db.Exec(e.ctx, `INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`,
		id, "t-"+id.String()[:8], id.String()[:8]+".test")
	require.NoError(e.t, err)
	return id
}

func (e *env) actor() services.Actor { return services.Actor{TenantID: e.tenant, UserID: e.user} }

func (e *env) create(name string) services.ConnectionEntry {
	e.t.Helper()
	got, err := e.svc.Create(e.ctx, e.actor(), services.CreateInput{
		Kind: catalog.KindTelegramBot, Name: name, DisplayName: "Sales bot", Credentials: map[string]any{"token": goodToken, "chat": "-100"},
	})
	require.NoError(e.t, err)
	return got
}

func (e *env) nodes(tenant uuid.UUID) map[string]graph.NodeSpec {
	e.t.Helper()
	v, err := e.reg.ForTenant(e.ctx, tenant)
	require.NoError(e.t, err)
	return v.Catalog.Nodes
}

// noSecret fails when any value carries a credential.
func noSecret(t *testing.T, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), goodToken)
	assert.NotContains(t, string(raw), badToken)
	assert.NotContains(t, string(raw), "platform-secret")
}

// ---- tests

func TestCreateTestsFirstAndNeverReturnsTheSecret(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(e.ctx, e.actor(), services.CreateInput{Kind: catalog.KindTelegramBot, Name: "sales_bot", Credentials: map[string]any{"token": badToken}})
	var failed *services.TestFailedError
	require.ErrorAs(t, err, &failed)
	assert.Equal(t, services.ReasonAuth, failed.Reason)
	assert.Contains(t, failed.Detail, "401 Unauthorized")
	assert.NotContains(t, failed.Detail, badToken, "the connector's message is masked")
	noSecret(t, err.Error())
	conns, err := e.store.List(e.ctx, e.tenant, "")
	require.NoError(t, err)
	assert.Empty(t, conns, "nothing is saved when Test fails")

	created := e.create("sales_bot")
	assert.Equal(t, "@sales_bot", created.Connection.Identity.Display)
	assert.Equal(t, connectors.StatusConnected, created.Connection.Status)
	assert.Equal(t, map[string]string{"chat": "-100"}, created.Fields, "only non-secret fields")
	noSecret(t, created)

	list, err := e.svc.List(e.ctx, e.actor(), services.ListOptions{WithFields: true})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Len(t, list[0].Connections, 1)
	require.NotNil(t, list[0].Connections[0].LastChecked)
	noSecret(t, list)

	_, err = e.svc.Create(e.ctx, e.actor(), services.CreateInput{Kind: catalog.KindTelegramBot, Name: "Bad Name", Credentials: map[string]any{}})
	var invalid *services.InvalidInputError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, map[string]string{"name": "name", "token": "required"}, invalid.Fields)
}

func TestConcurrentCreateWithOneNameGivesNameTaken(t *testing.T) {
	e := newEnv(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = e.svc.Create(e.ctx, e.actor(), services.CreateInput{Kind: catalog.KindTelegramBot, Name: "same", Credentials: map[string]any{"token": goodToken}})
		}(i)
	}
	wg.Wait()
	ok, taken := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, connectors.ErrNameTaken):
			taken++
		default:
			t.Fatalf("unexpected error %v", err)
		}
	}
	assert.Equal(t, 1, ok)
	assert.Equal(t, 1, taken)
}

func TestUpdateKeepsCredentialsAndChecksVersion(t *testing.T) {
	e := newEnv(t)
	c := e.create("sales_bot")
	name := "Renamed"
	got, err := e.svc.Update(e.ctx, e.actor(), services.UpdateInput{ID: c.Connection.ID, Version: c.Connection.Version, DisplayName: &name})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Connection.DisplayName)
	_, creds, err := e.store.Get(e.ctx, e.tenant, c.Connection.ID)
	require.NoError(t, err)
	assert.Equal(t, goodToken, creds["token"], "credentials absent → kept")

	// An empty secret keeps the stored one; a plain field changes.
	got, err = e.svc.Update(e.ctx, e.actor(), services.UpdateInput{ID: c.Connection.ID, Version: got.Connection.Version, Credentials: map[string]any{"token": "", "chat": "-200"}})
	require.NoError(t, err)
	_, creds, err = e.store.Get(e.ctx, e.tenant, c.Connection.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.Credentials{"token": goodToken, "chat": "-200"}, creds)

	// A refused new token is not saved.
	_, err = e.svc.Update(e.ctx, e.actor(), services.UpdateInput{ID: c.Connection.ID, Version: got.Connection.Version, Credentials: map[string]any{"token": badToken}})
	var failed *services.TestFailedError
	require.ErrorAs(t, err, &failed)
	noSecret(t, failed.Detail)
	_, creds, err = e.store.Get(e.ctx, e.tenant, c.Connection.ID)
	require.NoError(t, err)
	assert.Equal(t, goodToken, creds["token"])

	_, err = e.svc.Update(e.ctx, e.actor(), services.UpdateInput{ID: c.Connection.ID, Version: 1, DisplayName: &name})
	require.ErrorIs(t, err, connectors.ErrVersionConflict)
}

func TestAnotherTenantGetsNotFound(t *testing.T) {
	e := newEnv(t)
	c := e.create("sales_bot")
	other := services.Actor{TenantID: e.newTenant(), UserID: 2}
	_, err := e.svc.Test(e.ctx, other, c.Connection.ID)
	require.ErrorIs(t, err, connectors.ErrNotFound)
	name := "x"
	_, err = e.svc.Update(e.ctx, other, services.UpdateInput{ID: c.Connection.ID, Version: c.Connection.Version, DisplayName: &name})
	require.ErrorIs(t, err, connectors.ErrNotFound)
	require.ErrorIs(t, e.svc.Delete(e.ctx, other, c.Connection.ID), connectors.ErrNotFound)
	_, err = e.svc.SetDisabled(e.ctx, other, c.Connection.ID, true)
	require.ErrorIs(t, err, connectors.ErrNotFound)
	_, err = e.svc.ConnectionUsage(e.ctx, other, c.Connection.ID)
	require.ErrorIs(t, err, connectors.ErrNotFound)
	// The connection is untouched.
	got, _, err := e.store.Get(e.ctx, e.tenant, c.Connection.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusConnected, got.Status)
}

func TestDisabledConnectionLeavesTheCatalog(t *testing.T) {
	e := newEnv(t)
	assert.NotContains(t, e.nodes(e.tenant), "telegram.send")
	c := e.create("sales_bot")
	assert.Contains(t, e.nodes(e.tenant), "telegram.send")

	got, err := e.svc.SetDisabled(e.ctx, e.actor(), c.Connection.ID, true)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusDisabled, got.Connection.Status)
	assert.NotContains(t, e.nodes(e.tenant), "telegram.send")

	// A check keeps it disabled.
	res, err := e.svc.Test(e.ctx, e.actor(), c.Connection.ID)
	require.NoError(t, err)
	assert.True(t, res.OK)
	assert.Equal(t, "@sales_bot", res.Identity)
	assert.NotContains(t, e.nodes(e.tenant), "telegram.send")

	got, err = e.svc.SetDisabled(e.ctx, e.actor(), c.Connection.ID, false)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusConnected, got.Connection.Status)
	assert.Contains(t, e.nodes(e.tenant), "telegram.send")
}

func TestTestRecordsTheStatus(t *testing.T) {
	e := newEnv(t)
	c := e.create("sales_bot")
	// The token is revoked at the provider.
	_, err := e.store.Save(e.ctx, c.Connection, connectors.Credentials{"token": badToken})
	require.NoError(t, err)
	res, err := e.svc.Test(e.ctx, e.actor(), c.Connection.ID)
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.NotContains(t, res.Detail, badToken)
	assert.Equal(t, services.ReasonAuth, res.Reason)
	got, _, err := e.store.Get(e.ctx, e.tenant, c.Connection.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusError, got.Status)
	assert.Equal(t, "auth", got.StatusReason, "the stored reason is a category, never the provider's text")
}

func TestConnectionInUseIsNotDeleted(t *testing.T) {
	e := newEnv(t)
	c := e.create("sales_bot")
	use := connectors.Use{AutomationID: uuid.New(), Name: "New order", Version: 3, NodeIDs: []string{"notify"}, Enabled: true}
	e.usage.uses[c.Connection.ID] = []connectors.Use{use, {AutomationID: use.AutomationID, Name: "New order", Version: 4}}

	list, err := e.svc.List(e.ctx, e.actor(), services.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, list[0].Connections[0].UsedBy, "versions of one automation count once")

	err = e.svc.Delete(e.ctx, e.actor(), c.Connection.ID)
	var inUse *connectors.InUseError
	require.ErrorAs(t, err, &inUse)
	assert.Equal(t, use, inUse.Uses[0])

	delete(e.usage.uses, c.Connection.ID)
	require.NoError(t, e.svc.Delete(e.ctx, e.actor(), c.Connection.ID))
	require.ErrorIs(t, e.svc.Delete(e.ctx, e.actor(), c.Connection.ID), connectors.ErrNotFound)
}

func TestPlatformKeyIsOptIn(t *testing.T) {
	e := newEnv(t)
	list, err := e.svc.List(e.ctx, e.actor(), services.ListOptions{})
	require.NoError(t, err)
	require.NotNil(t, list[0].Platform)
	assert.Equal(t, services.PlatformState{Policy: connectors.PlatformOptIn, Available: true}, *list[0].Platform)
	assert.NotContains(t, e.nodes(e.tenant), "telegram.send")

	require.NoError(t, e.svc.SetPlatformEnabled(e.ctx, e.actor(), catalog.KindTelegramBot, true))
	list, err = e.svc.List(e.ctx, e.actor(), services.ListOptions{})
	require.NoError(t, err)
	assert.True(t, list[0].Platform.EnabledForTenant)
	assert.Contains(t, e.nodes(e.tenant), "telegram.send")
	noSecret(t, list)

	require.ErrorIs(t, e.svc.SetPlatformEnabled(e.ctx, e.actor(), "unknown", true), connectors.ErrNotFound)
}

func TestOAuthStateIsOneTimeAndBound(t *testing.T) {
	e := newEnv(t)
	// The fake connector authenticates with an API key: OAuth needs an oauth2 connector.
	_, err := e.svc.OAuthStart(e.ctx, e.actor(), catalog.KindTelegramBot, "", "", uuid.Nil, "https://app.test/cb")
	var invalid *services.InvalidInputError
	require.ErrorAs(t, err, &invalid)

	// The state rules are the repository's and the service's: exercise them through the repository.
	states := persistence.NewOAuthStates(e.db)
	put := func(a services.Actor, state string, expires time.Time) {
		require.NoError(t, states.Put(e.ctx, services.OAuthState{Hash: services.StateHash(state), TenantID: a.TenantID, UserID: a.UserID, Kind: catalog.KindTelegramBot, Name: "oauth_bot", DisplayName: "Sales", ExpiresAt: expires}))
	}
	me := e.actor()
	put(me, "s1", e.now.Add(10*time.Minute))
	_, _, err = e.svc.OAuthComplete(e.ctx, services.Actor{TenantID: me.TenantID, UserID: 99}, "s1", "code")
	require.ErrorIs(t, err, services.ErrStateInvalid, "another user")
	_, _, err = e.svc.OAuthComplete(e.ctx, services.Actor{TenantID: e.newTenant(), UserID: me.UserID}, "s1", "code")
	require.ErrorIs(t, err, services.ErrStateInvalid, "another tenant")

	conn, _, err := e.svc.OAuthComplete(e.ctx, me, "s1", "code")
	require.NoError(t, err)
	assert.Equal(t, "oauth_bot", conn.Name, "the name given at the start")
	assert.Equal(t, "Sales", conn.DisplayName)
	_, _, err = e.svc.OAuthComplete(e.ctx, me, "s1", "code")
	require.ErrorIs(t, err, services.ErrStateInvalid, "once")

	put(me, "s2", e.now.Add(-time.Second))
	_, _, err = e.svc.OAuthComplete(e.ctx, me, "s2", "code")
	require.ErrorIs(t, err, services.ErrStateInvalid, "expired")

	put(me, "s3", e.now.Add(time.Minute))
	abandoned, err := e.svc.OAuthAbandon(e.ctx, me, "s3")
	require.NoError(t, err)
	assert.Equal(t, catalog.KindTelegramBot, abandoned.Kind)
	assert.Equal(t, "oauth_bot", abandoned.Name, "the form context comes back")
	_, _, err = e.svc.OAuthComplete(e.ctx, me, "s3", "code")
	require.ErrorIs(t, err, services.ErrStateInvalid, "an abandoned state is gone")
}

func (e *env) putState(state string, connectionID uuid.UUID) {
	e.t.Helper()
	require.NoError(e.t, persistence.NewOAuthStates(e.db).Put(e.ctx, services.OAuthState{
		Hash: services.StateHash(state), TenantID: e.tenant, UserID: e.user, Kind: catalog.KindTelegramBot, ConnectionID: connectionID, ExpiresAt: e.now.Add(time.Minute),
	}))
}

func (e *env) putNamed(state, name string) {
	e.t.Helper()
	require.NoError(e.t, persistence.NewOAuthStates(e.db).Put(e.ctx, services.OAuthState{
		Hash: services.StateHash(state), TenantID: e.tenant, UserID: e.user, Kind: catalog.KindTelegramBot, Name: name, ExpiresAt: e.now.Add(time.Minute),
	}))
}

func TestOAuthReauthSavesOnlyTheChosenConnection(t *testing.T) {
	e := newEnv(t)
	e.putNamed("a", "account_a")
	a, _, err := e.svc.OAuthComplete(e.ctx, e.actor(), "a", "code")
	require.NoError(t, err)
	e.oauth.identity = connectors.Identity{Display: "@b", External: "acct-2"}
	e.putNamed("b", "account_b")
	b, _, err := e.svc.OAuthComplete(e.ctx, e.actor(), "b", "code")
	require.NoError(t, err)
	_, err = e.svc.SetDisabled(e.ctx, e.actor(), b.ID, true)
	require.NoError(t, err)
	require.NoError(t, e.store.SetStatus(e.ctx, e.tenant, a.ID, connectors.StatusNeedsReauth, "reauth"))
	bBefore, _, err := e.store.Get(e.ctx, e.tenant, b.ID)
	require.NoError(t, err)

	// Re-authorizing A, the administrator signs in as account B.
	e.putState("again", a.ID)
	got, pending, err := e.svc.OAuthComplete(e.ctx, e.actor(), "again", "code")
	require.NoError(t, err)
	assert.Equal(t, a.ID, got.ID)
	assert.Equal(t, a.ID, pending.ConnectionID)
	assert.Equal(t, connectors.StatusConnected, got.Status)
	assert.Equal(t, "@b", got.Identity.Display)
	bAfter, _, err := e.store.Get(e.ctx, e.tenant, b.ID)
	require.NoError(t, err)
	assert.Equal(t, bBefore, bAfter, "B is not touched: still disabled, same version")
	conns, err := e.store.List(e.ctx, e.tenant, "")
	require.NoError(t, err)
	assert.Len(t, conns, 2)
}

func TestOAuthReauthOfADisabledConnectionKeepsItDisabled(t *testing.T) {
	e := newEnv(t)
	e.putNamed("a", "account_a")
	a, _, err := e.svc.OAuthComplete(e.ctx, e.actor(), "a", "code")
	require.NoError(t, err)
	_, err = e.svc.SetDisabled(e.ctx, e.actor(), a.ID, true)
	require.NoError(t, err)
	e.putState("again", a.ID)
	got, _, err := e.svc.OAuthComplete(e.ctx, e.actor(), "again", "code")
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusDisabled, got.Status)
	assert.NotContains(t, e.nodes(e.tenant), "telegram.send")
}

func TestOAuthCreateWithATakenNameLeavesNothing(t *testing.T) {
	e := newEnv(t)
	e.create("sales_bot")
	e.putNamed("dup", "sales_bot")
	_, pending, err := e.svc.OAuthComplete(e.ctx, e.actor(), "dup", "code")
	require.ErrorIs(t, err, connectors.ErrNameTaken)
	assert.Equal(t, "sales_bot", pending.Name, "the form context comes back with the conflict")
	conns, err := e.store.List(e.ctx, e.tenant, "")
	require.NoError(t, err)
	assert.Len(t, conns, 1)
}

func TestOAuthProviderFailureKeepsTheFormContext(t *testing.T) {
	e := newEnv(t)
	e.oauth.err = connectors.ErrOAuth2Refused
	e.putNamed("x", "my_bot")
	_, pending, err := e.svc.OAuthComplete(e.ctx, e.actor(), "x", "code")
	require.ErrorIs(t, err, services.ErrOAuthRefused)
	assert.Equal(t, "my_bot", pending.Name)
	e.oauth.err = errors.New("oauth2: cannot fetch token: 500")
	e.putNamed("y", "my_bot")
	_, pending, err = e.svc.OAuthComplete(e.ctx, e.actor(), "y", "code")
	require.ErrorIs(t, err, services.ErrOAuthExchange)
	assert.Equal(t, catalog.KindTelegramBot, pending.Kind)
}

func TestRefusalIsCategorizedMaskedAndBounded(t *testing.T) {
	e := newEnv(t)
	c := e.create("sales_bot")
	long := strings.Repeat("x", 40) + badToken
	_, err := e.store.Save(e.ctx, c.Connection, connectors.Credentials{"token": long})
	require.NoError(t, err)
	e.tg.echo = func(token string) string {
		return "HTTP 401 for Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(token)) + " " + url.QueryEscape(token) + strings.Repeat(" <html>", 2000)
	}
	res, err := e.svc.Test(e.ctx, e.actor(), c.Connection.ID)
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Equal(t, services.ReasonAuth, res.Reason)
	assert.NotContains(t, res.Detail, base64.StdEncoding.EncodeToString([]byte(long)))
	assert.NotContains(t, res.Detail, url.QueryEscape(long))
	assert.LessOrEqual(t, len([]rune(res.Detail)), 500)
	got, _, err := e.store.Get(e.ctx, e.tenant, c.Connection.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusError, got.Status)
	assert.Equal(t, "auth", got.StatusReason)
}

// A short secret (a three-letter password) cannot be masked inside text: wherever it occurs — as a
// word or inside an identifier or a dump — the provider's text is dropped whole, from the refusal and
// from the identity.
func TestShortSecretEchoedByTheConnectorIsNeverReturned(t *testing.T) {
	e := newEnv(t)
	for _, echo := range []func(string) string{
		func(tok string) string { return "login failed for password " + tok + " (401)" },
		func(tok string) string { return `{"error":"bad_creds","user":"svc_` + tok + `_01"}` },
		func(tok string) string { return "HTTP401:" + strings.ToUpper(tok) + "x" },
	} {
		e.tg.echo = echo
		_, err := e.svc.Create(e.ctx, e.actor(), services.CreateInput{Kind: catalog.KindTelegramBot, Name: "short", Credentials: map[string]any{"token": "pw1"}})
		var failed *services.TestFailedError
		require.ErrorAs(t, err, &failed)
		assert.Empty(t, failed.Detail)
		assert.NotContains(t, strings.ToLower(err.Error()), "pw1")
	}

	e.tg.accept = "pw1"
	for i, identityOf := range []func(string) string{
		func(tok string) string { return "bot " + tok },
		func(tok string) string { return "@acme" + tok + "bot" },
	} {
		e.tg.identityOf = identityOf
		got, err := e.svc.Create(e.ctx, e.actor(), services.CreateInput{Kind: catalog.KindTelegramBot, Name: fmt.Sprintf("short_%d", i), Credentials: map[string]any{"token": "pw1"}})
		require.NoError(t, err)
		assert.Empty(t, got.Connection.Identity.Display)
		assert.NotContains(t, got.Connection.Identity.External, "pw1")
		list, err := e.svc.List(e.ctx, e.actor(), services.ListOptions{WithFields: true})
		require.NoError(t, err)
		assert.NotContains(t, fmt.Sprintf("%+v", list), "pw1")
	}
}

// The identity Connector.Test derives from a new OAuth2 token is stored and listed: it is sanitized
// against the token set before the connection is created or re-authorized.
func TestOAuthIdentityEchoingTheTokenIsSanitized(t *testing.T) {
	e := newEnv(t)
	e.oauth.identity = connectors.Identity{Display: "token " + goodToken, External: "id-" + goodToken}
	e.putNamed("a", "")
	created, _, err := e.svc.OAuthComplete(e.ctx, e.actor(), "a", "code")
	require.NoError(t, err)
	assert.NotContains(t, created.Identity.Display, goodToken)
	assert.NotContains(t, created.Identity.External, goodToken)
	assert.NotContains(t, created.DisplayName, goodToken, "the display name falls back to the sanitized identity")

	e.putState("b", created.ID)
	reauth, _, err := e.svc.OAuthComplete(e.ctx, e.actor(), "b", "code")
	require.NoError(t, err)
	assert.NotContains(t, fmt.Sprintf("%+v", reauth.Identity), goodToken)
	list, err := e.svc.List(e.ctx, e.actor(), services.ListOptions{WithFields: true})
	require.NoError(t, err)
	noSecret(t, list)
}

func TestRetryWithTheSameKeyReturnsTheFirstResult(t *testing.T) {
	e := newEnv(t)
	in := services.CreateInput{Kind: catalog.KindTelegramBot, Name: "sales_bot", Credentials: map[string]any{"token": goodToken}, IdempotencyKey: "k-1"}
	first, err := e.svc.Create(e.ctx, e.actor(), in)
	require.NoError(t, err)
	tested := e.tg.tested.Load()
	// The answer was lost; «Retry» sends the same key.
	again, err := e.svc.Create(e.ctx, e.actor(), in)
	require.NoError(t, err)
	assert.Equal(t, first.Connection.ID, again.Connection.ID)
	assert.Equal(t, tested, e.tg.tested.Load(), "the provider is not asked again")

	// The key is bound to its payload.
	other := in
	other.Name = "another"
	_, err = e.svc.Create(e.ctx, e.actor(), other)
	var invalid *services.InvalidInputError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "invalid", invalid.Fields["idempotency_key"])

	name := "Renamed"
	upd := services.UpdateInput{ID: first.Connection.ID, Version: first.Connection.Version, DisplayName: &name, IdempotencyKey: "k-2"}
	u1, err := e.svc.Update(e.ctx, e.actor(), upd)
	require.NoError(t, err)
	u2, err := e.svc.Update(e.ctx, e.actor(), upd)
	require.NoError(t, err, "a retried update is not a version conflict")
	assert.Equal(t, u1.Connection.Version, u2.Connection.Version)

	// Another tenant's key space is its own.
	stranger := services.Actor{TenantID: e.newTenant(), UserID: e.user}
	theirs, err := e.svc.Create(e.ctx, stranger, in)
	require.NoError(t, err)
	assert.NotEqual(t, first.Connection.ID, theirs.Connection.ID)
}

// A retry that overlaps its first request (the provider is still answering) does not ask the provider
// again: it is told the request is in progress, and once the first finished it gets its connection.
func TestOverlappingRetryWaitsForTheFirstRequest(t *testing.T) {
	e := newEnv(t)
	in := services.CreateInput{Kind: catalog.KindTelegramBot, Name: "sales_bot", Credentials: map[string]any{"token": goodToken}, IdempotencyKey: "k-overlap"}
	inside, release := make(chan struct{}), make(chan struct{})
	e.tg.during = func() { e.tg.during = nil; close(inside); <-release }
	done := make(chan error, 1)
	var first services.ConnectionEntry
	go func() {
		var err error
		first, err = e.svc.Create(e.ctx, e.actor(), in)
		done <- err
	}()
	<-inside
	_, err := e.svc.Create(e.ctx, e.actor(), in)
	require.ErrorIs(t, err, services.ErrRequestInProgress)
	close(release)
	require.NoError(t, <-done)
	again, err := e.svc.Create(e.ctx, e.actor(), in)
	require.NoError(t, err)
	assert.Equal(t, first.Connection.ID, again.Connection.ID)
	assert.EqualValues(t, 1, e.tg.tested.Load())
}

// A request cancelled before its save commits leaves no connection and frees its key; the retry runs.
func TestCancelledRequestFreesItsKey(t *testing.T) {
	e := newEnv(t)
	in := services.CreateInput{Kind: catalog.KindTelegramBot, Name: "sales_bot", Credentials: map[string]any{"token": goodToken}, IdempotencyKey: "k-cancel"}
	ctx, cancel := context.WithCancel(e.ctx)
	e.tg.during = func() { e.tg.during = nil; cancel() }
	_, err := e.svc.Create(ctx, e.actor(), in)
	require.Error(t, err)
	conns, err := e.store.List(e.ctx, e.tenant, "")
	require.NoError(t, err)
	assert.Empty(t, conns)
	got, err := e.svc.Create(e.ctx, e.actor(), in)
	require.NoError(t, err)
	assert.Equal(t, "sales_bot", got.Connection.Name)
}

// The key is recorded in the transaction of the save: when recording fails, the connection is not
// created either, so a key never misses a change that committed.
func TestKeyAndSaveCommitTogether(t *testing.T) {
	e := newEnv(t)
	failing := failingRequests{Requests: persistence.NewRequests(e.db)}
	svc := services.NewIntegrationsService(services.IntegrationsDeps{Directory: e.svc.Directory(), Store: e.store, Requests: failing})
	_, err := svc.Create(e.ctx, e.actor(), services.CreateInput{Kind: catalog.KindTelegramBot, Name: "sales_bot", Credentials: map[string]any{"token": goodToken}, IdempotencyKey: "k-tx"})
	require.Error(t, err)
	conns, err := e.store.List(e.ctx, e.tenant, "")
	require.NoError(t, err)
	assert.Empty(t, conns)
}

type failingRequests struct{ services.Requests }

func (failingRequests) Complete(context.Context, pgx.Tx, uuid.UUID, string, uuid.UUID) error {
	return errors.New("record failed")
}

// A disable committed while the provider is being asked wins: the check's outcome is written only to
// the version it read.
func TestCheckDoesNotOverwriteAConcurrentDisable(t *testing.T) {
	e := newEnv(t)
	c := e.create("sales_bot")
	e.tg.during = func() {
		e.tg.during = nil
		require.NoError(t, e.store.SetStatus(e.ctx, e.tenant, c.Connection.ID, connectors.StatusDisabled, ""))
	}
	_, err := e.svc.Test(e.ctx, e.actor(), c.Connection.ID)
	require.ErrorIs(t, err, connectors.ErrVersionConflict)
	got, _, err := e.store.Get(e.ctx, e.tenant, c.Connection.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusDisabled, got.Status)
}
