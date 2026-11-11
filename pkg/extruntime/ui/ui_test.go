package ui_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

const (
	hashV1   = "h1h1h1h1h1h1h1h1"
	hashShel = "5he115he115he115"
	hashOld  = "01d01d01d01d01d0"
	pkgName  = "@acme/insurance"
)

// fakeSource is a settable snapshot: activation, revert and replacement
// happen by changing it between requests, with no re-registration.
type fakeSource struct {
	mu   sync.Mutex
	snap *ui.Snapshot
	rev  int
}

func (s *fakeSource) set(pkgs []ui.ActivePackage, bindings ...ui.Binding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rev++
	s.snap = &ui.Snapshot{Revision: fmt.Sprint(s.rev), Packages: pkgs, Bindings: bindings}
}

func (s *fakeSource) Snapshot(context.Context) (*ui.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snap, nil
}

type fakeArtifacts map[string]string

func (a fakeArtifacts) Open(_ context.Context, hash, p string) (ui.ArtifactFile, error) {
	body, ok := a[hash+"/"+p]
	if !ok {
		return nil, ui.ErrArtifactNotFound
	}
	return nopCloser{bytes.NewReader([]byte(body))}, nil
}

type nopCloser struct{ io.ReadSeeker }

func (nopCloser) Close() error { return nil }

type fakeCatalog map[string]manifest.Resource

func (c fakeCatalog) Resolve(_ context.Context, id string) (spi.ResolvedResource, bool, error) {
	r, ok := c[id]
	return spi.ResolvedResource{Resource: r, Package: pkgName, Version: "v1"}, ok, nil
}
func (fakeCatalog) Entities(context.Context) ([]spi.ResolvedResource, error) { return nil, nil }
func (fakeCatalog) Guards(context.Context, string, manifest.GuardPhase) ([]spi.ResolvedGuard, error) {
	return nil, nil
}

type fakeInvoker struct {
	calls []host.Request
	out   json.RawMessage
	err   error
}

func (i *fakeInvoker) Invoke(_ context.Context, req host.Request) (host.Result, error) {
	i.calls = append(i.calls, req)
	return host.Result{InvocationID: "inv-1", Output: i.out}, i.err
}

func perm(name string) permission.Permission { return permission.New(permission.WithName(name)) }

func hostManifest() clienthost.Manifest {
	return clienthost.Manifest{
		Package: "@iota-uz/sdk", PackageVersion: "0.5.6",
		SDKCommit: "0123456789abcdef0123456789abcdef01234567", SDKReleaseVersion: sdkidentity.ReleaseVersion,
		ProtocolVersion: clienthost.ProtocolVersion, Entry: "/assets/runtime-host.js",
	}
}

func pkgV1(extra ...manifest.Resource) ui.ActivePackage {
	res := []manifest.Resource{
		{ID: "acme.insurance.policies", Kind: manifest.ResPage, Path: "/insurance/policies", Permission: "Policy.Read",
			Chunk: "client/policies.js", Export: "policies",
			Nav: &manifest.Nav{Label: manifest.Label{"en": "Policies", "ru": "Полисы"}, Group: "acme.insurance", Order: 1}},
		{ID: "acme.insurance.policy", Kind: manifest.ResPage, Path: "/insurance/policies/:id", Route: "acme.insurance.policyRoute",
			Permission: "Policy.Read", Chunk: "client/policy.js", Export: "policy"},
		{ID: "acme.insurance.policyRoute", Kind: manifest.ResRoute, Path: "/insurance/policies/:id",
			Params: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{{Name: "id", Type: manifest.Type{Kind: manifest.KindUUID}}}}},
		{ID: "acme.insurance.open", Kind: manifest.ResPage, Path: "/insurance/open", Chunk: "client/open.js", Export: "open",
			Nav: &manifest.Nav{Label: manifest.Label{"en": "Open quotes"}, Group: "acme.insurance", Order: 2}},
		{ID: "acme.insurance.nav", Kind: manifest.ResNavigation,
			Groups:  []manifest.NavGroup{{ID: "acme.insurance", Label: manifest.Label{"en": "Insurance", "ru": "Страхование"}}},
			Palette: []manifest.PaletteEntry{{ID: "acme.insurance.palette.policies", Label: manifest.Label{"en": "Go to policies"}, Target: "acme.insurance.policies", Keywords: []string{"insurance"}}},
		},
	}
	res = append(res, extra...)
	return ui.ActivePackage{Package: pkgName, Version: "v1", ArtifactHash: hashV1, Manifest: manifest.Manifest{Schema: manifest.SchemaVersion, Namespace: "acme.insurance", Resources: res}}
}

