// Package hosttest is test support for the extension runtime slices: one real
// PostgreSQL database per test, real Node pools, and the doubles of the
// neighbouring slices (catalog, artifact registry, data store, subject loader).
// It is never imported by production code.
package hosttest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
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

// ItemEntity is the entity of the items table the ItemStore serves.
const ItemEntity = "acme.test.item"

// Env is one database, one tenant and the shared doubles.
type Env struct {
	T         *testing.T
	DB        *pgxpool.Pool
	Tenant    uuid.UUID
	Catalog   *Catalog
	Artifacts *Artifacts
	Bindings  *host.PgBindings
	Store     *ItemStore
	Subjects  *Subjects
	Logger    *slog.Logger

	worker string
}

// New creates the database (with the repository migrations applied) and a
// test table host_items served by Store.
func New(t *testing.T) *Env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	e := &Env{T: t, DB: scope.Pool, Tenant: scope.Tenant.ID}
	_, err := e.DB.Exec(context.Background(), `CREATE TABLE host_items (id bigserial PRIMARY KEY, tenant_id uuid NOT NULL, label text NOT NULL)`)
	require.NoError(t, err)
	e.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e.Catalog = NewCatalog()
	e.Artifacts = &Artifacts{byVersion: map[string]pool.Artifact{}, secrets: map[string][]string{}}
	e.Bindings = host.NewPgBindings(e.DB, e.Artifacts)
	e.Store = &ItemStore{DB: e.DB}
	e.Subjects = NewSubjects()
	e.worker = pooltest.BuildWorker(t)
	e.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: manifest.Resource{ID: ItemEntity, Kind: manifest.ResEntity}})
	return e
}

// NewPool starts a Node pool; closed when the test ends. Several pools on one
// Env model a process restart.
func (e *Env) NewPool(size int) *pool.Pool {
	e.T.Helper()
	p, err := pool.New(pool.Config{
		Size:              size,
		WorkerScript:      e.worker,
		CancelGrace:       300 * time.Millisecond,
		RestartBackoffMin: 20 * time.Millisecond,
		Logger:            e.Logger,
	})
	require.NoError(e.T, err)
	e.T.Cleanup(func() { _ = p.Close(context.Background()) })
	return p
}

// NewTenant inserts another tenant.
func (e *Env) NewTenant() uuid.UUID {
	e.T.Helper()
	id := uuid.New()
	_, err := e.DB.Exec(context.Background(),
		`INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`, id, "t-"+id.String()[:8], id.String()[:8]+".test")
	require.NoError(e.T, err)
	return id
}

// Subject is a user with permissions of the env tenant.
func (e *Env) Subject(userID int64, perms ...string) spi.Subject {
	return spi.Subject{TenantID: e.Tenant, UserID: userID, Kind: spi.SubjectInitiator, Permissions: perms, Locale: "en"}
}

// Items lists the labels in host_items in insertion order.
func (e *Env) Items() []string {
	rows, err := e.DB.Query(context.Background(), `SELECT label FROM host_items WHERE tenant_id = $1 ORDER BY id`, e.Tenant)
	require.NoError(e.T, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		require.NoError(e.T, rows.Scan(&l))
		out = append(out, l)
	}
	require.NoError(e.T, rows.Err())
	return out
}

// Eventually polls cond until it holds or the timeout passes.
func (e *Env) Eventually(timeout time.Duration, cond func() bool, msg string, args ...any) {
	e.T.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	require.Fail(e.T, "condition not met within "+timeout.String(), fmt.Sprintf(msg, args...))
}

// ---- manifest type helpers ----

func Str() manifest.Type  { return manifest.Type{Kind: manifest.KindString} }
func Int() manifest.Type  { return manifest.Type{Kind: manifest.KindInt} }
func Bool() manifest.Type { return manifest.Type{Kind: manifest.KindBool} }

// Obj builds an object type from alternating name, type arguments.
func Obj(props ...any) *manifest.Type {
	t := manifest.Type{Kind: manifest.KindObject}
	for i := 0; i < len(props); i += 2 {
		t.Properties = append(t.Properties, manifest.Property{Name: props[i].(string), Type: props[i+1].(manifest.Type)})
	}
	return &t
}

// Optional marks a type's object key as optional.
func Optional(t manifest.Type) manifest.Type { t.Optional = true; return t }

// JSONType wraps a type pointer as a value for properties.
func JSONType(t *manifest.Type) manifest.Type { return *t }

func Command(id string, atomic bool, in, out *manifest.Type, errs ...string) manifest.Resource {
	return manifest.Resource{ID: id, Kind: manifest.ResCommand, Atomic: atomic, Input: in, Output: out, Errors: errs}
}

func Operation(id string, in, out *manifest.Type, errs ...string) manifest.Resource {
	return manifest.Resource{ID: id, Kind: manifest.ResOperation, Input: in, Output: out, Errors: errs}
}

// ---- artifacts ----

// Artifacts is the artifact registry of package versions.
type Artifacts struct {
	mu        sync.Mutex
	byVersion map[string]pool.Artifact
	secrets   map[string][]string
}

// Add registers the server.mjs source of pkg@version.
func (a *Artifacts) Add(t testing.TB, pkg, version, source string) pool.Artifact {
	art := pooltest.Artifact(t, source)
	a.mu.Lock()
	a.byVersion[pkg+"@"+version] = art
	a.mu.Unlock()
	return art
}

func (a *Artifacts) ResolveArtifact(_ context.Context, pkg, version string) (pool.Artifact, []string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	art, ok := a.byVersion[pkg+"@"+version]
	if !ok {
		return pool.Artifact{}, nil, fmt.Errorf("no artifact for %s@%s", pkg, version)
	}
	return art, a.secrets[pkg], nil
}

