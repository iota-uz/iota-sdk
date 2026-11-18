package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// repoRoot is the working directory of the tests: migrations, locales and the
// built @iota-uz/sdk package (web/sdk) resolve relative to it.
var repoRoot string

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	repoRoot = wd
	os.Exit(m.Run())
}

const smokeNamespace = "smoke.greeter"

// smokeApp is the server composition of cmd/server (serverComponents) on a
// real PostgreSQL database, driven through its HTTP controllers behind the
// production middleware and session stack.
type smokeApp struct {
	t      *testing.T
	scope  *itf.Scope
	srv    *httptest.Server
	admin  *smokeActor
	tenant uuid.UUID
}

type smokeActor struct{ token string }

// newSmokeApp composes the server's components with the configuration the
// operator would set in the environment. extra overrides or adds EXTRUNTIME_*
// keys (in their koanf form).
func newSmokeApp(t *testing.T, extruntime map[string]any) *smokeApp {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	values := map[string]any{}
	for k, v := range extruntime {
		values[k] = v
	}
	// ".env" files and the process environment give the database; the
	// extension runtime keys are what this test is about.
	envSrc := envprov.New(".env", ".env.local", ".env.testing")
	src, err := config.Build(envSrc, static.New(values))
	require.NoError(t, err)

	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:       "server_extruntime_smoke",
		Components: serverComponents(),
		Source:     src,
		Database:   itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase, Pool: itf.PoolConfig{MaxConns: 24}},
		Isolation:  itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	a := &smokeApp{t: t, scope: h.Scope(t)}
	a.tenant = a.scope.Tenant.ID
	a.serve()
	return a
}

// smokeSection is the extension runtime configuration of a deployment that
// runs the workers of the built @iota-uz/sdk package (pnpm --filter
// @iota-uz/sdk build) shipped next to the binary.
func smokeSection(t *testing.T) map[string]any {
	t.Helper()
	distDir := filepath.Join(repoRoot, "web", "sdk")
	dist, err := runtime.ResolveDist(runtime.DistConfig{Dir: distDir})
	if err != nil {
		t.Skipf("the @iota-uz/sdk package is not built (run `pnpm install && pnpm --filter @iota-uz/sdk build`): %v", err)
	}
	manifest, err := json.Marshal(clienthost.Manifest{
		Package: runtime.SDKPackage, PackageVersion: dist.Identity.ReleaseVersion, SDKCommit: dist.Identity.SourceCommit,
		SDKReleaseVersion: dist.Identity.ReleaseVersion, ProtocolVersion: dist.Identity.ProtocolVersion, Entry: "/assets/runtime-host.js",
	})
	require.NoError(t, err)
	manifestPath := filepath.Join(t.TempDir(), "client-host.json")
	require.NoError(t, os.WriteFile(manifestPath, manifest, 0o600))
	return map[string]any{
		"extruntime.enabled":              true,
		"extruntime.poolsize":             1,
		"extruntime.distdir":              distDir,
		"extruntime.cachedir":             t.TempDir(),
		"extruntime.ui.hostmanifest":      manifestPath,
		"extruntime.configurator.enabled": true,
	}
}

func mustResolve[T any](t *testing.T, c *composition.Container) T {
	t.Helper()
	v, err := composition.Resolve[T](c)
	require.NoError(t, err)
	return v
}

// newAdmin creates a signed-in user holding the extension rights.
func (a *smokeApp) newAdmin() *smokeActor {
	t := a.t
	ctx := composables.WithParams(a.scope.Ctx, itf.DefaultParams())
	ctx = itf.TestExecution(composables.WithSession(ctx, itf.MockSession()))
	perms := []permission.Permission{
		corePerms.ExtensionsRead, corePerms.ExtensionsAuthor, corePerms.ExtensionsReview, corePerms.ExtensionsPublish, corePerms.ExtensionsOperate,
	}
	permRepo := persistence.NewPermissionRepository()
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	addr := fmt.Sprintf("admin-%s@example.com", uuid.NewString()[:8])
	email, err := internet.NewEmail(addr)
	require.NoError(t, err)
	var created user.User
	require.NoError(t, composables.InTx(ctx, func(ctx context.Context) error {
		for _, p := range perms {
			if err := permRepo.Save(ctx, p); err != nil {
				return err
			}
		}
		var err error
		created, err = userRepo.Create(ctx, user.New("Admin", "Tester", email, user.UILanguageEN,
			user.WithTenantID(a.tenant), user.WithPermissions(perms)))
		return err
	}))
	sess, err := mustResolve[*services.AuthService](t, a.scope.Container).CreateSession(ctx, created)
	require.NoError(t, err)
	return &smokeActor{token: sess.Token()}
}