func shellPkg() ui.ActivePackage {
	return ui.ActivePackage{Package: "@acme/shell", Version: "v2", ArtifactHash: hashShel, Manifest: manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: "acme.shell",
		Resources: []manifest.Resource{{ID: "acme.shell.main", Kind: manifest.ResShell, Chunk: "client/shell.js", Export: "shell"}},
	}}
}

// legacy is a native Templ page in the authenticated layout, with the
// middleware chain of a real controller.
type legacy struct{ path, body string }

func (l legacy) Register(r *mux.Router) {
	s := r.PathPrefix(l.path).Subrouter()
	s.Use(middleware.Authorize(), middleware.RedirectNotAuthenticated(), middleware.ProvideUser(), middleware.ProvideDynamicLogo(), middleware.WithPageContext(), middleware.NavItems())
	s.HandleFunc("", func(w http.ResponseWriter, req *http.Request) {
		child := templ.Raw(`<div id="legacy-body">` + l.body + `</div>`)
		ctx := templ.WithChildren(req.Context(), child)
		page := layouts.Authenticated(layouts.AuthenticatedProps{BaseProps: layouts.BaseProps{Title: "Legacy"}})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Render(ctx, w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods(http.MethodGet)
}

type env struct {
	suite *itf.Suite
	src   *fakeSource
	rt    *ui.Runtime
	inv   *fakeInvoker
}

func newEnv(t *testing.T, perms ...string) *env {
	t.Helper()
	suite := itf.NewSuite(t, modules.Components()...)
	src := &fakeSource{}
	src.set(nil)
	inv := &fakeInvoker{out: json.RawMessage(`{"ok":1}`)}
	cat := fakeCatalog{
		"acme.insurance.expiring": {ID: "acme.insurance.expiring", Kind: manifest.ResQuery, Permission: "Policy.Read"},
		"acme.insurance.renew":    {ID: "acme.insurance.renew", Kind: manifest.ResCommand},
		"acme.insurance.activate": {ID: "acme.insurance.activate", Kind: manifest.ResCommand, Atomic: true},
		"acme.insurance.extend":   {ID: "acme.insurance.extend", Kind: manifest.ResOperation},
	}
	rt, err := ui.NewRuntime(ui.Config{
		Source: src,
		Artifacts: fakeArtifacts{
			hashV1 + "/client/policies.js": "export const policies = {}",
			hashV1 + "/client/chunk.css":   "body{}",
			hashV1 + "/server.mjs":         "export const secretHandler = 'HANDLER_SECRET'",
			hashV1 + "/contracts.mjs":      "export const contracts = {}",
			hashV1 + "/manifest.json":      "{}",
			hashV1 + "/package.json":       "{}",
			hashOld + "/client/old.js":     "export const old = {}",
		},
		Invoker: inv, Catalog: cat, Host: hostManifest(),
		NativeRoutes: ui.NativeRoutes{"test.routes.legacy": "/legacy"},
	})
	require.NoError(t, err)
	ps := make([]permission.Permission, 0, len(perms))
	for _, p := range perms {
		ps = append(ps, perm(p))
	}
	install := rt.Installer(nil)
	suite.AsUser(itf.User(ps...)).WithMiddleware(func(ctx context.Context, _ *http.Request) context.Context { return install(ctx) })
	// The page host is registered first, as its descriptor orders it, then a
	// native page that a screen binding can replace.
	suite.Register(ui.NewController(rt)).Register(legacy{path: "/legacy", body: "native legacy page"})
	return &env{suite: suite, src: src, rt: rt, inv: inv}
}

// contextOf extracts the JSON of a script element by id.
func scriptJSON(t *testing.T, body, id string, into any) {
	t.Helper()
	marker := `id="` + id + `" type="application/json">`
	i := strings.Index(body, marker)
	require.GreaterOrEqual(t, i, 0, "script %s not found in page", id)
	rest := body[i+len(marker):]
	j := strings.Index(rest, "</script>")
	require.GreaterOrEqual(t, j, 0)
	require.NoError(t, json.Unmarshal([]byte(rest[:j]), into), rest[:j])
}

// statusCode is the status of a recorded response. The recorder's result body
// is in memory; it is closed all the same.
func statusCode(resp *itf.Response) int {
	raw := resp.Raw()
	code := raw.StatusCode
	_ = raw.Body.Close()
	return code
}

func (e *env) get(t *testing.T, path string) (int, string) {
	t.Helper()
	resp := e.suite.GET(path).Expect(t)
	return statusCode(resp), resp.Body()
}

func (e *env) post(t *testing.T, path string, body any) (int, ui.RPCResponseForTest) {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	resp := e.suite.POST(path).Header("Content-Type", "application/json").JSON(json.RawMessage(b)).Expect(t)
	var out ui.RPCResponseForTest
	require.NoError(t, json.Unmarshal([]byte(resp.Body()), &out), resp.Body())
	return statusCode(resp), out
}

func TestPageHostServesActivatedPagesWithoutRestart(t *testing.T) {
	e := newEnv(t, "Policy.Read")

	status, _ := e.get(t, "/insurance/policies")
	assert.Equal(t, http.StatusNotFound, status, "nothing is activated yet")

	e.src.set([]ui.ActivePackage{pkgV1()})
	status, body := e.get(t, "/insurance/policies")
	require.Equal(t, http.StatusOK, status, body)
	// same boundary as legacy pages: inside the authenticated layout, with the outlet
	assert.Contains(t, body, "data-iota-shell-outlet")
	assert.Contains(t, body, `id="iota-client-route-mount"`)
	var rc clienthost.RouteContext
	scriptJSON(t, body, "iota-client-context", &rc)
	assert.Equal(t, "acme.insurance.policies", rc.Route.ID)
	assert.Equal(t, "/insurance/policies", rc.Route.Path)
	assert.Equal(t, "/_ext/rpc", rc.Services.RPC)
	assert.Contains(t, rc.Permissions, "Policy.Read")
	var data ui.PageData
	require.NoError(t, json.Unmarshal(rc.Initial, &data))
	assert.Equal(t, ui.PageDataKind, data.Kind)
	assert.Equal(t, "/_ext/assets/"+hashV1+"/client/policies.js", data.Page.Module, "immutable, content-addressed chunk URL")
	assert.Equal(t, "policies", data.Page.Export)
	assert.Contains(t, body, `src="/assets/runtime-host.js"`)
	assert.Contains(t, body, "<title>Policies</title>")

	// typed route params
	id := "6f1c1f0e-7a52-4b5e-9a39-0d2f6a3c4b11"
	status, body = e.get(t, "/insurance/policies/"+id)
	require.Equal(t, http.StatusOK, status, body)
	scriptJSON(t, body, "iota-client-context", &rc)
	require.NoError(t, json.Unmarshal(rc.Initial, &data))
	assert.Equal(t, map[string]string{"id": id}, data.Params)
	require.NotNil(t, data.ParamTypes)
	assert.Equal(t, manifest.KindObject, data.ParamTypes.Kind, "the client decodes params with the route's types")
	status, _ = e.get(t, "/insurance/policies/not-a-uuid")
	assert.Equal(t, http.StatusNotFound, status, "params are validated against the route contract")

	// deactivation takes effect on the next request
	e.src.set(nil)
	status, _ = e.get(t, "/insurance/policies")
	assert.Equal(t, http.StatusNotFound, status)
}

func TestPagePermissionIsEnforcedServerSideIndependentOfNavigation(t *testing.T) {
	e := newEnv(t) // no Policy.Read
	e.src.set([]ui.ActivePackage{pkgV1()})

	status, body := e.get(t, "/insurance/policies")
	assert.Equal(t, http.StatusForbidden, status)
	assert.NotContains(t, body, "iota-client-context", "no module reference leaks to a user without the permission")

	status, _ = e.get(t, "/insurance/open")
	assert.Equal(t, http.StatusOK, status, "a page without a permission is open to authenticated users")

	// the same page is hidden from the navigation, and still denied by URL
	status, body = e.get(t, "/legacy")
	require.Equal(t, http.StatusOK, status)
	assert.NotContains(t, body, "Policies")
}

func TestNavigationProjectionReachesSidebarAndShellContext(t *testing.T) {
	e := newEnv(t, "Policy.Read")
	patchOrder := 1
	_ = patchOrder
	e.src.set([]ui.ActivePackage{pkgV1()})

	status, body := e.get(t, "/legacy")
	require.Equal(t, http.StatusOK, status, body)
	assert.Contains(t, body, "Insurance", "group container in the Templ sidebar")
	assert.Contains(t, body, "Policies")
	assert.Contains(t, body, `href="/insurance/policies"`)

	// pkg/shellcontext publishes the very same navigation to Solid
	var sc struct {
		Navigation []struct {
			Key, Title, Href, Parent string
		}
	}
	scriptJSON(t, body, "iota-shell-context", &sc)
	var found, group bool
	for _, n := range sc.Navigation {
		if n.Key == "acme.insurance.policies" {
			found = true
			assert.Equal(t, "Policies", n.Title)
			assert.Equal(t, "/insurance/policies", n.Href)
			assert.Equal(t, "acme.insurance", n.Parent)
		}
		if n.Key == "acme.insurance" {
			group = true
		}
	}
	assert.True(t, found && group, "shell context lacks the projected entries: %+v", sc.Navigation)

	// a user without the page permission does not see the entry at all
	e2 := newEnv(t)
	e2.src.set([]ui.ActivePackage{pkgV1()})
	_, body = e2.get(t, "/legacy")
	assert.NotContains(t, body, `href="/insurance/policies"`)
}

func TestNavigationPatchesRenameMoveAndHideStandardItems(t *testing.T) {
	e := newEnv(t, "Policy.Read")
	type navDoc struct {
		Navigation []struct{ Key, Title, Parent string }
	}
	read := func() (navDoc, map[string]string, map[string]string) {
		_, body := e.get(t, "/legacy")
		var d navDoc
		scriptJSON(t, body, "iota-shell-context", &d)
		titles, parents := map[string]string{}, map[string]string{}
		for _, n := range d.Navigation {
			titles[n.Key], parents[n.Key] = n.Title, n.Parent
		}
		return d, titles, parents
	}
	before, _, _ := read()
	children := map[string]int{}
	for _, n := range before.Navigation {
		children[n.Parent]++
	}
	var std string // a standard root item with several children
	for _, n := range before.Navigation {
		if n.Parent == "" && children[n.Key] >= 2 {
			std = n.Key
			break
		}
	}
	require.NotEmpty(t, std, "the application has navigation to patch")

	first := 1
	e.src.set([]ui.ActivePackage{pkgV1(manifest.Resource{ID: "acme.insurance.patches", Kind: manifest.ResNavigation, Patches: []manifest.NavPatch{
		{Ref: std, Rename: manifest.Label{"en": "Renamed by package"}},
		{Ref: "acme.insurance.policies", Parent: std, Order: &first},
		{Ref: "missing.item", Hide: true},
	}})})
	_, titles, parents := read()
	assert.Equal(t, "Renamed by package", titles[std])
	assert.Equal(t, std, parents["acme.insurance.policies"], "moved under the standard item")

	e.src.set([]ui.ActivePackage{pkgV1(manifest.Resource{ID: "acme.insurance.hide", Kind: manifest.ResNavigation, Patches: []manifest.NavPatch{{Ref: std, Hide: true}}})})
	after, _, _ := read()
	for _, n := range after.Navigation {
		assert.NotEqual(t, std, n.Key, "hidden by patch")
		assert.NotEqual(t, std, n.Parent, "with its subtree")
	}
	// hiding is navigation-only: no page was removed
	status, _ := e.get(t, "/insurance/open")
	assert.Equal(t, http.StatusOK, status)
}

func TestScreenBindingReplacesNativePageAndRevertRestoresIt(t *testing.T) {
	e := newEnv(t, "Policy.Read")
	status, body := e.get(t, "/legacy")
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "native legacy page")

	pkg := pkgV1()
	binding := ui.Binding{ID: "b-1", SlotKind: ui.SlotScreen, TargetID: "test.routes.legacy", Package: pkgName, Version: "v1", Export: "open", ActiveFrom: time.Now()}
	e.src.set([]ui.ActivePackage{pkg}, binding)
	status, body = e.get(t, "/legacy")
	require.Equal(t, http.StatusOK, status, body)
	assert.NotContains(t, body, "native legacy page")
	var rc clienthost.RouteContext
	scriptJSON(t, body, "iota-client-context", &rc)
	var data ui.PageData
	require.NoError(t, json.Unmarshal(rc.Initial, &data))
	assert.Equal(t, "acme.insurance.open", rc.Route.ID)
	assert.Equal(t, "test.routes.legacy", data.ReplacedRoute)
	assert.Equal(t, "/legacy", rc.Route.Path)

	// the replacement is held to its own page permission
	e403 := newEnv(t)
	e403.src.set([]ui.ActivePackage{pkgV1(manifest.Resource{ID: "acme.insurance.guarded", Kind: manifest.ResPage, Path: "/insurance/guarded", Permission: "Policy.Read", Chunk: "client/g.js", Export: "guarded"})},
		ui.Binding{ID: "b-2", SlotKind: ui.SlotScreen, TargetID: "test.routes.legacy", Package: pkgName, Version: "v1", Export: "guarded"})
	status, _ = e403.get(t, "/legacy")
	assert.Equal(t, http.StatusForbidden, status)

	// admin revert: closing the binding brings the native page back
	e.src.set([]ui.ActivePackage{pkg})
	status, body = e.get(t, "/legacy")
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "native legacy page")

	// a binding to a missing page falls back to the native one
	e.src.set([]ui.ActivePackage{pkg}, ui.Binding{ID: "b-3", SlotKind: ui.SlotScreen, TargetID: "test.routes.legacy", Package: pkgName, Version: "v1", Export: "missing"})
	_, body = e.get(t, "/legacy")
	assert.Contains(t, body, "native legacy page")
}

func TestShellBindingIsInjectedIntoLegacyAndRuntimePages(t *testing.T) {
	e := newEnv(t, "Policy.Read")
	_, body := e.get(t, "/legacy")
	assert.NotContains(t, body, "iota-runtime-context", "no binding, standard shell, no runtime")
	assert.Contains(t, body, "data-iota-shell-standard")

	shell := ui.Binding{ID: "s-1", SlotKind: ui.SlotShell, TargetID: "core.shell", Package: "@acme/shell", Version: "v2", Export: "shell", ActiveFrom: time.Now()}
	e.src.set([]ui.ActivePackage{pkgV1(), shellPkg()}, shell)

	for _, path := range []string{"/legacy", "/insurance/policies"} {
		status, body := e.get(t, path)
		require.Equal(t, http.StatusOK, status, path)
		var rc ui.RuntimeContext
		scriptJSON(t, body, ui.RuntimeContextElementID, &rc)
		require.NotNil(t, rc.Shell, path)
		assert.Equal(t, "acme.shell.main", rc.Shell.ID)
		assert.Equal(t, "/_ext/assets/"+hashShel+"/client/shell.js", rc.Shell.Module)
		assert.Equal(t, "s-1", rc.Shell.Binding)
		assert.Equal(t, "/_ext/rpc", rc.RPC)
		assert.Contains(t, rc.Permissions, "Policy.Read")
		assert.Contains(t, body, `src="/assets/runtime-host.js"`, path)
		// the standard shell and its outlet stay in the page, so the host can fall back
		assert.Contains(t, body, "data-iota-shell-standard", path)
		assert.Contains(t, body, "data-iota-shell-outlet", path)
		// workspaces / palette extras, permission filtered and localized
		require.Len(t, rc.Navigation.Palette, 1, path)
		assert.Equal(t, "/insurance/policies", rc.Navigation.Palette[0].Href)
	}

	// without the page permission the palette entry is not offered
	e2 := newEnv(t)
	e2.src.set([]ui.ActivePackage{pkgV1(), shellPkg()}, shell)
	_, body = e2.get(t, "/legacy")
	var rc ui.RuntimeContext
	scriptJSON(t, body, ui.RuntimeContextElementID, &rc)
	assert.Empty(t, rc.Navigation.Palette)

	// admin revert: closing the binding returns the standard shell, no restart
	e.src.set([]ui.ActivePackage{pkgV1(), shellPkg()})
	_, body = e.get(t, "/legacy")
	assert.NotContains(t, body, "iota-runtime-context")
	assert.Contains(t, body, "data-iota-shell-standard")
}

func TestRPCInvokesAsTheSessionUserAndKeepsStructuredErrors(t *testing.T) {
	e := newEnv(t, "Policy.Read")
	req := map[string]any{"kind": "query", "resource": "acme.insurance.expiring", "input": map[string]any{"days": 30}}

	status, out := e.post(t, "/_ext/rpc", req)
	require.Equal(t, http.StatusOK, status)
	assert.True(t, out.OK)
	assert.JSONEq(t, `{"ok":1}`, string(out.Value))
	require.Len(t, e.inv.calls, 1)
	call := e.inv.calls[0]
	assert.Equal(t, "acme.insurance.expiring", call.Contract)
	assert.JSONEq(t, `{"days":30}`, string(call.Input))
	assert.Equal(t, host.TriggerUI, call.Trigger.Kind)
	assert.Contains(t, call.Subject.Permissions, "Policy.Read", "the session user's current permissions")
	assert.Equal(t, spi.SubjectInitiator, call.Subject.Kind)

	// declared domain error crosses with kind, code and params intact
	e.inv.err = &pool.WireError{Kind: pool.ErrKindDomain, Code: "acme.insurance.notActive", Params: json.RawMessage(`{"status":"draft"}`)}
	status, out = e.post(t, "/_ext/rpc", map[string]any{"kind": "command", "resource": "acme.insurance.renew", "input": nil})
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	require.NotNil(t, out.Error)
	assert.Equal(t, "domain", out.Error.Kind)
	assert.Equal(t, "acme.insurance.notActive", out.Error.Code)
	assert.JSONEq(t, `{"status":"draft"}`, string(out.Error.Params))

	// validation issues keep phase, source, path and params
	e.inv.err = &pool.WireError{Kind: pool.ErrKindValidation, Code: "validation", Issues: []pool.Issue{{
		Code: "acme.insurance.tooLong", Phase: "input", Source: "acme.insurance.check", Path: json.RawMessage(`["notes"]`), Params: json.RawMessage(`{"max":10}`)}}}
	status, out = e.post(t, "/_ext/rpc", map[string]any{"kind": "atomic", "resource": "acme.insurance.activate", "input": map[string]any{}})
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	require.Len(t, out.Error.Issues, 1)
	assert.Equal(t, "acme.insurance.tooLong", out.Error.Issues[0].Code)
	assert.JSONEq(t, `["notes"]`, string(out.Error.Issues[0].Path))
	assert.JSONEq(t, `{"max":10}`, string(out.Error.Issues[0].Params))

	// no binding is reported, not retried against another implementation
	e.inv.err = host.ErrNoBinding
	status, out = e.post(t, "/_ext/rpc", map[string]any{"kind": "command", "resource": "acme.insurance.renew", "input": nil})
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, ui.CodeNoBinding, out.Error.Code)
	e.inv.err = nil
}

