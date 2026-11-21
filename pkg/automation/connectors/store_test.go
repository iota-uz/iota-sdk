package connectors_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

const botToken = "123456:AAH-very-secret-bot-token"

type env struct {
	t      *testing.T
	ctx    context.Context
	db     *pgxpool.Pool
	tenant uuid.UUID
}

func newEnv(t *testing.T) *env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_connectors",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	return &env{t: t, ctx: context.Background(), db: scope.Pool, tenant: scope.Tenant.ID}
}

func (e *env) newTenant() uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	_, err := e.db.Exec(e.ctx, `INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`,
		id, "t-"+id.String()[:8], id.String()[:8]+".test")
	require.NoError(e.t, err)
	return id
}

func key(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func keyring(t *testing.T, active string, ids ...string) connectors.Keyring {
	t.Helper()
	keys := map[string][]byte{}
	for _, id := range ids {
		keys[id] = key(id[len(id)-1])
	}
	k, err := connectors.NewKeyring(active, keys)
	require.NoError(t, err)
	return k
}

// ---- a fake connector

type telegram struct {
	tested  atomic.Int32
	nodes   []catalog.NodeType
	sources []catalog.TriggerSource
	policy  connectors.PlatformPolicy
	kind    catalog.ConnectorKind
	auth    connectors.AuthKind
	oauth   *connectors.OAuth2Config
}

func (c *telegram) Kind() catalog.ConnectorKind {
	if c.kind != "" {
		return c.kind
	}
	return catalog.KindTelegramBot
}
func (c *telegram) Nodes() []catalog.NodeType         { return c.nodes }
func (c *telegram) Triggers() []catalog.TriggerSource { return c.sources }
func (c *telegram) Descriptor() connectors.Descriptor {
	auth := c.auth
	if auth == "" {
		auth = connectors.AuthAPIKey
	}
	return connectors.Descriptor{
		Kind: c.Kind(), Title: map[string]string{"en": "Telegram bot"}, Auth: auth, OAuth2: c.oauth, Platform: c.policy,
		Credentials: json.RawMessage(`{"type":"object","properties":{"token":{"type":"string","x-secret":true},"username":{"type":"string"},"access_token":{"type":"string","x-secret":true}},"additionalProperties":false}`),
	}
}
func (c *telegram) Test(_ context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	c.tested.Add(1)
	return connectors.Identity{Display: "Support bot", External: "bot-1"}, nil
}

type node struct{ spec graph.NodeSpec }

func (n node) Spec() graph.NodeSpec              { return n.spec }
func (n node) Requires() []catalog.ConnectorKind { return nil }

type source struct{ events []graph.EventSpec }

func (s source) Events() []graph.EventSpec         { return s.events }
func (s source) Requires() []catalog.ConnectorKind { return nil }

func telegramConnector(t *testing.T) *telegram {
	p, err := graph.FieldsParams(expr.Fields{{Name: "text", Field: expr.StringField}})
	require.NoError(t, err)
	return &telegram{
		nodes: []catalog.NodeType{node{spec: graph.NodeSpec{Type: "telegram.send", Behavior: graph.BehaviorCall, Params: p, Outcomes: []string{"sent"}}}},
		sources: []catalog.TriggerSource{source{events: []graph.EventSpec{{Key: "telegram.message", Subject: graph.SubjectConversation,
			Payload: expr.Fields{{Name: "text", Field: expr.StringField}}}}}},
	}
}

func (e *env) save(s connectors.ConnectionStore, tenant uuid.UUID, name string, creds connectors.Credentials) connectors.Connection {
	e.t.Helper()
	c, err := s.Save(e.ctx, connectors.Connection{TenantID: tenant, Kind: catalog.KindTelegramBot, Name: name, DisplayName: "Support"}, creds)
	require.NoError(e.t, err)
	return c
}

// ---- acceptance

func TestTelegramAppearsAfterSave(t *testing.T) {
	e := newEnv(t)
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	reg := catalog.NewRegistry(store)
	dir := connectors.NewDirectory(reg)
	require.NoError(t, dir.Register(telegramConnector(t)))
	assert.Error(t, dir.Register(telegramConnector(t)), "a kind twice")

	v, err := reg.ForTenant(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.NotContains(t, v.Catalog.Nodes, "telegram.send")
	assert.NotContains(t, v.Catalog.Events, "telegram.message")
	assert.Equal(t, catalog.ReasonConnectionUnavailable, v.Unavailable["telegram.send"])
	assert.Equal(t, catalog.ReasonConnectionUnavailable, v.Unavailable["telegram.message"])

	c := e.save(store, e.tenant, "support_bot", connectors.Credentials{"token": botToken})
	v, err = reg.ForTenant(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Contains(t, v.Catalog.Nodes, "telegram.send")
	assert.Contains(t, v.Catalog.Events, "telegram.message")
	assert.Equal(t, []catalog.ConnectionRef{{ID: c.ID.String(), Name: "support_bot"}}, v.Connections[catalog.KindTelegramBot])

	// Another tenant still does not see it; a disabled connection does not count.
	other, err := reg.ForTenant(e.ctx, e.newTenant())
	require.NoError(t, err)
	assert.NotContains(t, other.Catalog.Nodes, "telegram.send")
	require.NoError(t, store.SetStatus(e.ctx, e.tenant, c.ID, connectors.StatusDisabled, "paused"))
	v, err = reg.ForTenant(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.NotContains(t, v.Catalog.Nodes, "telegram.send")
}

func TestOptInPlatformCredential(t *testing.T) {
	e := newEnv(t)
	tg := telegramConnector(t)
	tg.policy = connectors.PlatformOptIn
	dir := connectors.NewDirectory(nil)
	require.NoError(t, dir.Register(tg))
	platform := connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		catalog.KindTelegramBot: {Credentials: connectors.Credentials{"token": "platform-secret"}, Identity: connectors.Identity{Display: "IOTA bot"}},
	})
	grants := connectors.NewPlatformGrants(e.db)
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, platform, grants)

	got, err := store.Connected(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Empty(t, got[catalog.KindTelegramBot], "opt_in before SetEnabled")
	_, _, err = store.Open(e.ctx, e.tenant, "platform:telegram_bot")
	assert.ErrorIs(t, err, connectors.ErrNotFound)

	require.NoError(t, grants.SetEnabled(e.ctx, e.tenant, catalog.KindTelegramBot, true, 0))
	got, err = store.Connected(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Equal(t, []catalog.ConnectionRef{{ID: "platform:telegram_bot", Name: "platform"}}, got[catalog.KindTelegramBot])
	info, creds, err := store.Open(e.ctx, e.tenant, "platform:telegram_bot")
	require.NoError(t, err)
	assert.True(t, info.Platform)
	assert.Equal(t, "platform-secret", creds["token"])

	// The tenant's own connection comes first.
	c := e.save(store, e.tenant, "own_bot", connectors.Credentials{"token": botToken})
	got, err = store.Connected(e.ctx, e.tenant)
	require.NoError(t, err)
	require.Len(t, got[catalog.KindTelegramBot], 2)
	assert.Equal(t, c.ID.String(), got[catalog.KindTelegramBot][0].ID)
	assert.Equal(t, "platform:telegram_bot", got[catalog.KindTelegramBot][1].ID)

	// Another tenant has not enabled it.
	got, err = store.Connected(e.ctx, e.newTenant())
	require.NoError(t, err)
	assert.Empty(t, got[catalog.KindTelegramBot])

	require.NoError(t, grants.SetEnabled(e.ctx, e.tenant, catalog.KindTelegramBot, false, 0))
	got, err = store.Connected(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Equal(t, []catalog.ConnectionRef{{ID: c.ID.String(), Name: "own_bot"}}, got[catalog.KindTelegramBot])

	// auto: every tenant.
	tg.policy = connectors.PlatformAuto
	got, err = store.Connected(e.ctx, e.newTenant())
	require.NoError(t, err)
	assert.Len(t, got[catalog.KindTelegramBot], 1)
}

func TestMovedRowDoesNotDecrypt(t *testing.T) {
	e := newEnv(t)
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	c := e.save(store, e.tenant, "support_bot", connectors.Credentials{"token": botToken})
	_, creds, err := store.Get(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, botToken, creds["token"])

	other := e.newTenant()
	_, err = e.db.Exec(e.ctx, `UPDATE automation.connections SET tenant_id = $1 WHERE id = $2`, other, c.ID)
	require.NoError(t, err)
	_, _, err = store.Get(e.ctx, other, c.ID)
	assert.ErrorIs(t, err, connectors.ErrUndecryptable)
	_, _, err = store.Get(e.ctx, e.tenant, c.ID)
	assert.ErrorIs(t, err, connectors.ErrNotFound, "the old tenant no longer has it")

	_, err = e.db.Exec(e.ctx, `UPDATE automation.connections SET tenant_id = $1, kind = 'smtp' WHERE id = $2`, e.tenant, c.ID)
	require.NoError(t, err)
	_, _, err = store.Get(e.ctx, e.tenant, c.ID)
	assert.ErrorIs(t, err, connectors.ErrUndecryptable, "another kind")
}

func TestRewrapMovesEveryRowToTheActiveKey(t *testing.T) {
	e := newEnv(t)
	old := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	tenants := []uuid.UUID{e.tenant, e.newTenant()}
	var ids []uuid.UUID
	for i := 0; i < 7; i++ {
		c := e.save(old, tenants[i%2], fmt.Sprintf("bot_%d", i), connectors.Credentials{"token": fmt.Sprintf("%s-%d", botToken, i)})
		ids = append(ids, c.ID)
	}

	rotated := connectors.NewStore(e.db, keyring(t, "k2", "k1", "k2"), nil, nil, nil)
	total := 0
	for {
		n, err := rotated.Rewrap(e.ctx, 3)
		require.NoError(t, err)
		if n == 0 {
			break
		}
		assert.LessOrEqual(t, n, 3)
		total += n
	}
	assert.Equal(t, 7, total)
	var left int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.connections WHERE key_id <> 'k2'`).Scan(&left))
	assert.Zero(t, left)
	n, err := rotated.Rewrap(e.ctx, 3)
	require.NoError(t, err)
	assert.Zero(t, n, "idempotent")

	// The old key can go.
	onlyNew := connectors.NewStore(e.db, keyring(t, "k2", "k2"), nil, nil, nil)
	for i, id := range ids {
		c, creds, err := onlyNew.Get(e.ctx, tenants[i%2], id)
		require.NoError(t, err)
		assert.Equal(t, "k2", c.KeyID)
		assert.Equal(t, fmt.Sprintf("%s-%d", botToken, i), creds["token"])
	}

	// A row whose key is gone is reported, not lost and not returned.
	_, err = e.db.Exec(e.ctx, `UPDATE automation.connections SET key_id = 'gone' WHERE id = $1`, ids[0])
	require.NoError(t, err)
	_, err = onlyNew.Rewrap(e.ctx, 10)
	assert.ErrorIs(t, err, connectors.ErrUndecryptable)
	_, _, err = onlyNew.Get(e.ctx, tenants[0], ids[0])
	assert.ErrorIs(t, err, connectors.ErrNotConfigured)
	got, err := onlyNew.Connected(e.ctx, tenants[0])
	require.NoError(t, err)
	for _, r := range got[catalog.KindTelegramBot] {
		assert.NotEqual(t, ids[0].String(), r.ID)
	}
}

func TestSecretNeverLeaves(t *testing.T) {
	e := newEnv(t)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	c := e.save(store, e.tenant, "support_bot", connectors.Credentials{"token": botToken})

	list, err := store.List(e.ctx, e.tenant, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	raw, err := json.Marshal(list)
	require.NoError(t, err)
	logger.Info("list", "connections", list, "json", string(raw), "printed", fmt.Sprintf("%+v %#v", list, list))

	_, creds, err := store.Get(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	logger.Info("creds", "creds", creds, "printed", fmt.Sprintf("%v %+v %#v", creds, creds, creds))

	// Errors on every failing path.
	var errs []error
	_, err = store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "support_bot"}, connectors.Credentials{"token": botToken})
	errs = append(errs, err)
	_, err = store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "Bad Name"}, connectors.Credentials{"token": botToken})
	errs = append(errs, err)
	stale := c
	stale.Version = 99
	_, err = store.Save(e.ctx, stale, connectors.Credentials{"token": botToken})
	errs = append(errs, err)
	_, err = e.db.Exec(e.ctx, `UPDATE automation.connections SET kind = 'smtp' WHERE id = $1`, c.ID)
	require.NoError(t, err)
	_, _, err = store.Get(e.ctx, e.tenant, c.ID)
	errs = append(errs, err)
	err = connectors.ValidateCredentials(telegramConnector(t).Descriptor(), connectors.Credentials{"token": botToken, "extra": botToken})
	errs = append(errs, err)
	for _, err := range errs {
		require.Error(t, err)
		logger.Error("failed", "err", err)
		assert.NotContains(t, err.Error(), botToken)
		assert.NotContains(t, fmt.Sprintf("%+v", err), botToken)
	}
	assert.NotContains(t, string(raw), botToken)
	assert.NotContains(t, logs.String(), botToken)
	if os.Getenv("SHOW_LOGS") != "" {
		t.Log(logs.String())
	}

	var secret []byte
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT secret FROM automation.connections WHERE id = $1`, c.ID).Scan(&secret))
	assert.False(t, bytes.Contains(secret, []byte(botToken)), "stored in the clear")
}

func TestStoreRules(t *testing.T) {
	e := newEnv(t)
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	c := e.save(store, e.tenant, "support_bot", connectors.Credentials{"token": botToken})
	assert.Equal(t, 1, c.Version)
	assert.Equal(t, connectors.StatusConnected, c.Status)
	assert.Equal(t, "k1", c.KeyID)

	// Name taken in (tenant, kind); free in another kind or tenant.
	_, err := store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "support_bot"}, connectors.Credentials{})
	assert.ErrorIs(t, err, connectors.ErrNameTaken)
	_, err = store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindSMTP, Name: "support_bot"}, connectors.Credentials{})
	assert.NoError(t, err)
	other := e.newTenant()
	_, err = store.Save(e.ctx, connectors.Connection{TenantID: other, Kind: catalog.KindTelegramBot, Name: "support_bot"}, connectors.Credentials{})
	assert.NoError(t, err)
	_, err = store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "new_bot"}, nil)
	assert.ErrorIs(t, err, connectors.ErrInvalid, "a new connection needs credentials")

	// Update: creds nil keeps them; a stale version conflicts.
	c.DisplayName = "Renamed"
	updated, err := store.Save(e.ctx, c, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)
	_, creds, err := store.Get(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, botToken, creds["token"])
	_, err = store.Save(e.ctx, c, nil)
	assert.ErrorIs(t, err, connectors.ErrVersionConflict)
	updated, err = store.Save(e.ctx, updated, connectors.Credentials{"token": "rotated"})
	require.NoError(t, err)
	_, creds, err = store.Get(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "rotated", creds["token"])

	// Isolation: another tenant gets not found everywhere.
	_, _, err = store.Get(e.ctx, other, c.ID)
	assert.ErrorIs(t, err, connectors.ErrNotFound)
	foreign := updated
	foreign.TenantID = other
	_, err = store.Save(e.ctx, foreign, nil)
	assert.ErrorIs(t, err, connectors.ErrNotFound)
	assert.ErrorIs(t, store.SetStatus(e.ctx, other, c.ID, connectors.StatusError, "x"), connectors.ErrNotFound)
	assert.ErrorIs(t, store.Delete(e.ctx, other, c.ID), connectors.ErrNotFound)
	assert.ErrorIs(t, store.UpdateInbound(e.ctx, other, c.ID, func(s json.RawMessage) (json.RawMessage, error) { return s, nil }), connectors.ErrNotFound)
	list, err := store.List(e.ctx, other, catalog.KindTelegramBot)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.NotEqual(t, c.ID, list[0].ID)

	// Inbound state under the row lock.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, store.UpdateInbound(e.ctx, e.tenant, c.ID, func(s json.RawMessage) (json.RawMessage, error) {
				var st struct{ Cursor int }
				_ = json.Unmarshal(s, &st)
				st.Cursor++
				return json.Marshal(st)
			}))
		}()
	}
	wg.Wait()
	var cursor int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT (inbound->>'Cursor')::int FROM automation.connections WHERE id = $1`, c.ID).Scan(&cursor))
	assert.Equal(t, 8, cursor)

	// Open: a connected connection; needs_reauth and error are refused.
	info, creds, err := store.Open(e.ctx, e.tenant, c.ID.String())
	require.NoError(t, err)
	assert.Equal(t, catalog.ConnectionInfo{ID: c.ID.String(), Kind: catalog.KindTelegramBot, Name: "support_bot"}, info)
	assert.Equal(t, "rotated", creds["token"])
	require.NoError(t, store.SetStatus(e.ctx, e.tenant, c.ID, connectors.StatusNeedsReauth, "expired"))
	_, _, err = store.Open(e.ctx, e.tenant, c.ID.String())
	assert.ErrorIs(t, err, connectors.ErrNeedsReauth)
	_, _, err = store.Open(e.ctx, e.tenant, "not-a-uuid")
	assert.ErrorIs(t, err, connectors.ErrNotFound)

	// Delete.
	require.NoError(t, store.Delete(e.ctx, e.tenant, c.ID))
	_, _, err = store.Get(e.ctx, e.tenant, c.ID)
	assert.ErrorIs(t, err, connectors.ErrNotFound)
}

type fakeUses []connectors.Use

func (f fakeUses) Uses(context.Context, uuid.UUID, uuid.UUID) ([]connectors.Use, error) {
	return f, nil
}

func TestDeleteRefusesAConnectionInUse(t *testing.T) {
	e := newEnv(t)
	uses := fakeUses{{AutomationID: uuid.New(), Name: "Reply", Version: 3, NodeIDs: []string{"send"}, Enabled: true}}
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), uses, nil, nil)
	c := e.save(store, e.tenant, "support_bot", connectors.Credentials{"token": botToken})
	err := store.Delete(e.ctx, e.tenant, c.ID)
	require.ErrorIs(t, err, connectors.ErrConnectionInUse)
	var inUse *connectors.InUseError
	require.ErrorAs(t, err, &inUse)
	assert.Equal(t, []connectors.Use(uses), inUse.Uses)
	assert.Contains(t, err.Error(), "Reply v3")
}

func TestWithoutAKeyOnlyPlatformWorks(t *testing.T) {
	e := newEnv(t)
	tg := telegramConnector(t)
	tg.policy = connectors.PlatformAuto
	reg := catalog.NewRegistry(nil)
	dir := connectors.NewDirectory(reg)
	require.NoError(t, dir.Register(tg))
	platform := connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		catalog.KindTelegramBot: {Credentials: connectors.Credentials{"token": "platform-secret"}},
	})
	store := connectors.NewStore(e.db, nil, nil, platform, nil)
	_, err := store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot"}, connectors.Credentials{"token": botToken})
	assert.ErrorIs(t, err, connectors.ErrNotConfigured)
	_, err = store.List(e.ctx, e.tenant, "")
	assert.ErrorIs(t, err, connectors.ErrNotConfigured)
	_, err = store.Rewrap(e.ctx, 10)
	assert.ErrorIs(t, err, connectors.ErrNotConfigured)
	got, err := store.Connected(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Equal(t, []catalog.ConnectionRef{{ID: "platform:telegram_bot", Name: "platform"}}, got[catalog.KindTelegramBot])
	_, creds, err := store.Open(e.ctx, e.tenant, "platform:telegram_bot")
	require.NoError(t, err)
	assert.Equal(t, "platform-secret", creds["token"])
}

func TestUsageIsRecordedApart(t *testing.T) {
	e := newEnv(t)
	rec := connectors.NewUsageRecorder(e.db)
	run := uuid.New()
	require.NoError(t, rec.Record(e.ctx, catalog.UsageEntry{TenantID: e.tenant, Kind: catalog.KindOpenAI, ConnectionID: "platform:openai", Unit: "token_in", Quantity: 120, RunID: &run}))
	require.NoError(t, rec.Record(e.ctx, catalog.UsageEntry{TenantID: e.tenant, Kind: catalog.KindOpenAI, ConnectionID: uuid.NewString(), Unit: "token_in", Quantity: 30}))
	assert.ErrorIs(t, rec.Record(e.ctx, catalog.UsageEntry{TenantID: e.tenant, Kind: catalog.KindOpenAI, ConnectionID: "platform:openai", Unit: "token_in"}), connectors.ErrInvalid)
	var platform, tenant int64
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT
		COALESCE(sum(quantity) FILTER (WHERE connection_id LIKE 'platform:%'), 0),
		COALESCE(sum(quantity) FILTER (WHERE connection_id NOT LIKE 'platform:%'), 0)
		FROM automation.connector_usage WHERE tenant_id = $1 AND kind = 'openai'`, e.tenant).Scan(&platform, &tenant))
	assert.Equal(t, int64(120), platform)
	assert.Equal(t, int64(30), tenant)
}

