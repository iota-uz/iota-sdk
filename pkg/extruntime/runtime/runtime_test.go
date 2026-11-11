package runtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage/managetest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool/pooltest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

const ns = "acme.insurance"

func obj(props ...manifest.Property) *manifest.Type {
	return &manifest.Type{Kind: manifest.KindObject, Properties: props}
}

func testManifest() manifest.Manifest {
	return manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: ns,
		Resources: []manifest.Resource{
			{ID: ns + ".policy", Kind: manifest.ResEntity, Fields: []manifest.Field{{ID: "number", Type: manifest.Type{Kind: manifest.KindString}, Unique: true}}},
			{ID: ns + ".renewPolicy", Kind: manifest.ResCommand, Input: obj(), Output: obj(manifest.Property{Name: "ok", Type: manifest.Type{Kind: manifest.KindBool}})},
		},
		Implementations: []manifest.Implementation{{Export: "renew", Implements: ns + ".renewPolicy", Kind: manifest.ImplCommand}},
		Subscriptions:   []manifest.Subscription{{ID: ns + ".onCreated", Export: "onCreated", Event: "core.user.created.v1"}},
	}
}

const serverSource = `export const renew = async (ctx) => ({ ok: ctx.user.userId !== undefined });
export const onCreated = async (tx, event) => { return null; };
`

func files(t *testing.T, version string) map[string]string {
	t.Helper()
	m, err := json.Marshal(testManifest())
	require.NoError(t, err)
	return map[string]string{
		"package.json":  fmt.Sprintf(`{"name":"@acme/ins","version":%q,"iota":{"namespace":%q}}`, version, ns),
		"manifest.json": string(m),
		"server.mjs":    serverSource,
		"contracts.mjs": "export {}\n",
	}
}

func put(t *testing.T, svc *manage.Service, actor spi.Subject, version string) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.GetPackage(ctx, actor, ns); err != nil {
		_, err = svc.CreatePackage(ctx, actor, ns, "")
		require.NoError(t, err)
	}
	for p, c := range files(t, version) {
		_, err := svc.PutFile(ctx, actor, ns, p, []byte(c), nil)
		require.NoError(t, err)
	}
}