func TestRPCRejectsWhatTheBrowserMustNotCall(t *testing.T) {
	e := newEnv(t) // no Policy.Read
	before := len(e.inv.calls)

	status, out := e.post(t, "/_ext/rpc", map[string]any{"kind": "query", "resource": "acme.insurance.expiring", "input": nil})
	assert.Equal(t, http.StatusForbidden, status, "the contract permission is enforced for the browser")
	assert.Equal(t, "denied", out.Error.Kind)

	status, out = e.post(t, "/_ext/rpc", map[string]any{"kind": "command", "resource": "acme.insurance.extend", "input": nil})
	assert.Equal(t, http.StatusForbidden, status, "operations are internal")
	assert.Equal(t, ui.CodeNotCallable, out.Error.Code)

	status, out = e.post(t, "/_ext/rpc", map[string]any{"kind": "query", "resource": "acme.insurance.renew", "input": nil})
	assert.Equal(t, ui.CodeNotCallable, out.Error.Code, "the declared kind must match the contract")
	_ = status

	status, out = e.post(t, "/_ext/rpc", map[string]any{"kind": "query", "resource": "acme.insurance.nope", "input": nil})
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, ui.CodeUnknownResource, out.Error.Code)

	status, _ = e.post(t, "/_ext/rpc", map[string]any{"resource": "x", "bogus": 1})
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Len(t, e.inv.calls, before, "nothing reached the host")
}

