package data_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/crm"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

const (
	pkgName = "@acme/insurance"
	ns      = "acme.insurance"
)

type env struct {
	t       *testing.T
	ctx     context.Context
	tx      pgx.Tx
	tenant  uuid.UUID
	catalog *fakeCatalog
	natives *data.NativeRegistry
	guard   *recordingGuard
	store   *data.Store
}

// fakeCatalog resolves the resources of applied manifests the way the active
// catalog does.
type fakeCatalog struct {
	res map[string]spi.ResolvedResource
}

func (c *fakeCatalog) Resolve(_ context.Context, id string) (spi.ResolvedResource, bool, error) {
	r, ok := c.res[id]
	return r, ok, nil
}

func (c *fakeCatalog) Entities(context.Context) ([]spi.ResolvedResource, error) {
	var out []spi.ResolvedResource
	for _, r := range c.res {
		out = append(out, r)
	}
	return out, nil
}

// activate makes version v of m the active one.
func (c *fakeCatalog) activate(pkg, version string, m *manifest.Manifest) {
	for _, r := range m.Resources {
		c.res[r.ID] = spi.ResolvedResource{Resource: r, Package: pkg, Version: version}
	}
}

type recordingGuard struct {
	calls []spi.WriteChange
	fail  func(spi.WriteChange) error
}

func (g *recordingGuard) CheckWrite(_ context.Context, _ pgx.Tx, _ spi.Scope, c spi.WriteChange) error {
	g.calls = append(g.calls, c)
	if g.fail != nil {
		return g.fail(c)
	}
	return nil
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := itf.Setup(t, itf.WithComponents(
		logging.NewComponent(),
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		crm.NewComponent(),
	))
	cat := &fakeCatalog{res: map[string]spi.ResolvedResource{}}
	nat := data.DefaultNatives()
	g := &recordingGuard{}
	return &env{
		t: t, ctx: e.Ctx, tx: e.Tx, tenant: e.Tenant.ID, catalog: cat, natives: nat, guard: g,
		store: data.NewStore(cat, nat, g),
	}
}

func (e *env) scope(version string, userID int64, perms ...string) spi.Scope {
	return spi.Scope{
		Subject: spi.Subject{TenantID: e.tenant, UserID: userID, Permissions: perms},
		Caller:  spi.Caller{Package: pkgName, Version: version},
	}
}

// install compiles, plans, applies and activates a manifest version.
func (e *env) install(version string, m *manifest.Manifest) data.MigrationPlan {
	e.t.Helper()
	plan, err := data.Prepare(e.ctx, e.tx, data.NewCompiler(e.natives, e.catalog), pkgName, version, m, true)
	require.NoError(e.t, err)
	e.catalog.activate(pkgName, version, m)
	return plan
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	_, err := e.tx.Exec(e.ctx, sql, args...)
	require.NoError(e.t, err)
}

func (e *env) scalar(sql string, args ...any) string {
	e.t.Helper()
	var s *string
	require.NoError(e.t, e.tx.QueryRow(e.ctx, sql, args...).Scan(&s))
	if s == nil {
		return ""
	}
	return *s
}

func (e *env) newUser(first string) int64 {
	e.t.Helper()
	var id int64
	require.NoError(e.t, e.tx.QueryRow(e.ctx,
		`INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		 VALUES ($1, 'user', $2, 'Tester', $3, 'en') RETURNING id`,
		e.tenant, first, first+"-"+uuid.NewString()+"@example.com").Scan(&id))
	return id
}

func (e *env) newClient(first string) int64 {
	e.t.Helper()
	var id int64
	require.NoError(e.t, e.tx.QueryRow(e.ctx,
		`INSERT INTO clients (tenant_id, first_name, last_name) VALUES ($1, $2, 'Doe') RETURNING id`,
		e.tenant, first).Scan(&id))
	return id
}

func js(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func decode(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m), string(raw))
	return m
}

func decodeList(t *testing.T, raw json.RawMessage) []map[string]any {
	t.Helper()
	var m []map[string]any
	require.NoError(t, json.Unmarshal(raw, &m), string(raw))
	return m
}

func ref(entity, key string) map[string]string {
	return map[string]string{"entity": entity, "key": key}
}

// attempt runs f in a savepoint and rolls it back, so a database error does
// not abort the test transaction.
func (e *env) attempt(f func(tx pgx.Tx) error) error {
	e.t.Helper()
	sp, err := e.tx.Begin(e.ctx)
	require.NoError(e.t, err)
	defer func() { _ = sp.Rollback(e.ctx) }()
	return f(sp)
}

func (e *env) mustInsert(sc spi.Scope, entity string, v any) map[string]any {
	e.t.Helper()
	out, err := e.store.Insert(e.ctx, e.tx, sc, entity, js(v))
	require.NoError(e.t, err)
	return decode(e.t, out)
}

func clone(t *testing.T, m *manifest.Manifest) *manifest.Manifest {
	t.Helper()
	var c manifest.Manifest
	require.NoError(t, json.Unmarshal(js(m), &c))
	return &c
}

func resource(m *manifest.Manifest, id string) *manifest.Resource {
	for i := range m.Resources {
		if m.Resources[i].ID == id {
			return &m.Resources[i]
		}
	}
	panic("no resource " + id)
}

func (c *fakeCatalog) Guards(context.Context, string, manifest.GuardPhase) ([]spi.ResolvedGuard, error) {
	return nil, nil
}