// ---- catalog ----

// Catalog is a mutable spi.Catalog.
type Catalog struct {
	mu     sync.Mutex
	res    map[string]spi.ResolvedResource
	guards []spi.ResolvedGuard
}

func NewCatalog() *Catalog { return &Catalog{res: map[string]spi.ResolvedResource{}} }

func (c *Catalog) Add(r spi.ResolvedResource) {
	c.mu.Lock()
	c.res[r.Resource.ID] = r
	c.mu.Unlock()
}

// AddGuard activates a guard of a package version.
func (c *Catalog) AddGuard(g manifest.Guard, pkg, version string) {
	c.mu.Lock()
	c.guards = append(c.guards, spi.ResolvedGuard{Guard: g, Package: pkg, Version: version})
	c.mu.Unlock()
}

func (c *Catalog) Resolve(_ context.Context, id string) (spi.ResolvedResource, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.res[id]
	return r, ok, nil
}

func (c *Catalog) Entities(context.Context) ([]spi.ResolvedResource, error) { return nil, nil }

func (c *Catalog) Guards(_ context.Context, target string, phase manifest.GuardPhase) ([]spi.ResolvedGuard, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []spi.ResolvedGuard
	for _, g := range c.guards {
		if g.Guard.Target == target && g.Guard.Phase == phase {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Guard.ID < out[j].Guard.ID })
	return out, nil
}

// ---- subjects ----

// Subjects is a host.SubjectLoader whose answers tests change at will.
type Subjects struct {
	mu      sync.Mutex
	perms   map[int64][]string
	blocked map[int64]string
	loads   map[int64]int
}

func NewSubjects() *Subjects {
	return &Subjects{perms: map[int64][]string{}, blocked: map[int64]string{}, loads: map[int64]int{}}
}

// Grant sets the current permissions of a user.
func (s *Subjects) Grant(user int64, perms ...string) {
	s.mu.Lock()
	s.perms[user] = perms
	delete(s.blocked, user)
	s.mu.Unlock()
}

// Block makes the user unable to act, as a disabled account.
func (s *Subjects) Block(user int64, reason string) {
	s.mu.Lock()
	s.blocked[user] = reason
	s.mu.Unlock()
}

// Loads reports how often the user's rights were loaded.
func (s *Subjects) Loads(user int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loads[user]
}

func (s *Subjects) LoadSubject(_ context.Context, tenant uuid.UUID, user int64, kind spi.SubjectKind) (spi.Subject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loads[user]++
	if reason, ok := s.blocked[user]; ok {
		return spi.Subject{}, &host.SubjectBlockedError{TenantID: tenant, UserID: user, Reason: reason}
	}
	return spi.Subject{TenantID: tenant, UserID: user, Kind: kind, Permissions: append([]string{}, s.perms[user]...), Locale: "en"}, nil
}

// ---- data store double: one real table ----

// ItemStore serves the acme.test.item entity from host_items. Guard, when set,
// is consulted before every insert, as a real data store consults its
// spi.WriteGuard.
type ItemStore struct {
	DB    *pgxpool.Pool
	Guard spi.WriteGuard

	mu     sync.Mutex
	scopes []spi.Scope
}

func (s *ItemStore) seen(sc spi.Scope) {
	s.mu.Lock()
	s.scopes = append(s.scopes, sc)
	s.mu.Unlock()
}

// Callers lists the caller of each data call, in order.
func (s *ItemStore) Callers() []spi.Caller {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]spi.Caller, len(s.scopes))
	for i, sc := range s.scopes {
		out[i] = sc.Caller
	}
	return out
}

func (s *ItemStore) Get(ctx context.Context, tx pgx.Tx, sc spi.Scope, _, key string) (json.RawMessage, error) {
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

func (s *ItemStore) Find(ctx context.Context, tx pgx.Tx, sc spi.Scope, _ string, _ spi.FindQuery) (json.RawMessage, error) {
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

func (s *ItemStore) Count(ctx context.Context, tx pgx.Tx, sc spi.Scope, _ string, _ *manifest.Filter) (int64, error) {
	s.seen(sc)
	var n int64
	err := tx.QueryRow(ctx, `SELECT count(*) FROM host_items WHERE tenant_id = $1`, sc.Subject.TenantID).Scan(&n)
	return n, err
}

func (s *ItemStore) Insert(ctx context.Context, tx pgx.Tx, sc spi.Scope, entity string, values json.RawMessage) (json.RawMessage, error) {
	s.seen(sc)
	var v struct{ Label string }
	if err := json.Unmarshal(values, &v); err != nil {
		return nil, err
	}
	if s.Guard != nil {
		err := s.Guard.CheckWrite(ctx, tx, sc, spi.WriteChange{Entity: entity, Op: "insert", Proposed: values, Changed: []string{"label"}})
		if err != nil {
			return nil, err
		}
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO host_items (tenant_id, label) VALUES ($1, $2) RETURNING id`, sc.Subject.TenantID, v.Label).Scan(&id); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"key": strconv.FormatInt(id, 10), "label": v.Label})
}

func (s *ItemStore) Update(context.Context, pgx.Tx, spi.Scope, string, string, json.RawMessage) (json.RawMessage, error) {
	return nil, fmt.Errorf("ItemStore.Update is not needed by these tests")
}

func (s *ItemStore) Delete(context.Context, pgx.Tx, spi.Scope, string, string) error {
	return fmt.Errorf("ItemStore.Delete is not needed by these tests")
}

func (s *ItemStore) PutOverlay(context.Context, pgx.Tx, spi.Scope, string, string, json.RawMessage) (json.RawMessage, error) {
	return nil, fmt.Errorf("ItemStore.PutOverlay is not needed by these tests")
}