func TestAssetsAreImmutableAndConfined(t *testing.T) {
	e := newEnv(t)
	e.src.set([]ui.ActivePackage{pkgV1()})
	resp := e.suite.GET("/_ext/assets/" + hashV1 + "/client/policies.js").Expect(t).Status(200)
	assert.Equal(t, "export const policies = {}", resp.Body())
	assert.Contains(t, resp.Header("Cache-Control"), "immutable")
	assert.Contains(t, resp.Header("Cache-Control"), "max-age=31536000")
	assert.Equal(t, "text/javascript; charset=utf-8", resp.Header("Content-Type"))
	assert.Equal(t, "nosniff", resp.Header("X-Content-Type-Options"))
	etag := resp.Header("ETag")
	require.NotEmpty(t, etag)

	e.suite.GET("/_ext/assets/"+hashV1+"/client/policies.js").Header("If-None-Match", etag).Expect(t).Status(304)
	e.suite.GET("/_ext/assets/" + hashV1 + "/client/missing.js").Expect(t).Status(404)
	for _, p := range []string{"/../x/secret.js", "/client/%2e%2e/%2e%2e/etc/passwd", "/client//policies.js"} {
		resp := e.suite.GET("/_ext/assets/" + hashV1 + p).Expect(t)
		assert.NotEqual(t, 200, statusCode(resp), p)
		assert.NotContains(t, resp.Body(), "export const", p)
	}
	e.suite.GET("/_ext/assets/short/client/policies.js").Expect(t).Status(404)
}