// ---- secret resolver

type fallback struct{}

func (fallback) Secret(_ context.Context, _ spi.Scope, name string) (string, error) {
	return "fallback:" + name, nil
}

func TestSecretResolverServesOnlySecretFields(t *testing.T) {
	e := newEnv(t)
	dir := connectors.NewDirectory(nil)
	require.NoError(t, dir.Register(telegramConnector(t)))
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	e.save(store, e.tenant, "support_bot", connectors.Credentials{"token": botToken, "username": "support"})
	r := connectors.SecretResolver(store, dir, fallback{})
	sc := spi.Scope{Subject: spi.Subject{TenantID: e.tenant}}

	v, err := r.Secret(e.ctx, sc, "support_bot.token")
	require.NoError(t, err)
	assert.Equal(t, botToken, v)
	_, err = r.Secret(e.ctx, sc, "support_bot.username")
	assert.ErrorIs(t, err, connectors.ErrNotFound, "not an x-secret field")
	v, err = r.Secret(e.ctx, sc, "OPENAI_KEY")
	require.NoError(t, err)
	assert.Equal(t, "fallback:OPENAI_KEY", v)
	v, err = r.Secret(e.ctx, sc, "other_bot.token")
	require.NoError(t, err)
	assert.Equal(t, "fallback:other_bot.token", v)
	// Another tenant cannot reach it.
	v, err = r.Secret(e.ctx, spi.Scope{Subject: spi.Subject{TenantID: e.newTenant()}}, "support_bot.token")
	require.NoError(t, err)
	assert.Equal(t, "fallback:support_bot.token", v)
	_, err = connectors.SecretResolver(store, dir, nil).Secret(e.ctx, sc, "nope")
	assert.ErrorIs(t, err, connectors.ErrNotFound)
}

