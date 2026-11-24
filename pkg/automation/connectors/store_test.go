package connectors_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
	t.Helper()
	p, err := graph.FieldsParams(expr.Fields{{Name: "text", Field: expr.StringField}})
	require.NoError(t, err)
	return &telegram{
		nodes: []catalog.NodeType{node{spec: graph.NodeSpec{Type: "telegram.send", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture, Params: p, Outcomes: []string{"sent"}}}},
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
	require.Error(t, dir.Register(telegramConnector(t)), "a kind twice")

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
	require.ErrorIs(t, err, connectors.ErrNotFound)

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
	require.ErrorIs(t, err, connectors.ErrUndecryptable)
	_, _, err = store.Get(e.ctx, e.tenant, c.ID)
	require.ErrorIs(t, err, connectors.ErrNotFound, "the old tenant no longer has it")
	// The moved row is not advertised either: the catalog would offer what Open refuses.
	got, err := store.Connected(e.ctx, other)
	require.NoError(t, err)
	assert.Empty(t, got[catalog.KindTelegramBot])

	_, err = e.db.Exec(e.ctx, `UPDATE automation.connections SET tenant_id = $1, kind = 'smtp' WHERE id = $2`, e.tenant, c.ID)
	require.NoError(t, err)
	_, _, err = store.Get(e.ctx, e.tenant, c.ID)
	require.ErrorIs(t, err, connectors.ErrUndecryptable, "another kind")
	got, err = store.Connected(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Empty(t, got[catalog.KindSMTP])
}

// shortKeyring hands out a 16-byte key: the store must refuse it rather than fall back to AES-128.
type shortKeyring struct{}

func (shortKeyring) Active() (string, []byte)  { return "short", bytes.Repeat([]byte{1}, 16) }
func (shortKeyring) Key(string) ([]byte, bool) { return bytes.Repeat([]byte{1}, 16), true }

func TestStoreIsAES256Only(t *testing.T) {
	e := newEnv(t)
	store := connectors.NewStore(e.db, shortKeyring{}, nil, nil, nil)
	_, err := store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "bot"}, connectors.Credentials{"token": botToken})
	require.ErrorIs(t, err, connectors.ErrNotConfigured)
	var n int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.connections`).Scan(&n))
	assert.Zero(t, n)
}

// Rewrap waits for a row another transaction holds instead of reporting it done.
func TestRewrapWaitsForLockedRows(t *testing.T) {
	e := newEnv(t)
	old := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	locked := e.save(old, e.tenant, "locked_bot", connectors.Credentials{"token": botToken})
	e.save(old, e.tenant, "free_bot", connectors.Credentials{"token": botToken})

	tx, err := e.db.Begin(e.ctx)
	require.NoError(t, err)
	_, err = tx.Exec(e.ctx, `SELECT 1 FROM automation.connections WHERE id = $1 FOR UPDATE`, locked.ID)
	require.NoError(t, err)

	rotated := connectors.NewStore(e.db, keyring(t, "k2", "k1", "k2"), nil, nil, nil)
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := rotated.Rewrap(e.ctx, 10)
		done <- result{n, err}
	}()
	select {
	case r := <-done:
		t.Fatalf("Rewrap returned %d, %v while a row was locked", r.n, r.err)
	case <-time.After(300 * time.Millisecond):
	}
	require.NoError(t, tx.Rollback(e.ctx))
	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, 2, r.n)
	var left int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.connections WHERE key_id <> 'k2'`).Scan(&left))
	assert.Zero(t, left)
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
	require.ErrorIs(t, err, connectors.ErrUndecryptable)
	_, _, err = onlyNew.Get(e.ctx, tenants[0], ids[0])
	require.ErrorIs(t, err, connectors.ErrNotConfigured)
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
	require.ErrorIs(t, err, connectors.ErrNameTaken)
	_, err = store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindSMTP, Name: "support_bot"}, connectors.Credentials{})
	require.NoError(t, err)
	other := e.newTenant()
	_, err = store.Save(e.ctx, connectors.Connection{TenantID: other, Kind: catalog.KindTelegramBot, Name: "support_bot"}, connectors.Credentials{})
	require.NoError(t, err)
	_, err = store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "new_bot"}, nil)
	require.ErrorIs(t, err, connectors.ErrInvalid, "a new connection needs credentials")

	// Update: creds nil keeps them; a stale version conflicts.
	c.DisplayName = "Renamed"
	updated, err := store.Save(e.ctx, c, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)
	_, creds, err := store.Get(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, botToken, creds["token"])
	_, err = store.Save(e.ctx, c, nil)
	require.ErrorIs(t, err, connectors.ErrVersionConflict)
	updated, err = store.Save(e.ctx, updated, connectors.Credentials{"token": "rotated"})
	require.NoError(t, err)
	_, creds, err = store.Get(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "rotated", creds["token"])

	// Isolation: another tenant gets not found everywhere.
	_, _, err = store.Get(e.ctx, other, c.ID)
	require.ErrorIs(t, err, connectors.ErrNotFound)
	foreign := updated
	foreign.TenantID = other
	_, err = store.Save(e.ctx, foreign, nil)
	require.ErrorIs(t, err, connectors.ErrNotFound)
	require.ErrorIs(t, store.SetStatus(e.ctx, other, c.ID, connectors.StatusError, "x"), connectors.ErrNotFound)
	require.ErrorIs(t, store.Delete(e.ctx, other, c.ID), connectors.ErrNotFound)
	require.ErrorIs(t, store.UpdateInbound(e.ctx, other, c.ID, func(s json.RawMessage) (json.RawMessage, error) { return s, nil }), connectors.ErrNotFound)
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
				var st struct {
					Cursor int `json:"cursor"`
				}
				_ = json.Unmarshal(s, &st)
				st.Cursor++
				return json.Marshal(st)
			}))
		}()
	}
	wg.Wait()
	var cursor int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT (inbound->>'cursor')::int FROM automation.connections WHERE id = $1`, c.ID).Scan(&cursor))
	assert.Equal(t, 8, cursor)

	// Open: a connected connection; needs_reauth and error are refused.
	info, creds, err := store.Open(e.ctx, e.tenant, c.ID.String())
	require.NoError(t, err)
	assert.Equal(t, catalog.ConnectionInfo{ID: c.ID.String(), Kind: catalog.KindTelegramBot, Name: "support_bot"}, info)
	assert.Equal(t, "rotated", creds["token"])
	require.NoError(t, store.SetStatus(e.ctx, e.tenant, c.ID, connectors.StatusNeedsReauth, "expired"))
	_, _, err = store.Open(e.ctx, e.tenant, c.ID.String())
	require.ErrorIs(t, err, connectors.ErrNeedsReauth)
	_, _, err = store.Open(e.ctx, e.tenant, "not-a-uuid")
	require.ErrorIs(t, err, connectors.ErrNotFound)

	// Delete.
	require.NoError(t, store.Delete(e.ctx, e.tenant, c.ID))
	_, _, err = store.Get(e.ctx, e.tenant, c.ID)
	require.ErrorIs(t, err, connectors.ErrNotFound)
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
	require.ErrorIs(t, err, connectors.ErrNotConfigured)
	_, err = store.List(e.ctx, e.tenant, "")
	require.ErrorIs(t, err, connectors.ErrNotConfigured)
	_, err = store.Rewrap(e.ctx, 10)
	require.ErrorIs(t, err, connectors.ErrNotConfigured)
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
	require.ErrorIs(t, rec.Record(e.ctx, catalog.UsageEntry{TenantID: e.tenant, Kind: catalog.KindOpenAI, ConnectionID: "platform:openai", Unit: "token_in"}), connectors.ErrInvalid)
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
	require.ErrorIs(t, err, connectors.ErrNotFound, "not an x-secret field")
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
	require.ErrorIs(t, err, connectors.ErrNotFound)
}

// ---- OAuth2

type tokenServer struct {
	*httptest.Server
	refreshes atomic.Int32
	refuse    atomic.Value // "": grant; else the OAuth error code
	lifetime  atomic.Int32 // seconds of a refreshed token; 0: 3600
	verifier  atomic.Value
}

func newTokenServer(t *testing.T) *tokenServer {
	t.Helper()
	ts := &tokenServer{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
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
			life := ts.lifetime.Load()
			if life == 0 {
				life = 3600
			}
			_, _ = fmt.Fprintf(w, `{"access_token":"access-%d","token_type":"Bearer","expires_in":%d}`, 1+ts.refreshes.Load(), life)
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
	require.ErrorIs(t, err, connectors.ErrInvalid)

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

	// A provider issuing tokens shorter-lived than RefreshBefore: parallel callers still refresh once.
	ts.lifetime.Store(300)
	before := ts.refreshes.Load()
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
	assert.Equal(t, tokens[0], tokens[1])
	assert.Equal(t, before+1, ts.refreshes.Load())
	ts.lifetime.Store(0)

	// A connection disabled while a caller waits for the lock gives no token.
	_, state, err = o.Begin(e.ctx, e.tenant, "google_sheets", "https://app.test/callback")
	require.NoError(t, err)
	_, err = o.Complete(e.ctx, e.tenant, state, "code") // expiring again
	require.NoError(t, err)
	tx, err := e.db.Begin(e.ctx)
	require.NoError(t, err)
	_, err = tx.Exec(e.ctx, `SELECT 1 FROM automation.connections WHERE id = $1 FOR UPDATE`, c.ID)
	require.NoError(t, err)
	waited := make(chan error, 1)
	go func() {
		_, err := o.Token(e.ctx, e.tenant, c.ID)
		waited <- err
	}()
	for i := 0; ; i++ {
		var waiting int
		require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND datname = current_database()`).Scan(&waiting))
		if waiting > 0 {
			break
		}
		require.Less(t, i, 200, "Token never waited for the row lock")
		time.Sleep(10 * time.Millisecond)
	}
	_, err = tx.Exec(e.ctx, `UPDATE automation.connections SET status = 'disabled', version = version + 1 WHERE id = $1`, c.ID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(e.ctx))
	require.ErrorIs(t, <-waited, connectors.ErrNotConnected)
	require.NoError(t, store.SetStatus(e.ctx, e.tenant, c.ID, connectors.StatusConnected, ""))

	// invalid_client is the platform's problem: an error, not a re-authorization of every tenant.
	ts.refuse.Store("invalid_client")
	_, err = o.Token(e.ctx, e.tenant, c.ID)
	require.Error(t, err)
	require.NotErrorIs(t, err, connectors.ErrNeedsReauth)
	assert.NotContains(t, err.Error(), "refresh-1")
	list, err := store.List(e.ctx, e.tenant, "google_sheets")
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusConnected, list[0].Status)

	// Provider-controlled fields (error code, description, status text) never reach Error().
	ts.refuse.Store("leak_refresh-1_echo")
	_, err = o.Token(e.ctx, e.tenant, c.ID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "refresh-1")
	assert.NotContains(t, err.Error(), "leak")
	assert.Contains(t, err.Error(), "HTTP 400")

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
	require.ErrorIs(t, err, connectors.ErrNeedsReauth)
	assert.NotContains(t, fmt.Sprint(err), "refresh-1")

	_, err = o.Complete(e.ctx, e.tenant, "2026.11.garbage", "code")
	require.ErrorIs(t, err, connectors.ErrInvalid)
}