// The route serves the client subtree of artifacts that are live, nothing else:
// not the server bundle, contracts or manifests of a live artifact, and not
// any file of an artifact that is no longer (or not yet) activated.
func TestAssetsServeOnlyTheClientSubtreeOfLiveArtifacts(t *testing.T) {
	e := newEnv(t)
	client := "/_ext/assets/" + hashV1 + "/client/policies.js"

	e.suite.GET(client).Expect(t).Status(404) // nothing activated yet
	e.src.set([]ui.ActivePackage{pkgV1()})
	e.suite.GET(client).Expect(t).Status(200)
	e.suite.GET("/_ext/assets/" + hashV1 + "/client/chunk.css").Expect(t).Status(200)
	for _, p := range []string{"server.mjs", "contracts.mjs", "manifest.json", "package.json"} {
		resp := e.suite.GET("/_ext/assets/" + hashV1 + "/" + p).Expect(t)
		assert.Equal(t, 404, statusCode(resp), p)
		assert.NotContains(t, resp.Body(), "HANDLER_SECRET", p)
	}
	e.suite.GET("/_ext/assets/" + hashOld + "/client/old.js").Expect(t).Status(404) // never activated

	// a screen binding keeps its package's artifact reachable; a deactivated
	// package's artifact is gone
	e.src.set([]ui.ActivePackage{shellPkg()}, ui.Binding{ID: "b1", SlotKind: ui.SlotShell, Package: "@acme/shell", Version: "v2", Export: "shell"})
	e.suite.GET(client).Expect(t).Status(404)
}