// ---- OAuth2

type tokenServer struct {
	*httptest.Server
	refreshes atomic.Int32
	refuse    atomic.Value // "": grant; else the OAuth error code
	verifier  atomic.Value
}

func newTokenServer(t *testing.T) *tokenServer {
	ts := &tokenServer{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		w.Header().Set("Content-Type", "application/json")
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			ts.verifier.Store(r.Form.Get("code_verifier"))
			_, _ = w.Write([]byte(`{"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer","expires_in":60}`))
		case "refresh_token":
			ts.refreshes.Add(1)
			time.Sleep(50 * time.Millisecond) // a slow provider: the second caller waits on the row lock
			if code, _ := ts.refuse.Load().(string); code != "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"` + code + `","error_description":"refresh-1 is revoked"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"access-2","token_type":"Bearer","expires_in":3600}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestOAuth2(t *testing.T) {
	e := newEnv(t)
	ts := newTokenServer(t)
	conn := telegramConnector(t)
	conn.kind = "google_sheets"
	conn.nodes, conn.sources = nil, nil
	conn.auth = connectors.AuthOAuth2
	conn.oauth = &connectors.OAuth2Config{AuthURL: ts.URL + "/auth", TokenURL: ts.URL + "/token", Scopes: []string{"sheets"}, ClientCredential: "google_client"}
	dir := connectors.NewDirectory(nil)
	require.NoError(t, dir.Register(conn))
	platform := connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		"google_client": {Credentials: connectors.Credentials{"client_id": "cid", "client_secret": "csecret"}},
	})
	// A dotted key id, as rotations are often named.
	store := connectors.NewStore(e.db, keyring(t, "2026.11", "2026.11"), nil, platform, nil)
	o, err := connectors.NewOAuth2(store, dir, platform, connectors.WithHTTPClient(ts.Client()))
	require.NoError(t, err)

	authURL, state, err := o.Begin(e.ctx, e.tenant, "google_sheets", "https://app.test/callback")
	require.NoError(t, err)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "S256", u.Query().Get("code_challenge_method"))
	assert.Equal(t, state, u.Query().Get("state"))
	assert.NotContains(t, authURL, "csecret")

	// The state is bound to the tenant.
	_, err = o.Complete(e.ctx, e.newTenant(), state, "code")
	assert.ErrorIs(t, err, connectors.ErrInvalid)

	c, err := o.Complete(e.ctx, e.tenant, state, "code")
	require.NoError(t, err)
	assert.Equal(t, "google_sheets", c.Name)
	assert.Equal(t, "bot-1", c.Identity.External)
	assert.NotEmpty(t, ts.verifier.Load())

	// The token expires within 15 minutes: two parallel calls refresh once.
	var wg sync.WaitGroup
	tokens := make([]string, 2)
	errs := make([]error, 2)
	for i := range tokens {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tokens[i], errs[i] = o.Token(e.ctx, e.tenant, c.ID)
		}()
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.Equal(t, []string{"access-2", "access-2"}, tokens)
	assert.Equal(t, int32(1), ts.refreshes.Load())
	_, creds, err := store.Get(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "refresh-1", creds["refresh_token"], "a refresh without a new refresh token keeps the old one")

	// Fresh: no refresh.
	tok, err := o.Token(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok)
	assert.Equal(t, int32(1), ts.refreshes.Load())

	// Re-authorizing the same identity updates the connection instead of adding one.
	_, state, err = o.Begin(e.ctx, e.tenant, "google_sheets", "https://app.test/callback")
	require.NoError(t, err)
	again, err := o.Complete(e.ctx, e.tenant, state, "code")
	require.NoError(t, err)
	assert.Equal(t, c.ID, again.ID)

	// invalid_client is the platform's problem: an error, not a re-authorization of every tenant.
	ts.refuse.Store("invalid_client")
	_, err = o.Token(e.ctx, e.tenant, c.ID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, connectors.ErrNeedsReauth)
	assert.NotContains(t, err.Error(), "refresh-1")
	list, err := store.List(e.ctx, e.tenant, "google_sheets")
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusConnected, list[0].Status)

	// A refused grant: needs_reauth, ErrNeedsReauth, and the provider's text is not echoed.
	ts.refuse.Store("invalid_grant")
	_, err = o.Token(e.ctx, e.tenant, c.ID)
	require.ErrorIs(t, err, connectors.ErrNeedsReauth)
	list, err = store.List(e.ctx, e.tenant, "google_sheets")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, connectors.StatusNeedsReauth, list[0].Status)
	assert.NotContains(t, list[0].StatusReason, "refresh-1")
	_, err = o.Token(e.ctx, e.tenant, c.ID)
	assert.ErrorIs(t, err, connectors.ErrNeedsReauth)
	assert.False(t, strings.Contains(fmt.Sprint(err), "refresh-1"))

	_, err = o.Complete(e.ctx, e.tenant, "2026.11.garbage", "code")
	assert.True(t, errors.Is(err, connectors.ErrInvalid))
}