// serve mounts the application's controllers behind the production
// middleware stack and the real session middleware.
func (a *smokeApp) serve() {
	t := a.t
	app, container := a.scope.App, a.scope.Container
	headersCfg := mustResolve[*headers.Config](t, container)
	reqLog := logrus.New()
	reqLog.SetLevel(logrus.WarnLevel)
	container.AppendMiddleware(
		middleware.WithLogger(reqLog, middleware.DefaultLoggerOptions(), headersCfg),
		middleware.Provide(constants.AppKey, app),
		middleware.Provide(constants.ContainerKey, container),
		middleware.Provide(constants.HeadKey, layouts.DefaultHead()),
		middleware.Provide(constants.LogoKey, assets.DefaultLogo()),
		middleware.Provide(constants.PoolKey, a.scope.Pool),
		middleware.Provide(constants.TenantIDKey, a.tenant),
		middleware.ProvideLocalizer(app.Bundle(), app.GetSupportedLanguages()),
		middleware.RequestParams(headersCfg),
	)
	r := mux.NewRouter()
	mws := app.Middleware()
	r.Use(mws...)
	for _, c := range app.Controllers() {
		mountController(t, r, c)
	}
	notFound := http.NotFoundHandler()
	for i := len(mws) - 1; i >= 0; i-- {
		notFound = mws[i](notFound)
	}
	r.NotFoundHandler = notFound
	a.srv = httptest.NewServer(r)
	t.Cleanup(a.srv.Close)
	a.admin = a.newAdmin()
}

// mountController registers a controller; one that needs build products this
// test does not have (the Lens Solid dist of the dashboard) is skipped.
func mountController(t *testing.T, r *mux.Router, c application.Controller) {
	t.Helper()
	defer func() {
		if rec := recover(); rec != nil {
			t.Logf("controller %s not mounted: %v", c.Descriptor().ID, rec)
		}
	}()
	c.Register(r)
}

type response struct {
	status int
	body   []byte
}

var noRedirects = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Timeout:       2 * time.Minute,
}

func (a *smokeApp) do(actor *smokeActor, method, path string, body []byte, headers ...string) response {
	t := a.t
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, a.srv.URL+path, bytes.NewReader(body))
	require.NoError(t, err)
	if actor != nil {
		req.AddCookie(&http.Cookie{Name: "sid", Value: actor.token})
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := noRedirects.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	out, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return response{status: res.StatusCode, body: out}
}

// manage calls the management API as the admin, expects the status and
// decodes a JSON answer into out.
func (a *smokeApp) manage(method, path string, in any, want int, out any) {
	a.t.Helper()
	var body []byte
	switch v := in.(type) {
	case nil:
	case []byte:
		body = v
	default:
		raw, err := json.Marshal(in)
		require.NoError(a.t, err)
		body = raw
	}
	r := a.do(a.admin, method, runtime.APIPrefix+path, body)
	require.Equal(a.t, want, r.status, "%s %s: %s", method, path, r.body)
	if out != nil {
		require.NoError(a.t, json.Unmarshal(r.body, out), string(r.body))
	}
}

// smokePackage reads the fixture package tree.
func smokePackage(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("cmd", "server", "testdata", "smoke")
	tree := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		tree[filepath.ToSlash(rel)] = string(b)
		return err
	}))
	require.NotEmpty(t, tree)
	return tree
}