func TestIndexResolvesMostSpecificPageAndReportsConflicts(t *testing.T) {
	mk := func(pkg string, res ...manifest.Resource) ui.ActivePackage {
		return ui.ActivePackage{Package: pkg, Version: "v1", ArtifactHash: "hhhhhhhh-" + strings.NewReplacer("@", "", "/", "-").Replace(pkg), Manifest: manifest.Manifest{Resources: res}}
	}
	page := func(id, path string) manifest.Resource {
		return manifest.Resource{ID: id, Kind: manifest.ResPage, Path: path, Chunk: "c.js"}
	}
	ix := ui.BuildIndex(&ui.Snapshot{Revision: "1", Packages: []ui.ActivePackage{
		mk("@a/one", page("a.one.items", "/items/:id"), page("a.one.new", "/items/new")),
		mk("@b/two", page("b.two.dup", "/items/new")),
	}}, nil)
	r, ok := ix.Resolve("/items/new")
	require.True(t, ok)
	assert.Equal(t, "a.one.new", r.Page.ID, "a literal beats a parameter; the first package keeps a contested path")
	r, ok = ix.Resolve("/items/42/")
	require.True(t, ok)
	assert.Equal(t, map[string]string{"id": "42"}, r.Params)
	_, ok = ix.Resolve("/items")
	assert.False(t, ok)
	require.Len(t, ix.Diagnostics, 1)
	assert.Contains(t, ix.Diagnostics[0], "b.two.dup")
}