// staleList answers List with an earlier snapshot, as if the row changed between List and Get.
type staleList struct {
	connectors.ConnectionStore
	snapshot []connectors.Connection
}

func (s staleList) List(context.Context, uuid.UUID, catalog.ConnectorKind) ([]connectors.Connection, error) {
	return s.snapshot, nil
}

func TestSecretResolverChecksTheRowItReads(t *testing.T) {
	e := newEnv(t)
	dir := connectors.NewDirectory(nil)
	require.NoError(t, dir.Register(telegramConnector(t)))
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	c := e.save(store, e.tenant, "support_bot", connectors.Credentials{"token": botToken})
	snapshot, err := store.List(e.ctx, e.tenant, "")
	require.NoError(t, err)
	r := connectors.SecretResolver(staleList{ConnectionStore: store, snapshot: snapshot}, dir, nil)
	sc := spi.Scope{Subject: spi.Subject{TenantID: e.tenant}}

	// Disabled after List.
	require.NoError(t, store.SetStatus(e.ctx, e.tenant, c.ID, connectors.StatusDisabled, "paused"))
	v, err := r.Secret(e.ctx, sc, "support_bot.token")
	require.ErrorIs(t, err, connectors.ErrNotConnected)
	assert.Empty(t, v)

	// Renamed after List.
	require.NoError(t, store.SetStatus(e.ctx, e.tenant, c.ID, connectors.StatusConnected, ""))
	cur, _, err := store.Get(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	cur.Name = "renamed_bot"
	_, err = store.Save(e.ctx, cur, nil)
	require.NoError(t, err)
	v, err = r.Secret(e.ctx, sc, "support_bot.token")
	require.ErrorIs(t, err, connectors.ErrNotFound)
	assert.Empty(t, v)
}

// An automatic refresh replaces the tokens and keeps the metadata: a display name or identity that
// carries the old access token — a row written before this check, through the public Save — is cleared
// as the token changes, since the old token could not be recognised in it afterwards. The name stays.
func TestRefreshScrubsMetadataCarryingTheOldToken(t *testing.T) {
	e := newEnv(t)
	ts := newTokenServer(t)
	conn := telegramConnector(t)
	conn.kind = "google_sheets"
	conn.nodes, conn.sources = nil, nil
	conn.auth = connectors.AuthOAuth2
	conn.oauth = &connectors.OAuth2Config{AuthURL: ts.URL + "/auth", TokenURL: ts.URL + "/token", ClientCredential: "google_client"}
	dir := connectors.NewDirectory(nil)
	require.NoError(t, dir.Register(conn))
	platform := connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		"google_client": {Credentials: connectors.Credentials{"client_id": "cid", "client_secret": "csecret"}},
	})
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, platform, nil)
	o, err := connectors.NewOAuth2(store, dir, platform, connectors.WithHTTPClient(ts.Client()))
	require.NoError(t, err)

	// A legacy row: the provider's display, with the access token in it, copied into the metadata.
	legacy, err := store.Save(e.ctx, connectors.Connection{
		TenantID: e.tenant, Kind: "google_sheets", Name: "sheets_access", DisplayName: "docs access-1",
		Identity: connectors.Identity{Display: "@access_1", External: "acct access-1", Kind: connectors.IdentityHandle},
		Status:   connectors.StatusConnected,
	}, connectors.Credentials{"access_token": "access-1", "refresh_token": "refresh-1", "expires_at": time.Now().Add(time.Minute).UTC().Format(time.RFC3339)})
	require.NoError(t, err)

	tok, err := o.Token(e.ctx, e.tenant, legacy.ID)
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok, "refreshed")
	got, creds, err := store.Get(e.ctx, e.tenant, legacy.ID)
	require.NoError(t, err)
	assert.Equal(t, "access-2", creds["access_token"])
	assert.NotContains(t, got.DisplayName, "access-1")
	assert.NotContains(t, got.Identity.External, "access-1")
	assert.Equal(t, "sheets_access", got.Name, "the name is never changed")
	assert.Equal(t, "sheets_access", got.DisplayName, "the display falls back to the name")
}