// TestServerComposition_ExtensionRuntimeSmoke enables the extension runtime
// through configuration only, on the component set of the real server, and
// takes a package the whole way: draft, check, build in the Node build
// worker of the installed @iota-uz/sdk, publish, review, activation, then a
// command call through /_ext/rpc that runs in the Node host worker and
// writes to the database.
func TestServerComposition_ExtensionRuntimeSmoke(t *testing.T) {
	a := newSmokeApp(t, smokeSection(t))

	// The composition provides the runtime and the dispatcher Go code of the
	// application calls native contracts through.
	rt := mustResolve[*runtime.Runtime](t, a.scope.Container)
	require.NotNil(t, rt.Manage)
	_ = mustResolve[native.Dispatcher](t, a.scope.Container)

	// The management API is mounted and the native catalog is the SDK
	// modules' (modules.Natives()).
	var natives struct {
		Sources []struct {
			Resources []struct {
				ID string `json:"id"`
			} `json:"resources"`
		} `json:"sources"`
	}
	a.manage(http.MethodGet, "/catalog/natives", nil, http.StatusOK, &natives)
	ids := map[string]bool{}
	for _, src := range natives.Sources {
		for _, r := range src.Resources {
			ids[r.ID] = true
		}
	}
	require.True(t, ids["core.user"] && ids["crm.client"], "the SDK modules' natives are in the catalog: %v", ids)

	// The configurator the section enabled answers its type endpoint.
	require.Equal(t, http.StatusOK, a.do(a.admin, http.MethodGet, "/extensions/types/kit", nil).status)

	// Nothing of the package exists before it is activated.
	call := func(name string) (int, struct {
		OK    bool            `json:"ok"`
		Value json.RawMessage `json:"value"`
	}) {
		raw, err := json.Marshal(map[string]any{"kind": "command", "resource": smokeNamespace + ".greet", "input": map[string]any{"name": name}})
		require.NoError(t, err)
		r := a.do(a.admin, http.MethodPost, "/_ext/rpc", raw, "Content-Type", "application/json")
		var out struct {
			OK    bool            `json:"ok"`
			Value json.RawMessage `json:"value"`
		}
		_ = json.Unmarshal(r.body, &out)
		return r.status, out
	}
	status, _ := call("Ada")
	require.Equal(t, http.StatusNotFound, status, "the command is unknown before activation")

	// Draft files, then check, build, publish, self-review and activation.
	a.manage(http.MethodPost, "/packages", map[string]string{"namespace": smokeNamespace}, http.StatusCreated, nil)
	for path, content := range smokePackage(t) {
		r := a.do(a.admin, http.MethodPut, runtime.APIPrefix+"/packages/"+smokeNamespace+"/files/"+path, []byte(content))
		require.Equal(t, http.StatusOK, r.status, "put %s: %s", path, r.body)
	}
	var check manage.CheckReport
	a.manage(http.MethodPost, "/packages/"+smokeNamespace+"/check", nil, http.StatusOK, &check)
	require.False(t, check.HasErrors, "check: %+v", check.Diagnostics)
	var built manage.BuildReport
	a.manage(http.MethodPost, "/packages/"+smokeNamespace+"/build", nil, http.StatusOK, &built)
	require.False(t, built.HasErrors, "build: %+v", built.Diagnostics)
	var version manage.VersionDetail
	a.manage(http.MethodPost, "/packages/"+smokeNamespace+"/publish", map[string]string{"version": "0.1.0"}, http.StatusCreated, &version)
	require.Equal(t, built.ArtifactHash, version.ArtifactHash)
	a.manage(http.MethodPost, "/packages/"+smokeNamespace+"/versions/0.1.0/review", map[string]any{
		"approve": true, "comment": "smoke", "expectSourceHash": version.SourceHash, "expectArtifactHash": version.ArtifactHash,
	}, http.StatusCreated, nil)
	var activation manage.ActivationResult
	a.manage(http.MethodPost, "/packages/"+smokeNamespace+"/versions/0.1.0/activate", nil, http.StatusOK, &activation)
	require.NotEmpty(t, activation.SchemaSteps, "activation created the entity table")
	require.NotEmpty(t, activation.Bound, "the package's implementations are bound")

	// The command runs in the Node host worker through the host.
	status, out := call(" Ada ")
	require.Equal(t, http.StatusOK, status)
	require.True(t, out.OK)
	var value struct {
		Greeting string `json:"greeting"`
		Visits   int    `json:"visits"`
	}
	require.NoError(t, json.Unmarshal(out.Value, &value), string(out.Value))
	require.Equal(t, "Hello, Ada!", value.Greeting)
	require.Equal(t, 1, value.Visits)

	var rows int
	require.NoError(t, a.scope.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM x_smoke_greeter.visit WHERE tenant_id = $1`, a.tenant).Scan(&rows))
	require.Equal(t, 1, rows, "the command wrote its row into the entity table")
	var recorded int
	require.NoError(t, a.scope.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM extruntime.invocations WHERE contract = $1 AND outcome = 'ok'`, smokeNamespace+".greet").Scan(&recorded))
	require.Equal(t, 1, recorded, "the host recorded the invocation")
}

// TestServerComposition_ExtensionRuntimeDisabledByDefault pins the default:
// without EXTRUNTIME_ENABLED the server composes exactly as before, with no
// runtime, no management API and no browser RPC.
func TestServerComposition_ExtensionRuntimeDisabledByDefault(t *testing.T) {
	a := newSmokeApp(t, nil)

	_, err := composition.Resolve[*runtime.Runtime](a.scope.Container)
	require.Error(t, err, "no runtime is provided while the section is disabled")
	require.Equal(t, http.StatusNotFound, a.do(a.admin, http.MethodGet, runtime.APIPrefix+"/packages", nil).status)
	require.Equal(t, http.StatusNotFound, a.do(a.admin, http.MethodPost, "/_ext/rpc", []byte(`{}`), "Content-Type", "application/json").status)
	require.Equal(t, http.StatusNotFound, a.do(a.admin, http.MethodGet, "/extensions", nil).status)
}