func TestComponentComposesTheRuntimeAndRunsAPackageEndToEnd(t *testing.T) {
	natives, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	comp, err := runtime.NewComponent(runtime.Config{
		Natives: natives, WorkerScript: pooltest.BuildWorker(t), CacheDir: t.TempDir(), PoolSize: 1,
		Builder: &managetest.Builder{}, Kits: &managetest.Kits{}, DisableBackground: true,
	})
	require.NoError(t, err)
	f := itf.Setup(t, itf.WithComponents(append(modules.Components(), comp)...), itf.WithUser(itf.User()))
	rt := itf.GetService[runtime.Runtime](f)
	require.NoError(t, f.Tx.Commit(f.Ctx))
	t.Cleanup(func() { _ = rt.Close(context.Background()) })
	ctx := context.Background()

	var owner int64
	require.NoError(t, f.Pool.QueryRow(ctx, `INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		VALUES ($1, 'user', 'Own', 'Er', $2, 'en') RETURNING id`, f.TenantID(), "owner-"+uuid.NewString()+"@example.com").Scan(&owner))
	admin := spi.Subject{TenantID: f.TenantID(), UserID: owner, Kind: spi.SubjectInitiator,
		Permissions: []string{manage.PermRead, manage.PermAuthor, manage.PermReview, manage.PermPublish, manage.PermOperate}}

	// the native catalog includes the events Go modules declare
	r, ok, err := rt.Catalog.Resolve(ctx, "core.user.created.v1")
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, r.Native)

	put(t, rt.Manage, admin, "1.0.0")
	_, err = rt.Manage.Publish(ctx, admin, ns, "1.0.0")
	require.NoError(t, err)
	_, err = rt.Manage.Review(ctx, admin, ns, "1.0.0", true, "ok")
	require.NoError(t, err)
	_, err = rt.Manage.Activate(ctx, admin, ns, "1.0.0")
	require.NoError(t, err)
	_, err = rt.Manage.AssignOwner(ctx, admin, ns+".onCreated", owner)
	require.NoError(t, err)

	// the host runs the command through the binding management created, in
	// the artifact management stored
	res, err := rt.Host.Invoke(ctx, host.Request{Contract: ns + ".renewPolicy", Input: json.RawMessage(`{}`), Subject: admin, Trigger: host.Trigger{Kind: host.TriggerUI}})
	require.NoError(t, err)
	require.JSONEq(t, `{"ok":true}`, string(res.Output))
	require.Equal(t, ns, res.Binding.Package)
	require.Equal(t, "1.0.0", res.Binding.Version)

	// an event published after activation reaches the subscription as its owner
	tx, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = events.Publish(ctx, tx, spi.Scope{Subject: admin}, coreservices.UserCreatedEvent, &spi.ObjectRef{Entity: "core.user", Key: "5"},
		coreservices.UserCreatedPayload{UserID: 5, Email: "a@b.c", FirstName: "A", LastName: "B", UserType: "user"})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.Eventually(t, func() bool {
		_, err := rt.Dispatcher.RunOnce(ctx)
		require.NoError(t, err)
		var done int
		require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM extruntime.subscription_receipts WHERE subscription = $1 AND status = 'done'`, ns+".onCreated").Scan(&done))
		return done == 1
	}, 10*time.Second, 100*time.Millisecond)

	// workflow management is wired to the engine
	list, err := rt.Manage.ListWorkflows(ctx, admin, workflowFilter())
	require.NoError(t, err)
	require.Empty(t, list)

	// the notification adapter follows the activations
	require.NoError(t, rt.Notify.Sync(ctx))

	// the container's hook started the runtime; Close stops it
	require.NoError(t, rt.Close(ctx))
}

// ---- HTTP API ----

type apiEnv struct {
	t       *testing.T
	srv     *httptest.Server
	tenant  uuid.UUID
	svc     *manage.Service
	db      *pgxpool.Pool
	cat     *catalog.Catalog
	natives *catalog.NativeSet
	arts    *artifact.Store
}

func newAPIEnv(t *testing.T) *apiEnv {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_runtime_api",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase, Pool: itf.PoolConfig{MaxConns: 6}},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	arts, err := artifact.NewStore(scope.Pool, t.TempDir())
	require.NoError(t, err)
	natives := catalog.NewNativeSet(data.DefaultNatives())
	cat := catalog.New(scope.Pool, natives)
	svc, err := manage.New(manage.Config{DB: scope.Pool, Builder: &managetest.Builder{}, Kits: &managetest.Kits{}, Artifacts: arts, Catalog: cat, Natives: data.DefaultNatives()})
	require.NoError(t, err)
	e := &apiEnv{t: t, tenant: scope.Tenant.ID, svc: svc, db: scope.Pool, cat: cat, natives: natives, arts: arts}
	subject := func(r *http.Request) (spi.Subject, error) {
		id, _ := strconv.ParseInt(r.Header.Get("X-User"), 10, 64)
		perms := map[int64][]string{
			1: {manage.PermRead, manage.PermAuthor, manage.PermReview, manage.PermPublish, manage.PermOperate},
			2: {manage.PermRead},
		}[id]
		if id == 0 {
			return spi.Subject{}, fmt.Errorf("no user")
		}
		return spi.Subject{TenantID: e.tenant, UserID: id, Kind: spi.SubjectInitiator, Permissions: perms}, nil
	}
	e.srv = httptest.NewServer(runtime.NewAPI(svc, subject, nil).Handler())
	t.Cleanup(e.srv.Close)
	return e
}

func (e *apiEnv) do(user int, method, path string, body any, headers ...string) (int, []byte, http.Header) {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		rd = bytes.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		require.NoError(e.t, err)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, e.srv.URL+runtime.APIPrefix+path, rd)
	require.NoError(e.t, err)
	if user != 0 {
		req.Header.Set("X-User", strconv.Itoa(user))
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(e.t, err)
	out, err := io.ReadAll(resp.Body)
	require.NoError(e.t, err)
	require.NoError(e.t, resp.Body.Close())
	return resp.StatusCode, out, resp.Header
}

func (e *apiEnv) json(user int, method, path string, body any, want int, into any) {
	e.t.Helper()
	status, out, _ := e.do(user, method, path, body)
	require.Equal(e.t, want, status, string(out))
	if into != nil {
		require.NoError(e.t, json.Unmarshal(out, into), string(out))
	}
}

func TestManagementAPIMirrorsTheServiceWithItsRightsAndErrors(t *testing.T) {
	e := newAPIEnv(t)
	const admin, reader = 1, 2

	// no user -> unauthenticated-style failure from the subject function
	status, _, _ := e.do(0, http.MethodGet, "/packages", nil)
	require.GreaterOrEqual(t, status, 400)

	// rights come from the service
	var errBody struct {
		Error struct{ Code, Message string }
	}
	e.json(reader, http.MethodPost, "/packages", map[string]string{"namespace": ns}, http.StatusForbidden, &errBody)
	require.Equal(t, "permissiondenied", strings.ReplaceAll(errBody.Error.Code, "_", ""))
	require.Contains(t, errBody.Error.Message, "Extensions.Author")
	e.json(admin, http.MethodPost, "/packages", map[string]string{"namespace": ns}, http.StatusCreated, nil)
	e.json(admin, http.MethodPost, "/packages", map[string]string{"namespace": ns}, http.StatusConflict, nil)
	e.json(admin, http.MethodPost, "/packages", map[string]any{"namespace": ns, "bogus": 1}, http.StatusBadRequest, nil)
	e.json(admin, http.MethodGet, "/packages/acme.none", nil, http.StatusNotFound, nil)

	// files: raw bodies with optimistic concurrency
	for p, c := range files(t, "1.0.0") {
		status, out, hdr := e.do(admin, http.MethodPut, "/packages/"+ns+"/files/"+p, []byte(c))
		require.Equal(t, http.StatusOK, status, string(out))
		_ = hdr
	}
	status, out, hdr := e.do(reader, http.MethodGet, "/packages/"+ns+"/files/server.mjs", nil)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, serverSource, string(out))
	etag := strings.Trim(hdr.Get("ETag"), `"`)
	require.Len(t, etag, 64)
	status, _, _ = e.do(admin, http.MethodPut, "/packages/"+ns+"/files/server.mjs", []byte("x"), "If-Match", `"stale"`)
	require.Equal(t, http.StatusConflict, status)
	status, _, _ = e.do(admin, http.MethodPut, "/packages/"+ns+"/files/server.mjs", []byte(serverSource), "If-Match", `"`+etag+`"`)
	require.Equal(t, http.StatusOK, status)
	var list []manage.File
	e.json(reader, http.MethodGet, "/packages/"+ns+"/files", nil, http.StatusOK, &list)
	require.Len(t, list, 6)
	e.json(admin, http.MethodPost, "/packages/"+ns+"/move", map[string]string{"from": "src/server.ts", "to": "src/s.ts"}, http.StatusNoContent, nil)
	status, _, _ = e.do(admin, http.MethodDelete, "/packages/"+ns+"/files/src/s.ts", nil)
	require.Equal(t, http.StatusNoContent, status)

	// a failing build is reported with its diagnostics; publish is refused with them
	status, _, _ = e.do(admin, http.MethodPut, "/packages/"+ns+"/files/src/bad.ts", []byte("BAD_TYPE"))
	require.Equal(t, http.StatusOK, status)
	var rep manage.BuildReport
	e.json(admin, http.MethodPost, "/packages/"+ns+"/build", nil, http.StatusOK, &rep)
	require.True(t, rep.HasErrors)
	var refused struct {
		Error struct {
			Code        string
			Diagnostics []struct{ File, Code string }
		}
	}
	e.json(admin, http.MethodPost, "/packages/"+ns+"/publish", map[string]string{"version": "1.0.0"}, http.StatusConflict, &refused)
	require.Equal(t, "ts.2345", refused.Error.Diagnostics[0].Code)
	require.Equal(t, "src/bad.ts", refused.Error.Diagnostics[0].File)
	e.json(admin, http.MethodDelete, "/packages/"+ns+"/files/src/bad.ts", nil, http.StatusNoContent, nil)

	var chk manage.CheckReport
	e.json(admin, http.MethodPost, "/packages/"+ns+"/check", nil, http.StatusOK, &chk)
	require.False(t, chk.HasErrors)

	// publish -> review -> activate
	var v manage.VersionDetail
	e.json(reader, http.MethodPost, "/packages/"+ns+"/publish", nil, http.StatusForbidden, nil)
	e.json(admin, http.MethodPost, "/packages/"+ns+"/publish", nil, http.StatusCreated, &v)
	require.Equal(t, "1.0.0", v.Version.Version)
	e.json(admin, http.MethodPost, "/packages/"+ns+"/versions/1.0.0/activate", nil, http.StatusConflict, nil) // not approved
	e.json(admin, http.MethodPost, "/packages/"+ns+"/versions/1.0.0/review", map[string]any{"approve": true, "expectSourceHash": "wrong"}, http.StatusConflict, nil)
	var rv manage.Review
	e.json(admin, http.MethodPost, "/packages/"+ns+"/versions/1.0.0/review", map[string]any{"approve": true, "comment": "ok", "expectSourceHash": v.SourceHash, "expectArtifactHash": v.ArtifactHash}, http.StatusCreated, &rv)
	var act manage.ActivationResult
	e.json(admin, http.MethodPost, "/packages/"+ns+"/versions/1.0.0/activate", nil, http.StatusOK, &act)
	require.Len(t, act.Bound, 1)
	var pk manage.Package
	e.json(reader, http.MethodGet, "/packages/"+ns, nil, http.StatusOK, &pk)
	require.Equal(t, "1.0.0", pk.ActiveVersion)
	var vs []manage.Version
	e.json(reader, http.MethodGet, "/packages/"+ns+"/versions", nil, http.StatusOK, &vs)
	require.True(t, vs[0].Active && vs[0].Approved)
	var acts []manage.Activation
	e.json(reader, http.MethodGet, "/packages/"+ns+"/activations", nil, http.StatusOK, &acts)
	require.Len(t, acts, 1)
	var binds []manage.Binding
	e.json(reader, http.MethodGet, "/bindings", nil, http.StatusOK, &binds)
	require.Len(t, binds, 1)
	require.Equal(t, ns+".renewPolicy", binds[0].TargetID)

	// export / import round trip with the same tree identity
	status, tarball, hdr := e.do(reader, http.MethodGet, "/packages/"+ns+"/export?format=tar&version=1.0.0", nil)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, hdr.Get("Content-Disposition"), ".tar")
	treeHash := hdr.Get("X-Tree-Hash")
	require.NotEmpty(t, treeHash)
	status, out, _ = e.do(admin, http.MethodPost, "/import/tree", tarball)
	require.Equal(t, http.StatusOK, status, string(out))
	status, _, hdr = e.do(reader, http.MethodGet, "/packages/"+ns+"/export?format=zip", nil)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, treeHash, hdr.Get("X-Tree-Hash"))
	status, _, _ = e.do(admin, http.MethodPost, "/import/tree", []byte("not an archive"))
	require.Equal(t, http.StatusBadRequest, status)
	e.json(admin, http.MethodPost, "/import/git", map[string]string{"repoUrl": "/tmp/x.git", "commit": strings.Repeat("a", 40)}, http.StatusBadRequest, nil)

	// kit
	var kit struct {
		Hash    string
		Modules []string
	}
	e.json(reader, http.MethodGet, "/kit", nil, http.StatusOK, &kit)
	require.Contains(t, kit.Modules, "@iota-kit/"+ns)
	status, tgz, hdr := e.do(reader, http.MethodGet, "/kit/modules/@iota-kit/core.tgz", nil)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "application/gzip", hdr.Get("Content-Type"))
	require.Greater(t, len(tgz), 100)

	// triggers
	owner := e.svc
	_ = owner
	e.json(admin, http.MethodPut, "/triggers/"+ns+".onCreated/owner", map[string]int64{"userId": 999999}, http.StatusBadRequest, nil)

	// workflows need the engine, which this transport test does not attach
	e.json(reader, http.MethodGet, "/workflows", nil, http.StatusConflict, nil)
	e.json(admin, http.MethodGet, "/workflows/not-a-uuid", nil, http.StatusBadRequest, nil)

	// deactivate / purge and the audit trail
	e.json(reader, http.MethodPost, "/packages/"+ns+"/deactivate", nil, http.StatusForbidden, nil)
	e.json(admin, http.MethodPost, "/packages/"+ns+"/deactivate", nil, http.StatusNoContent, nil)
	e.json(admin, http.MethodPost, "/packages/"+ns+"/purge", nil, http.StatusNoContent, nil)
	var audit []manage.AuditEntry
	e.json(reader, http.MethodGet, "/audit?namespace="+ns+"&action=publish", nil, http.StatusOK, &audit)
	outcomes := make([]string, 0, len(audit))
	for _, a := range audit {
		outcomes = append(outcomes, a.Outcome)
	}
	require.ElementsMatch(t, []string{"ok", "denied", "failed"}, outcomes)
	e.json(reader, http.MethodGet, "/audit?limit=x", nil, http.StatusBadRequest, nil)
}