// A disabled connection's token is refreshed for a check without enabling it; a refused refresh writes
// nothing. Token itself still refuses a disabled connection.
func TestCheckTokenRefreshesADisabledConnection(t *testing.T) {
	e := newEnv(t)
	ts := newTokenServer(t)
	conn := telegramConnector(t)
	conn.kind = "google_sheets"
	conn.nodes, conn.sources = nil, nil
	conn.auth = connectors.AuthOAuth2
	conn.oauth = &connectors.OAuth2Config{AuthURL: ts.URL + "/auth", TokenURL: ts.URL + "/token", ClientCredential: "google_client"}
	dir := connectors.NewDirectory(nil)
	require.NoError(t, dir.Register(conn))
	platform := connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		"google_client": {Credentials: connectors.Credentials{"client_id": "cid", "client_secret": "csecret"}},
	})
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, platform, nil)
	o, err := connectors.NewOAuth2(store, dir, platform, connectors.WithHTTPClient(ts.Client()))
	require.NoError(t, err)
	expired := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	save := func(name string) connectors.Connection {
		c, err := store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: "google_sheets", Name: name, Status: connectors.StatusDisabled},
			connectors.Credentials{"access_token": "access-1", "refresh_token": "refresh-1", "expires_at": expired})
		require.NoError(t, err)
		return c
	}
	c := save("sheets")
	_, err = o.Token(e.ctx, e.tenant, c.ID)
	require.ErrorIs(t, err, connectors.ErrNotConnected)

	checker, ok := o.(connectors.OAuth2Checker)
	require.True(t, ok)
	tok, err := checker.CheckToken(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok)
	got, creds, err := store.Get(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusDisabled, got.Status, "the check does not enable it")
	assert.Equal(t, "access-2", creds["access_token"])

	refused := save("sheets_refused")
	ts.refuse.Store("invalid_grant")
	_, err = checker.CheckToken(e.ctx, e.tenant, refused.ID)
	require.ErrorIs(t, err, connectors.ErrNeedsReauth)
	after, _, err := store.Get(e.ctx, e.tenant, refused.ID)
	require.NoError(t, err)
	assert.Equal(t, refused.Version, after.Version, "a refused check writes nothing")
	assert.Equal(t, connectors.StatusDisabled, after.Status)
}

