package host_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool/pooltest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

const itemEntity = "acme.test.item"

// env is one real PostgreSQL database, one real Node pool and the test doubles
// of the neighbouring slices (data store, catalog, artifact resolver).
type env struct {
	t        *testing.T
	db       *pgxpool.Pool
	tenant   uuid.UUID
	pool     *pool.Pool
	host     *host.Host
	bindings *host.PgBindings
	catalog  *catalog
	store    *itemStore
	art      *artifacts
	outbox   *outboxDouble
	starter  *starterDouble
	notifier *notifierDouble
	wf       *workflowDouble
}

type envOption func(*host.Config)

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_host",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	e := &env{t: t, db: scope.Pool, tenant: scope.Tenant.ID}
	_, err := e.db.Exec(context.Background(), `CREATE TABLE host_items (id bigserial PRIMARY KEY, tenant_id uuid NOT NULL, label text NOT NULL);
		CREATE TABLE host_outbox (id bigserial PRIMARY KEY, event text NOT NULL, payload jsonb)`)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e.pool, err = pool.New(pool.Config{
		Size:              1,
		WorkerScript:      pooltest.BuildWorker(t),
		CancelGrace:       300 * time.Millisecond,
		RestartBackoffMin: 20 * time.Millisecond,
		Logger:            logger,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = e.pool.Close(context.Background()) })

	e.catalog = &catalog{res: map[string]spi.ResolvedResource{}}
	e.store = &itemStore{db: e.db}
	e.art = &artifacts{byVersion: map[string]pool.Artifact{}, secrets: map[string][]string{}}
	e.bindings = host.NewPgBindings(e.db, e.art)
	e.outbox, e.starter, e.notifier, e.wf = &outboxDouble{}, &starterDouble{}, &notifierDouble{}, &workflowDouble{}

	cfg := host.Config{
		Pool: e.pool, DB: e.db, Bindings: e.bindings, Catalog: e.catalog, Data: e.store,
		Outbox: e.outbox, Starter: e.starter, Notifier: e.notifier, Workflow: e.wf,
		Logger: logger,
	}
	for _, o := range opts {
		o(&cfg)
	}
	e.host, err = host.New(cfg)
	require.NoError(t, err)

	e.catalog.res["core.native"] = spi.ResolvedResource{Native: true, Resource: manifest.Resource{ID: "core.native", Kind: manifest.ResEntity}}
	e.catalog.res[itemEntity] = spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: manifest.Resource{ID: itemEntity, Kind: manifest.ResEntity}}
	return e
}

func (e *env) subject(perms ...string) spi.Subject {
	return spi.Subject{TenantID: e.tenant, UserID: 7, Kind: spi.SubjectInitiator, Permissions: perms, Locale: "en"}
}

func (e *env) req(contract, input string, perms ...string) host.Request {
	return host.Request{Contract: contract, Input: json.RawMessage(input), Subject: e.subject(perms...), Trigger: host.Trigger{Kind: host.TriggerUI, Ref: "test"}}
}

// publish registers a package version's artifact (source of server.mjs).
func (e *env) publish(pkg, version, source string) {
	e.art.add(e.t, pkg, version, source)
}

// contract declares a command/operation/query contract in the catalog and
// binds it to pkg@version's export.
func (e *env) contract(res manifest.Resource, pkg, version, export string) {
	e.catalog.res[res.ID] = spi.ResolvedResource{Resource: res, Package: pkg, Version: version}
	_, err := e.bindings.Activate(context.Background(), res.ID, pkg, version, export, nil)
	require.NoError(e.t, err)
}

func (e *env) items() []string {
	rows, err := e.db.Query(context.Background(), `SELECT label FROM host_items ORDER BY id`)
	require.NoError(e.t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		require.NoError(e.t, rows.Scan(&l))
		out = append(out, l)
	}
	return out
}