func workflowFilter() workflow.ListFilter { return workflow.ListFilter{} }

func TestUISlicePicksUpActivationsBindingsColumnsAndAssets(t *testing.T) {
	e := newAPIEnv(t)
	ctx := context.Background()
	admin := spi.Subject{TenantID: e.tenant, UserID: 1, Kind: spi.SubjectInitiator,
		Permissions: []string{manage.PermRead, manage.PermAuthor, manage.PermReview, manage.PermPublish}}
	require.NoError(t, e.natives.Set().Add(native.Route("crm.clients", "/crm/clients")))

	m := manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: ns,
		Resources: []manifest.Resource{
			{ID: ns + ".clientExtra", Kind: manifest.ResOverlay, Base: "crm.client", Fields: []manifest.Field{{ID: "tier", Type: manifest.Type{Kind: manifest.KindString, Optional: true, Nullable: true}}}},
			{ID: ns + ".tierColumn", Kind: manifest.ResColumn, List: "crm.lists.clients", Overlay: ns + ".clientExtra", Field: "tier", Filterable: true, Sortable: true, Exportable: true,
				Label: manifest.Label{"en": "Tier"}},
			{ID: ns + ".clients", Kind: manifest.ResPage, Path: "/insurance/clients", Chunk: "client/clients.js", Export: "clients"},
			{ID: ns + ".shell", Kind: manifest.ResShell, Chunk: "client/shell.js", Export: "shell"},
		},
	}
	mj, err := json.Marshal(m)
	require.NoError(t, err)
	_, err = e.svc.CreatePackage(ctx, admin, ns, "")
	require.NoError(t, err)
	for p, c := range map[string]string{
		"package.json":           fmt.Sprintf(`{"name":"@acme/ins","version":"1.0.0","iota":{"namespace":%q}}`, ns),
		"manifest.json":          string(mj),
		"server.mjs":             "export {}\n",
		"dist/client/clients.js": "export const clients = 1\n",
		"dist/client/shell.js":   "export const shell = 1\n",
	} {
		_, err := e.svc.PutFile(ctx, admin, ns, p, []byte(c), nil)
		require.NoError(t, err)
	}
	_, err = e.svc.Publish(ctx, admin, ns, "1.0.0")
	require.NoError(t, err)
	_, err = e.svc.Review(ctx, admin, ns, "1.0.0", true, "ok")
	require.NoError(t, err)
	_, err = e.svc.Activate(ctx, admin, ns, "1.0.0")
	require.NoError(t, err)

	// a screen binding targets a native route, the shell any id
	_, err = e.svc.Bind(ctx, admin, manage.SlotScreen, "crm.clients", ns, "1.0.0", "clients")
	require.NoError(t, err)
	_, err = e.svc.Bind(ctx, admin, manage.SlotShell, "app", ns, "1.0.0", "shell")
	require.NoError(t, err)
	_, err = e.svc.Bind(ctx, admin, manage.SlotScreen, "crm.nothing", ns, "1.0.0", "clients")
	require.Error(t, err, "an unknown screen target is refused")

	src := ui.NewPGSource(e.db, runtime.UIManifests{Catalog: e.cat}, 0)
	snap, err := src.Snapshot(ctx)
	require.NoError(t, err)
	require.Len(t, snap.Packages, 1)
	pk := snap.Packages[0]
	require.Equal(t, []string{ns, "1.0.0"}, []string{pk.Package, pk.Version})
	require.NotEmpty(t, pk.ArtifactHash)
	require.Len(t, snap.Bindings, 2)
	kinds := map[string]string{}
	for _, b := range snap.Bindings {
		kinds[b.SlotKind] = b.TargetID + "=" + b.Export
	}
	require.Equal(t, map[string]string{"screen": "crm.clients=clients", "shell": "app=shell"}, kinds)
	rev := snap.Revision

	// activated column resources reach the list column source
	cfg := ui.Config{
		Source: src, Artifacts: runtime.UIArtifacts{Store: e.arts}, Catalog: e.cat,
		Host: clienthost.Manifest{Package: "@iota-uz/sdk", PackageVersion: "0.5.6", SDKCommit: "0123456789abcdef0123456789abcdef01234567",
			SDKReleaseVersion: sdkidentity.ReleaseVersion, ProtocolVersion: clienthost.ProtocolVersion, Entry: "/assets/runtime-host.js"},
		NativeRoutes: ui.NativeRoutes{"crm.clients": "/crm/clients"},
	}
	uirt, err := ui.NewRuntime(cfg)
	require.NoError(t, err)
	cols, err := uirt.Columns().Columns(ctx, "crm.lists.clients")
	require.NoError(t, err)
	require.Len(t, cols, 1)
	require.Equal(t, []string{ns + ".tierColumn", ns, "1.0.0"}, []string{cols[0].ID, cols[0].Package, cols[0].Version})

	// assets are served from the immutable artifact by hash
	arts := runtime.UIArtifacts{Store: e.arts}
	f, err := arts.Open(ctx, pk.ArtifactHash, "client/clients.js")
	require.NoError(t, err)
	b, _ := io.ReadAll(f)
	_ = f.Close()
	require.Equal(t, "export const clients = 1\n", string(b))
	// only the client subtree of an artifact is the browser's: the server
	// bundle, manifests and sources of the same artifact are not
	for _, bad := range []string{"client/missing.js", "../../etc/passwd", "/etc/passwd", "server.mjs", "manifest.json", "package.json", "contracts.mjs"} {
		_, err = arts.Open(ctx, pk.ArtifactHash, bad)
		require.ErrorIs(t, err, ui.ErrArtifactNotFound, bad)
	}
	_, err = arts.Open(ctx, "0000000000000000", "client/clients.js")
	require.ErrorIs(t, err, ui.ErrArtifactNotFound)

	// deactivation takes pages, columns and bindings away
	require.NoError(t, e.svc.Deactivate(ctx, admin, ns))
	snap, err = src.Snapshot(ctx)
	require.NoError(t, err)
	require.Empty(t, snap.Packages)
	require.Empty(t, snap.Bindings)
	require.NotEqual(t, rev, snap.Revision)
	cols, _ = uirt.Columns().Columns(ctx, "crm.lists.clients")
	require.Empty(t, cols)
}