// An update a connector handles itself is claimed before its handler runs and again before its reply is sent:
// two deliveries never both do either, and a claimant that was taken over cannot touch its successor's work.
func TestInboundActionsClaimOnce(t *testing.T) {
	e := newEnv(t)
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	acts, ok := store.(connectors.InboundActions)
	require.True(t, ok)
	const conn, event, key = "platform:telegram_bot@222222", "start", "u1"

	var claims atomic.Int32
	owners := make(chan uuid.UUID, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if a, claimed, err := acts.ClaimInbound(e.ctx, conn, event, key); err == nil && claimed {
				claims.Add(1)
				owners <- a.Owner
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), claims.Load(), "one delivery runs the handler")
	owner := <-owners
	require.NotEqual(t, uuid.Nil, owner)

	a, claimed, err := acts.ClaimInbound(e.ctx, conn, event, key)
	require.NoError(t, err)
	assert.False(t, claimed)
	assert.Equal(t, connectors.InboundClaimed, a.State)

	tenant := uuid.New()
	require.ErrorIs(t, acts.InboundHandled(e.ctx, conn, event, key, uuid.New(), tenant, "x"), connectors.ErrInboundLost, "not the claimant")
	require.NoError(t, acts.InboundHandled(e.ctx, conn, event, key, owner, tenant, "Linked"))
	a, claimed, err = acts.ClaimInbound(e.ctx, conn, event, key)
	require.NoError(t, err)
	assert.False(t, claimed)
	assert.Equal(t, connectors.InboundAction{State: connectors.InboundHandled, Tenant: tenant, Reply: "Linked"}, a)
	require.ErrorIs(t, acts.ReleaseInbound(e.ctx, conn, event, key, owner), connectors.ErrInboundLost, "a handled update is not released by the handler's claim")

	// The delivery is claimed once, with the kept result.
	claims.Store(0)
	deliveries := make(chan connectors.InboundAction, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if d, claimed, err := acts.ClaimDelivery(e.ctx, conn, event, key); err == nil && claimed {
				claims.Add(1)
				deliveries <- d
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), claims.Load(), "one delivery sends the reply")
	d := <-deliveries
	assert.Equal(t, "Linked", d.Reply)
	assert.Equal(t, tenant, d.Tenant)
	a, claimed, err = acts.ClaimDelivery(e.ctx, conn, event, key)
	require.NoError(t, err)
	assert.False(t, claimed)
	assert.Equal(t, connectors.InboundDelivering, a.State)

	// A delivery that could not send gives its claim up: the reply is sent by the next one.
	require.NoError(t, acts.ReleaseInbound(e.ctx, conn, event, key, d.Owner))
	d2, claimed, err := acts.ClaimDelivery(e.ctx, conn, event, key)
	require.NoError(t, err)
	require.True(t, claimed)
	require.ErrorIs(t, acts.InboundCompleted(e.ctx, conn, event, key, d.Owner), connectors.ErrInboundLost, "the released claim is not the owner any more")
	require.NoError(t, acts.InboundCompleted(e.ctx, conn, event, key, d2.Owner))
	a, claimed, err = acts.ClaimInbound(e.ctx, conn, event, key)
	require.NoError(t, err)
	assert.False(t, claimed)
	assert.Equal(t, connectors.InboundDone, a.State)
	_, claimed, err = acts.ClaimDelivery(e.ctx, conn, event, key)
	require.NoError(t, err)
	assert.False(t, claimed, "a finished update is not delivered again")

	// A failed handler gives its claim up; one that was abandoned is taken over, and the old claimant is fenced.
	first, claimed, err := acts.ClaimInbound(e.ctx, conn, event, "u2")
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, acts.ReleaseInbound(e.ctx, conn, event, "u2", first.Owner))
	second, claimed, err := acts.ClaimInbound(e.ctx, conn, event, "u2")
	require.NoError(t, err)
	assert.True(t, claimed, "released: the redelivery runs the handler")
	_, err = e.db.Exec(e.ctx, `UPDATE automation.inbound_actions SET updated_at = now() - interval '1 hour' WHERE dedup_key = 'u2'`)
	require.NoError(t, err)
	third, claimed, err := acts.ClaimInbound(e.ctx, conn, event, "u2")
	require.NoError(t, err)
	assert.True(t, claimed, "abandoned: taken over")
	require.NotEqual(t, second.Owner, third.Owner)
	_, claimed, err = acts.ClaimInbound(e.ctx, conn, event, "u2")
	require.NoError(t, err)
	assert.False(t, claimed, "and fresh again")
	require.ErrorIs(t, acts.InboundHandled(e.ctx, conn, event, "u2", second.Owner, tenant, "late"), connectors.ErrInboundLost, "the old handler's result is not recorded")
	require.ErrorIs(t, acts.ReleaseInbound(e.ctx, conn, event, "u2", second.Owner), connectors.ErrInboundLost, "nor does its failure drop the successor's claim")
	require.NoError(t, acts.InboundHandled(e.ctx, conn, event, "u2", third.Owner, tenant, "ok"))

	// The same for the delivery.
	_, err = e.db.Exec(e.ctx, `UPDATE automation.inbound_actions SET updated_at = now() - interval '1 hour' WHERE dedup_key = 'u2'`)
	require.NoError(t, err)
	old, claimed, err := acts.ClaimDelivery(e.ctx, conn, event, "u2")
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = e.db.Exec(e.ctx, `UPDATE automation.inbound_actions SET updated_at = now() - interval '1 hour' WHERE dedup_key = 'u2'`)
	require.NoError(t, err)
	next, claimed, err := acts.ClaimDelivery(e.ctx, conn, event, "u2")
	require.NoError(t, err)
	require.True(t, claimed, "an abandoned delivery is taken over")
	require.ErrorIs(t, acts.InboundCompleted(e.ctx, conn, event, "u2", old.Owner), connectors.ErrInboundLost)
	require.ErrorIs(t, acts.ReleaseInbound(e.ctx, conn, event, "u2", old.Owner), connectors.ErrInboundLost)
	require.NoError(t, acts.InboundCompleted(e.ctx, conn, event, "u2", next.Owner))
}

// DeleteReturning gives what the row held when it was deleted (under its lock), not what an earlier read saw.
func TestDeleteReturningGivesTheDeletedRow(t *testing.T) {
	e := newEnv(t)
	store := connectors.NewStore(e.db, keyring(t, "k1", "k1"), nil, nil, nil)
	c := e.save(store, e.tenant, "support_bot", connectors.Credentials{"token": botToken})
	c2, err := store.Save(e.ctx, connectors.Connection{ID: c.ID, TenantID: e.tenant, Kind: catalog.KindTelegramBot, Name: "support_bot", DisplayName: "Support", Version: c.Version},
		connectors.Credentials{"token": botToken + "-rotated"})
	require.NoError(t, err)
	require.Equal(t, c.ID, c2.ID, "saved again: the same connection")
	got, creds, err := store.DeleteReturning(e.ctx, e.tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.ID, got.ID)
	assert.Equal(t, botToken+"-rotated", creds["token"], "the credentials of the deleted row")
	_, _, err = store.DeleteReturning(e.ctx, e.tenant, c.ID)
	require.ErrorIs(t, err, connectors.ErrNotFound)
}