func (e *env) idleInTx() int {
	var n int
	require.NoError(e.t, e.db.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state LIKE 'idle in transaction%'`).Scan(&n))
	return n
}

type invocationRow struct {
	ID, Parent, Root, Contract, Package, Version, Export, Outcome, TriggerKind string
	Committed                                                                  int
	Partial                                                                    bool
	Result, Error                                                              *string
	Finished                                                                   bool
}

func (e *env) invocations(contract string) []invocationRow {
	rows, err := e.db.Query(context.Background(), `
		SELECT id::text, coalesce(parent_invocation_id::text, ''), root_id::text, contract, binding_package, binding_version, binding_export,
		       outcome, trigger_kind, committed_tx_count, partially_committed, result::text, error::text, finished_at IS NOT NULL
		FROM extruntime.invocations WHERE contract = $1 ORDER BY started_at, id`, contract)
	require.NoError(e.t, err)
	defer rows.Close()
	var out []invocationRow
	for rows.Next() {
		var r invocationRow
		require.NoError(e.t, rows.Scan(&r.ID, &r.Parent, &r.Root, &r.Contract, &r.Package, &r.Version, &r.Export, &r.Outcome, &r.TriggerKind, &r.Committed, &r.Partial, &r.Result, &r.Error, &r.Finished))
		out = append(out, r)
	}
	require.NoError(e.t, rows.Err())
	return out
}

// ---- manifest type helpers ----

func str() manifest.Type     { return manifest.Type{Kind: manifest.KindString} }
func num() manifest.Type     { return manifest.Type{Kind: manifest.KindInt} }
func boolean() manifest.Type { return manifest.Type{Kind: manifest.KindBool} }

func obj(props ...any) *manifest.Type {
	t := manifest.Type{Kind: manifest.KindObject}
	for i := 0; i < len(props); i += 2 {
		t.Properties = append(t.Properties, manifest.Property{Name: props[i].(string), Type: props[i+1].(manifest.Type)})
	}
	return &t
}

func optional(t manifest.Type) manifest.Type { t.Optional = true; return t }

func command(id string, atomic bool, in, out *manifest.Type, errs ...string) manifest.Resource {
	return manifest.Resource{ID: id, Kind: manifest.ResCommand, Atomic: atomic, Input: in, Output: out, Errors: errs}
}

// ---- artifacts ----

type artifacts struct {
	mu        sync.Mutex
	byVersion map[string]pool.Artifact
	secrets   map[string][]string
}

func (a *artifacts) add(t *testing.T, pkg, version, source string) {
	art := pooltest.Artifact(t, source)
	a.mu.Lock()
	a.byVersion[pkg+"@"+version] = art
	a.mu.Unlock()
}

func (a *artifacts) ResolveArtifact(_ context.Context, pkg, version string) (pool.Artifact, []string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	art, ok := a.byVersion[pkg+"@"+version]
	if !ok {
		return pool.Artifact{}, nil, fmt.Errorf("no artifact for %s@%s", pkg, version)
	}
	return art, a.secrets[pkg], nil
}

// ---- catalog ----

type catalog struct {
	res map[string]spi.ResolvedResource
}

func (c *catalog) Resolve(_ context.Context, id string) (spi.ResolvedResource, bool, error) {
	r, ok := c.res[id]
	return r, ok, nil
}
func (c *catalog) Entities(context.Context) ([]spi.ResolvedResource, error) { return nil, nil }

func (c *catalog) Guards(context.Context, string, manifest.GuardPhase) ([]spi.ResolvedGuard, error) {
	return nil, nil
}

// ---- data store double: one real table ----

type itemStore struct {
	db     *pgxpool.Pool
	mu     sync.Mutex
	scopes []spi.Scope
}

func (s *itemStore) seen(sc spi.Scope) {
	s.mu.Lock()
	s.scopes = append(s.scopes, sc)
	s.mu.Unlock()
}

func (s *itemStore) callers() []spi.Caller {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]spi.Caller, len(s.scopes))
	for i, sc := range s.scopes {
		out[i] = sc.Caller
	}
	return out
}

func (s *itemStore) Get(ctx context.Context, tx pgx.Tx, sc spi.Scope, _, key string) (json.RawMessage, error) {
	s.seen(sc)
	var label string
	err := tx.QueryRow(ctx, `SELECT label FROM host_items WHERE id = $1 AND tenant_id = $2`, key, sc.Subject.TenantID).Scan(&label)
	if err == pgx.ErrNoRows {
		return json.RawMessage(`null`), nil
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"key": key, "label": label})
}

func (s *itemStore) Find(ctx context.Context, tx pgx.Tx, sc spi.Scope, _ string, _ spi.FindQuery) (json.RawMessage, error) {
	s.seen(sc)
	rows, err := tx.Query(ctx, `SELECT id, label FROM host_items WHERE tenant_id = $1 ORDER BY id`, sc.Subject.TenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id int64
		var label string
		if err := rows.Scan(&id, &label); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{"key": strconv.FormatInt(id, 10), "label": label})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func (s *itemStore) Count(ctx context.Context, tx pgx.Tx, sc spi.Scope, _ string, _ *manifest.Filter) (int64, error) {
	s.seen(sc)
	var n int64
	err := tx.QueryRow(ctx, `SELECT count(*) FROM host_items WHERE tenant_id = $1`, sc.Subject.TenantID).Scan(&n)
	return n, err
}

func (s *itemStore) Insert(ctx context.Context, tx pgx.Tx, sc spi.Scope, _ string, values json.RawMessage) (json.RawMessage, error) {
	s.seen(sc)
	var v struct{ Label string }
	if err := json.Unmarshal(values, &v); err != nil {
		return nil, err
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO host_items (tenant_id, label) VALUES ($1, $2) RETURNING id`, sc.Subject.TenantID, v.Label).Scan(&id); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"key": strconv.FormatInt(id, 10), "label": v.Label})
}

