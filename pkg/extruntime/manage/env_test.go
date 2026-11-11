package manage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage/managetest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The migrator reads ./migrations relative to the working directory.
func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

const ns = "acme.insurance"

type env struct {
	t       *testing.T
	db      *pgxpool.Pool
	tenant  uuid.UUID
	svc     *manage.Service
	cat     *catalog.Catalog
	natives *catalog.NativeSet
	builder *managetest.Builder
	kits    *managetest.Kits
	arts    *artifact.Store
	cache   string
	// subjects
	admin, author, reviewer, reader spi.Subject
}

func newEnv(t *testing.T, opts ...func(*manage.Config)) *env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_manage",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase, Pool: itf.PoolConfig{MaxConns: 6}},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	e := &env{t: t, db: scope.Pool, tenant: scope.Tenant.ID, builder: &managetest.Builder{}, kits: &managetest.Kits{}, cache: t.TempDir()}
	var err error
	e.arts, err = artifact.NewStore(e.db, e.cache)
	require.NoError(t, err)
	e.natives = catalog.NewNativeSet(data.DefaultNatives())
	require.NoError(t, e.natives.Register(manifest.Resource{ID: "core.commands.RejectIncoming", Kind: manifest.ResCommand,
		Input: &manifest.Type{Kind: manifest.KindObject}, Output: &manifest.Type{Kind: manifest.KindObject}}))
	require.NoError(t, e.natives.Register(manifest.Resource{ID: "core.events.UserCreated", Kind: manifest.ResEvent,
		Payload: &manifest.Type{Kind: manifest.KindObject}}))
	e.cat = catalog.New(e.db, e.natives)
	cfg := manage.Config{DB: e.db, Builder: e.builder, Kits: e.kits, Artifacts: e.arts, Catalog: e.cat, Natives: data.DefaultNatives()}
	for _, o := range opts {
		o(&cfg)
	}
	e.svc, err = manage.New(cfg)
	require.NoError(t, err)

	sub := func(id int64, perms ...string) spi.Subject {
		return spi.Subject{TenantID: e.tenant, UserID: id, Kind: spi.SubjectInitiator, Permissions: perms, Locale: "en"}
	}
	e.admin = sub(1, manage.PermRead, manage.PermAuthor, manage.PermReview, manage.PermPublish, manage.PermOperate)
	e.author = sub(2, manage.PermRead, manage.PermAuthor)
	e.reviewer = sub(3, manage.PermRead, manage.PermReview)
	e.reader = sub(4, manage.PermRead)
	return e
}

func (e *env) newUser(name string) int64 {
	e.t.Helper()
	var id int64
	require.NoError(e.t, e.db.QueryRow(context.Background(),
		`INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		 VALUES ($1, 'user', $2, 'Tester', $3, 'en') RETURNING id`,
		e.tenant, name, name+"-"+uuid.NewString()+"@example.com").Scan(&id))
	return id
}

func (e *env) scalar(sql string, args ...any) string {
	e.t.Helper()
	var s *string
	require.NoError(e.t, e.db.QueryRow(context.Background(), sql, args...).Scan(&s))
	if s == nil {
		return ""
	}
	return *s
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.db.QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

// ---- package fixtures ----

func pkgJSON(namespace, version string) string {
	return fmt.Sprintf(`{"name":"@acme/%s","version":%q,"type":"module","iota":{"namespace":%q,"contracts":"./contracts.mjs","server":"./server.mjs"}}`, namespace, version, namespace)
}

func tm(kind manifest.TypeKind) manifest.Type { return manifest.Type{Kind: kind} }

func opt(t manifest.Type) manifest.Type { t.Optional, t.Nullable = true, true; return t }

func emptyObj() *manifest.Type { return &manifest.Type{Kind: manifest.KindObject} }

// insuranceManifest is the reference package: one entity, an ordinary and an
// atomic command with their implementations.
func insuranceManifest(numberType manifest.Type, extra ...manifest.Field) manifest.Manifest {
	fields := []manifest.Field{{ID: "number", Type: numberType, Unique: true}}
	fields = append(fields, extra...)
	return manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: ns,
		Resources: []manifest.Resource{
			{ID: ns + ".policy", Kind: manifest.ResEntity, Fields: fields},
			{ID: ns + ".renewPolicy", Kind: manifest.ResCommand, Input: emptyObj(), Output: emptyObj()},
			{ID: ns + ".activatePolicy", Kind: manifest.ResCommand, Atomic: true, Input: emptyObj(), Output: emptyObj()},
			{ID: ns + ".policies", Kind: manifest.ResPage, Path: "/insurance/policies", Export: "policies"},
		},
		Implementations: []manifest.Implementation{
			{Export: "renew", Implements: ns + ".renewPolicy", Kind: manifest.ImplCommand},
			{Export: "activate", Implements: ns + ".activatePolicy", Kind: manifest.ImplAtomic},
		},
		Subscriptions: []manifest.Subscription{{ID: ns + ".onRenewed", Export: "onRenewed", Event: "core.events.UserCreated"}},
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.MarshalIndent(v, "", " ")
	require.NoError(t, err)
	return b
}

// draft writes a complete tree into the package draft through PutFile.
func (e *env) draft(namespace, version string, m manifest.Manifest, extra map[string]string) {
	e.t.Helper()
	ctx := context.Background()
	if _, err := e.svc.GetPackage(ctx, e.admin, namespace); err != nil {
		_, err = e.svc.CreatePackage(ctx, e.author, namespace, "")
		require.NoError(e.t, err)
	}
	files := map[string][]byte{
		"package.json":  []byte(pkgJSON(namespace, version)),
		"manifest.json": mustJSON(e.t, m),
		"server.mjs":    []byte("export const renew = async () => ({}); // HANDLER_SECRET " + version + "\n"),
		"contracts.mjs": []byte("export const policy = { id: '" + namespace + ".policy' };\n"),
	}
	for p, c := range extra {
		files[p] = []byte(c)
	}
	for p, c := range files {
		_, err := e.svc.PutFile(ctx, e.author, namespace, p, c, nil)
		require.NoError(e.t, err)
	}
}

// release publishes, self-reviews and activates the draft.
func (e *env) release(namespace, version string) manage.ActivationResult {
	e.t.Helper()
	ctx := context.Background()
	_, err := e.svc.Publish(ctx, e.admin, namespace, version)
	require.NoError(e.t, err)
	_, err = e.svc.Review(ctx, e.admin, namespace, version, true, "lgtm")
	require.NoError(e.t, err)
	res, err := e.svc.Activate(ctx, e.admin, namespace, version)
	require.NoError(e.t, err)
	return res
}

func (e *env) openBindings() map[string]string {
	e.t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT slot_kind || ':' || target_id, package || '@' || version || '/' || export
		FROM extruntime.bindings WHERE active_to IS NULL`)
	require.NoError(e.t, err)
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		require.NoError(e.t, rows.Scan(&k, &v))
		out[k] = v
	}
	require.NoError(e.t, rows.Err())
	return out
}

func (e *env) auditActions(outcome string) map[string]int {
	e.t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT action, count(*) FROM extruntime.audit_log WHERE outcome = $1 GROUP BY action`, outcome)
	require.NoError(e.t, err)
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var a string
		var n int
		require.NoError(e.t, rows.Scan(&a, &n))
		out[a] = n
	}
	require.NoError(e.t, rows.Err())
	return out
}

func uuidOther() uuid.UUID { return uuid.MustParse("00000000-0000-0000-0000-0000000000aa") }

func spiAnon() spi.Subject { return spi.Subject{UserID: 99} }