func TestAssetPathConfinement(t *testing.T) {
	ok := map[string]string{"client/a.js": "client/a.js", "a.js": "a.js", "client/sub/b.css": "client/sub/b.css"}
	for in, want := range ok {
		got, valid := ui.CleanAssetPath(in)
		assert.True(t, valid, in)
		assert.Equal(t, want, got)
	}
	for _, in := range []string{"", "/abs.js", "../x.js", "client/../../x.js", "client//a.js", "client\\a.js", "a/./b.js", "a\x00.js", "client/.."} {
		_, valid := ui.CleanAssetPath(in)
		assert.False(t, valid, "%q must be rejected", in)
	}
}

func TestIndexSkipsPagesItCannotServe(t *testing.T) {
	bad := ui.ActivePackage{Package: "@a/bad", Version: "v1", ArtifactHash: "x", Manifest: manifest.Manifest{Resources: []manifest.Resource{
		{ID: "a.bad.short", Kind: manifest.ResPage, Path: "/short", Chunk: "c.js"},
	}}}
	escape := ui.ActivePackage{Package: "@a/esc", Version: "v1", ArtifactHash: "hhhhhhhh", Manifest: manifest.Manifest{Resources: []manifest.Resource{
		{ID: "a.esc.up", Kind: manifest.ResPage, Path: "/up", Chunk: "../other/c.js"},
		{ID: "a.esc.shell", Kind: manifest.ResShell, Chunk: "../s.js", Export: "shell"},
	}}}
	ix := ui.BuildIndex(&ui.Snapshot{Revision: "1", Packages: []ui.ActivePackage{bad, escape},
		Bindings: []ui.Binding{{ID: "s", SlotKind: ui.SlotShell, Package: "@a/esc", Version: "v1", Export: "shell"}}}, nil)
	assert.Empty(t, ix.Pages)
	assert.Nil(t, ix.Shell)
	assert.NotEmpty(t, ix.Diagnostics)
}