func (s *itemStore) Update(context.Context, pgx.Tx, spi.Scope, string, string, json.RawMessage) (json.RawMessage, error) {
	return nil, fmt.Errorf("itemStore.Update is not needed by these tests")
}
func (s *itemStore) Delete(context.Context, pgx.Tx, spi.Scope, string, string) error {
	return fmt.Errorf("itemStore.Delete is not needed by these tests")
}
func (s *itemStore) PutOverlay(context.Context, pgx.Tx, spi.Scope, string, string, json.RawMessage) (json.RawMessage, error) {
	return nil, fmt.Errorf("itemStore.PutOverlay is not needed by these tests")
}

// ---- producer SPI doubles: write through the tx they are handed ----

type outboxDouble struct {
	mu     sync.Mutex
	events []spi.EventIntent
	scopes []spi.Scope
}

func (o *outboxDouble) Emit(ctx context.Context, tx pgx.Tx, sc spi.Scope, e spi.EventIntent) (string, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO host_outbox (event, payload) VALUES ($1, $2)`, e.Event, []byte(e.Payload)); err != nil {
		return "", err
	}
	o.mu.Lock()
	o.events = append(o.events, e)
	o.scopes = append(o.scopes, sc)
	o.mu.Unlock()
	return "evt-" + strconv.Itoa(len(o.events)), nil
}

type starterDouble struct {
	mu     sync.Mutex
	starts []spi.StartIntent
}

func (s *starterDouble) Start(_ context.Context, _ pgx.Tx, _ spi.Scope, in spi.StartIntent) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts = append(s.starts, in)
	return "wfi-" + strconv.Itoa(len(s.starts)), nil
}

type notifierDouble struct {
	mu      sync.Mutex
	notices []spi.NoticeIntent
}

func (n *notifierDouble) Enqueue(_ context.Context, _ pgx.Tx, _ spi.Scope, in spi.NoticeIntent) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notices = append(n.notices, in)
	return "ntc-" + strconv.Itoa(len(n.notices)), nil
}

// ---- workflow handler double ----

type workflowDouble struct {
	mu      sync.Mutex
	methods []string
	scopes  []spi.Scope
}

func (w *workflowDouble) CallWorkflow(ctx context.Context, call host.WorkflowCall, method string, args json.RawMessage) (json.RawMessage, error) {
	w.mu.Lock()
	w.methods = append(w.methods, method)
	w.scopes = append(w.scopes, call.Scope())
	w.mu.Unlock()
	switch method {
	case "wf.sleep":
		return nil, &pool.WireError{Kind: pool.ErrKindSuspend, Suspend: json.RawMessage(`{"wakeAt":"2031-01-01T00:00:00Z"}`)}
	case "wf.transaction.begin":
		h, err := call.OpenTx(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"replay": false, "tx": h})
	case "wf.transaction.call":
		var a struct {
			Callable string
			Input    json.RawMessage
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, err
		}
		h, err := call.OpenTx(ctx)
		if err != nil {
			return nil, err
		}
		out, err := call.CallInTx(ctx, h, a.Callable, a.Input)
		if err != nil {
			_ = call.EndTx(ctx, h, host.TxRollback)
			return nil, err
		}
		return out, call.EndTx(ctx, h, host.TxCommit)
	case "wf.transaction.end":
		var a struct {
			Tx      string
			Outcome host.TxOutcome
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, err
		}
		return json.RawMessage(`null`), call.EndTx(ctx, a.Tx, a.Outcome)
	}
	return nil, fmt.Errorf("workflowDouble: unexpected %s", method)
}
