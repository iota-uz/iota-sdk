package mcptools_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage/managetest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/mcptools"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/rbacauth"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

const (
	adminID  = 1
	readerID = 2
	// authorID holds Extensions.Read and Author only: it may draft but not publish.
	authorID = 3
)

type invokeCall struct {
	req host.Request
}

// fakeInvoker stands in for the host (which needs a Node worker).
type fakeInvoker struct {
	mu     sync.Mutex
	calls  []invokeCall
	result host.Result
	err    error
}

func (f *fakeInvoker) Invoke(_ context.Context, req host.Request) (host.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, invokeCall{req})
	return f.result, f.err
}

type env struct {
	t      *testing.T
	tenant uuid.UUID
	db     *pgxpool.Pool
	svc    *manage.Service
	srv    *httptest.Server
	engine *mcp.Engine
	invoke *fakeInvoker
	users  map[uint]user.User
	audit  *recordingAuditor
}

type recordingAuditor struct {
	mu      sync.Mutex
	entries []mcp.AuditEntry
}

func (a *recordingAuditor) Record(_ context.Context, e mcp.AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, e)
}

func account(t *testing.T, tenant uuid.UUID, id uint, perms ...permission.Permission) user.User {
	t.Helper()
	email, err := internet.NewEmail(fmt.Sprintf("u%d@example.com", id))
	require.NoError(t, err)
	r := role.New("staff", role.WithID(id), role.WithPermissions(perms), role.WithTenantID(tenant))
	return user.New("", "", email, "", user.WithID(id), user.WithTenantID(tenant), user.WithRoles([]role.Role{r}))
}

func newEnv(t *testing.T) *env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_mcptools",
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
	e := &env{t: t, tenant: scope.Tenant.ID, db: scope.Pool, svc: svc, invoke: &fakeInvoker{}, audit: &recordingAuditor{}}

	e.users = map[uint]user.User{
		adminID:  account(t, e.tenant, adminID, corePerms.ExtensionsRead, corePerms.ExtensionsAuthor, corePerms.ExtensionsReview, corePerms.ExtensionsPublish, corePerms.ExtensionsOperate),
		readerID: account(t, e.tenant, readerID, corePerms.ExtensionsRead),
		authorID: account(t, e.tenant, authorID, corePerms.ExtensionsRead, corePerms.ExtensionsAuthor),
	}

	// HTTP transport: the same subject the session controller would build.
	perms := func(id int64) []string {
		u, ok := e.users[uint(id)]
		if !ok {
			return nil
		}
		var names []string
		for _, p := range user.EffectivePermissions(u) {
			names = append(names, p.Name())
		}
		return names
	}
	subject := func(r *http.Request) (spi.Subject, error) {
		id, _ := strconv.ParseInt(r.Header.Get("X-User"), 10, 64)
		if id == 0 {
			return spi.Subject{}, fmt.Errorf("no user")
		}
		return spi.Subject{TenantID: e.tenant, UserID: id, Kind: spi.SubjectInitiator, Permissions: perms(id), Credential: "session"}, nil
	}
	e.srv = httptest.NewServer(runtime.NewAPI(svc, subject, nil).Handler())
	t.Cleanup(e.srv.Close)

	// MCP transport: the registry, the RBAC authorizer and the engine.
	reg := mcp.NewRegistry()
	require.NoError(t, mcptools.Register(reg, mcptools.Deps{Manage: svc, Invoker: e.invoke}))
	authz := rbacauth.New(func(_ context.Context, tenant uuid.UUID, id uint) (user.User, error) {
		u, ok := e.users[id]
		if !ok || tenant != e.tenant {
			return nil, fmt.Errorf("unknown account")
		}
		return u, nil
	})
	e.engine, err = mcp.NewEngine(reg, authz, mcp.WithAuditor(e.audit))
	require.NoError(t, err)
	return e
}

func (e *env) principal(id uint, scopes mcp.Scopes) context.Context {
	return mcp.WithPrincipal(context.Background(), mcp.Principal{TenantID: e.tenant, UserID: id, ClientID: "test-client", Scopes: scopes})
}

// call runs a tool through the engine as a first-party session of the user.
func (e *env) call(id uint, tool string, input any) mcp.Result {
	e.t.Helper()
	return e.callCtx(e.principal(id, mcp.AllScopes()), tool, input)
}

func (e *env) callCtx(ctx context.Context, tool string, input any) mcp.Result {
	e.t.Helper()
	raw, err := json.Marshal(input)
	require.NoError(e.t, err)
	res := e.engine.Call(ctx, tool, raw)
	if res.OK {
		e.validateOutput(tool, res.Data)
	}
	return res
}

// validateOutput checks a result against the output schema the tool publishes
// to MCP clients, so the schema cannot drift from what the tool returns.
func (e *env) validateOutput(tool string, data json.RawMessage) {
	e.t.Helper()
	for _, spec := range e.engine.Registry().Specs() {
		if spec.Name != tool || spec.Output == nil {
			continue
		}
		schema, err := json.Marshal(spec.Output)
		require.NoError(e.t, err)
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
		require.NoError(e.t, err)
		c := jsonschema.NewCompiler()
		c.DefaultDraft(jsonschema.Draft2020)
		require.NoError(e.t, c.AddResource("urn:out", doc))
		compiled, err := c.Compile("urn:out")
		require.NoError(e.t, err, tool)
		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		require.NoError(e.t, err)
		require.NoError(e.t, compiled.Validate(inst), "%s result does not match its published output schema: %s", tool, data)
	}
}

func (e *env) ok(id uint, tool string, input any, into any) {
	e.t.Helper()
	res := e.call(id, tool, input)
	require.True(e.t, res.OK, "%s: %+v", tool, res.Error)
	if into != nil {
		require.NoError(e.t, json.Unmarshal(res.Data, into))
	}
}

func (e *env) failed(id uint, tool string, input any, code mcp.Code) *mcp.Failure {
	e.t.Helper()
	res := e.call(id, tool, input)
	require.False(e.t, res.OK, "%s should fail", tool)
	require.Equal(e.t, code, res.Error.Code, "%s: %+v", tool, res.Error)
	return res.Error
}

func (e *env) http(user int, method, path string, body any, headers ...string) (int, []byte) {
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
	req, err := http.NewRequest(method, e.srv.URL+runtime.APIPrefix+path, rd)
	require.NoError(e.t, err)
	req.Header.Set("X-User", strconv.Itoa(user))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(e.t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(e.t, err)
	return resp.StatusCode, out
}

func obj(props ...manifest.Property) *manifest.Type {
	return &manifest.Type{Kind: manifest.KindObject, Properties: props}
}

func manifestOf(ns string) manifest.Manifest {
	return manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: ns,
		Resources: []manifest.Resource{
			{ID: ns + ".policy", Kind: manifest.ResEntity, Description: "An insurance policy.",
				Fields: []manifest.Field{{ID: "number", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 20}, Unique: true}}},
			{ID: ns + ".renewPolicy", Kind: manifest.ResCommand, Description: "Renews a policy for another year.", Permission: "Policy.Renew",
				Input:  obj(manifest.Property{Name: "number", Type: manifest.Type{Kind: manifest.KindString}}, manifest.Property{Name: "note", Type: manifest.Type{Kind: manifest.KindString, Optional: true}}),
				Output: obj(manifest.Property{Name: "ok", Type: manifest.Type{Kind: manifest.KindBool}})},
		},
		Implementations: []manifest.Implementation{{Export: "renew", Implements: ns + ".renewPolicy", Kind: manifest.ImplCommand}},
	}
}

// sortedPaths gives file writes a deterministic order, so audit trails compare.
func sortedPaths(files map[string]string) []string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

func files(t *testing.T, ns, version string) map[string]string {
	t.Helper()
	m, err := json.Marshal(manifestOf(ns))
	require.NoError(t, err)
	return map[string]string{
		"package.json":  fmt.Sprintf(`{"name":"@acme/ins","version":%q,"iota":{"namespace":%q}}`, version, ns),
		"manifest.json": string(m),
		"server.mjs":    "export const renew = async () => ({ ok: true });\n",
		"contracts.mjs": "export {}\n",
		"src/bad.ts":    "export const x = BAD_TYPE;\n",
	}
}

type auditRow struct {
	Actor       int64
	Credential  string
	Action      string
	Outcome     string
	Target      string
	Version     string
	HasSource   bool
	HasArtifact bool
	Error       string
}

func (e *env) auditRows(ns string) []auditRow {
	e.t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT actor_user_id, credential, action, outcome, target, version,
		source_hash <> '', artifact_hash <> '', COALESCE(detail->>'error', '')
		FROM extruntime.audit_log WHERE namespace = $1 OR (namespace = '' AND target LIKE $2) ORDER BY id`, ns, "%"+ns+"%")
	require.NoError(e.t, err)
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		require.NoError(e.t, rows.Scan(&r.Actor, &r.Credential, &r.Action, &r.Outcome, &r.Target, &r.Version, &r.HasSource, &r.HasArtifact, &r.Error))
		r.Target = strings.ReplaceAll(r.Target, ns, "NS")
		r.Error = strings.ReplaceAll(r.Error, ns, "NS")
		out = append(out, r)
	}
	require.NoError(e.t, rows.Err())
	return out
}

func TestEveryToolIsRegisteredWithASchemaAndAPolicy(t *testing.T) {
	svc := &manage.Service{}
	tools, err := mcptools.Tools(mcptools.Deps{Manage: svc, Catalog: catalog.New(nil, catalog.NewNativeSet(data.DefaultNatives())), Invoker: &fakeInvoker{}})
	require.NoError(t, err)
	names := map[string]mcp.Spec{}
	for _, tl := range tools {
		s := tl.Spec()
		names[s.Name] = s
		require.NotNil(t, s.Input, s.Name)
		require.Equal(t, "object", s.Input.Type, s.Name)
		require.NotEmpty(t, s.Description, s.Name)
		require.NotEmpty(t, s.Policy.Scopes, s.Name)
	}
	for _, want := range []string{
		"ext_package_list", "ext_package_get", "ext_package_create", "ext_file_list", "ext_file_read", "ext_file_write",
		"ext_file_delete", "ext_file_move", "ext_import_git", "ext_export", "ext_check", "ext_build", "ext_test",
		"ext_publish", "ext_version_list", "ext_version_get", "ext_review", "ext_activate", "ext_deactivate", "ext_purge",
		"ext_activation_list", "ext_binding_list", "ext_bind", "ext_unbind", "ext_owner_list", "ext_owner_assign",
		"ext_owner_remove", "ext_kit_info", "ext_kit_module", "ext_audit_list", "ext_invoke", "ext_resource_list",
		"ext_resource_get", "ext_workflow_list", "ext_workflow_get", "ext_workflow_signal", "ext_workflow_decide_task",
		"ext_workflow_terminate", "ext_workflow_start_v2", "ext_workflow_resolve", "ext_workflow_retry", "ext_workflow_resume",
	} {
		require.Contains(t, names, want)
	}
	// every tool documents the RBAC permission the service requires
	require.Equal(t, []string{manage.PermPublish}, names["ext_publish"].Meta[mcptools.MetaPermissions])
	// read tools are admitted by Extensions.Read already in the engine
	require.Equal(t, []string{manage.PermRead}, names["ext_package_list"].Policy.Permissions)
	// audited tools leave the decision (and the denied audit row) to the service
	require.Empty(t, names["ext_publish"].Policy.Permissions)
}

// The same management sequence over the HTTP API and through the MCP engine
// leaves the same audit trail and the same refusals.
func TestMCPAndHTTPShareRightsAndAudit(t *testing.T) {
	e := newEnv(t)
	const httpNS, mcpNS = "acme.viahttp", "acme.viamcp"
	ctx := context.Background()

	// --- over HTTP ---
	status, _ := e.http(readerID, http.MethodPost, "/packages", map[string]string{"namespace": httpNS})
	require.Equal(t, http.StatusForbidden, status)
	status, _ = e.http(authorID, http.MethodPost, "/packages", map[string]string{"namespace": httpNS})
	require.Equal(t, http.StatusCreated, status)
	httpFiles := files(t, httpNS, "1.0.0")
	for _, p := range sortedPaths(httpFiles) {
		c := httpFiles[p]
		status, out := e.http(authorID, http.MethodPut, "/packages/"+httpNS+"/files/"+p, []byte(c))
		require.Equal(t, http.StatusOK, status, string(out))
	}
	status, _ = e.http(authorID, http.MethodPut, "/packages/"+httpNS+"/files/server.mjs", []byte("x"), "If-Match", `"stale"`)
	require.Equal(t, http.StatusConflict, status)
	status, _ = e.http(authorID, http.MethodPost, "/packages/"+httpNS+"/check", nil)
	require.Equal(t, http.StatusOK, status)
	status, _ = e.http(authorID, http.MethodPost, "/packages/"+httpNS+"/build", nil)
	require.Equal(t, http.StatusOK, status)
	status, _ = e.http(authorID, http.MethodDelete, "/packages/"+httpNS+"/files/src/bad.ts", nil)
	require.Equal(t, http.StatusNoContent, status)
	status, _ = e.http(authorID, http.MethodPost, "/packages/"+httpNS+"/test", nil)
	require.Equal(t, http.StatusOK, status)
	status, _ = e.http(authorID, http.MethodPost, "/packages/"+httpNS+"/publish", map[string]string{"version": "1.0.0"})
	require.Equal(t, http.StatusForbidden, status)
	var v manage.VersionDetail
	status, out := e.http(adminID, http.MethodPost, "/packages/"+httpNS+"/publish", map[string]string{"version": "1.0.0"})
	require.Equal(t, http.StatusCreated, status, string(out))
	require.NoError(t, json.Unmarshal(out, &v))
	status, _ = e.http(adminID, http.MethodPost, "/packages/"+httpNS+"/versions/1.0.0/review", map[string]any{"approve": true, "comment": "ok", "expectSourceHash": v.SourceHash, "expectArtifactHash": v.ArtifactHash})
	require.Equal(t, http.StatusCreated, status)
	status, _ = e.http(adminID, http.MethodPost, "/packages/"+httpNS+"/versions/1.0.0/activate", nil)
	require.Equal(t, http.StatusOK, status)
	status, _ = e.http(readerID, http.MethodGet, "/packages/"+httpNS+"/export?format=tar&version=1.0.0", nil)
	require.Equal(t, http.StatusOK, status)
	status, _ = e.http(readerID, http.MethodPost, "/packages/"+httpNS+"/deactivate", nil)
	require.Equal(t, http.StatusForbidden, status)
	status, _ = e.http(adminID, http.MethodPost, "/packages/"+httpNS+"/deactivate", nil)
	require.Equal(t, http.StatusNoContent, status)

	// --- through the MCP engine, the same sequence ---
	e.failed(readerID, "ext_package_create", map[string]string{"namespace": mcpNS}, mcp.CodeForbidden)
	e.ok(authorID, "ext_package_create", map[string]string{"namespace": mcpNS}, nil)
	mcpFiles := files(t, mcpNS, "1.0.0")
	for _, p := range sortedPaths(mcpFiles) {
		e.ok(authorID, "ext_file_write", map[string]string{"namespace": mcpNS, "path": p, "content": mcpFiles[p]}, nil)
	}
	e.failed(authorID, "ext_file_write", map[string]string{"namespace": mcpNS, "path": "server.mjs", "content": "x", "expected_hash": "stale"}, mcp.CodeConflict)
	var chk manage.CheckReport
	e.ok(authorID, "ext_check", map[string]string{"namespace": mcpNS}, &chk)
	var rep manage.BuildReport
	e.ok(authorID, "ext_build", map[string]string{"namespace": mcpNS}, &rep)
	require.True(t, rep.HasErrors, "the build reports the BAD_TYPE diagnostic with file and line, like HTTP")
	require.Equal(t, "src/bad.ts", rep.Diagnostics[0].File)
	require.Equal(t, 1, rep.Diagnostics[0].Line)
	e.ok(authorID, "ext_file_delete", map[string]string{"namespace": mcpNS, "path": "src/bad.ts"}, nil)
	e.ok(authorID, "ext_test", map[string]string{"namespace": mcpNS}, nil)
	e.failed(authorID, "ext_publish", map[string]string{"namespace": mcpNS, "version": "1.0.0"}, mcp.CodeForbidden)
	var mv manage.VersionDetail
	e.ok(adminID, "ext_publish", map[string]string{"namespace": mcpNS, "version": "1.0.0"}, &mv)
	e.ok(adminID, "ext_review", map[string]any{"namespace": mcpNS, "version": "1.0.0", "approve": true, "comment": "ok", "expect_source_hash": mv.SourceHash, "expect_artifact_hash": mv.ArtifactHash}, nil)
	var act manage.ActivationResult
	e.ok(adminID, "ext_activate", map[string]string{"namespace": mcpNS, "version": "1.0.0"}, &act)
	require.Len(t, act.Bound, 1)
	var exp struct {
		Filename   string
		TreeHash   string `json:"tree_hash"`
		DataBase64 string `json:"data_base64"`
	}
	e.ok(readerID, "ext_export", map[string]string{"namespace": mcpNS, "version": "1.0.0", "format": "tar"}, &exp)
	raw, err := base64.StdEncoding.DecodeString(exp.DataBase64)
	require.NoError(t, err)
	tree, err := manage.UnpackArchive(raw)
	require.NoError(t, err)
	require.Contains(t, tree, "package.json")
	e.failed(readerID, "ext_deactivate", map[string]string{"namespace": mcpNS}, mcp.CodeForbidden)
	e.ok(adminID, "ext_deactivate", map[string]string{"namespace": mcpNS}, nil)

	// --- the audit trails are the same, except for the credential ---
	httpRows, mcpRows := e.auditRows(httpNS), e.auditRows(mcpNS)
	require.NotEmpty(t, httpRows)
	for i := range httpRows {
		require.Equal(t, "session", httpRows[i].Credential)
	}
	for i := range mcpRows {
		require.Equal(t, "mcp:test-client", mcpRows[i].Credential, "the MCP credential is named in the audit row")
		mcpRows[i].Credential, httpRows[i].Credential = "", ""
	}
	require.Equal(t, httpRows, mcpRows)

	// denied attempts were audited with the real actor, not dropped
	var denied int
	for _, r := range mcpRows {
		if r.Outcome == manage.OutcomeDenied {
			denied++
			require.Contains(t, []int64{readerID, authorID}, r.Actor)
			require.Contains(t, r.Error, "Extensions.")
		}
	}
	require.GreaterOrEqual(t, denied, 3)
	_ = ctx
}

func TestPrincipalIsTheRealUserAndScopesNeverWidenRBAC(t *testing.T) {
	e := newEnv(t)

	// a grant carrying every scope does not give a reader author rights
	res := e.callCtx(e.principal(readerID, mcp.GrantedScopes(mcptools.ScopeRead, mcptools.ScopeManage, mcptools.ScopeInvoke)),
		"ext_package_create", map[string]string{"namespace": "acme.reader"})
	require.False(t, res.OK)
	require.Equal(t, mcp.CodeForbidden, res.Error.Code)
	rows := e.auditRows("acme.reader")
	require.Len(t, rows, 1)
	require.Equal(t, auditRow{Actor: readerID, Credential: "mcp:test-client", Action: manage.ActionCreatePackage, Outcome: manage.OutcomeDenied,
		Error: "missing " + manage.PermAuthor}, rows[0])

	// a read-only grant cannot attempt writes even for an admin; nothing is audited
	res = e.callCtx(e.principal(adminID, mcp.GrantedScopes(mcptools.ScopeRead)), "ext_package_create", map[string]string{"namespace": "acme.scoped"})
	require.False(t, res.OK)
	require.Equal(t, mcp.CodeForbidden, res.Error.Code)
	require.Contains(t, res.Error.Message, mcptools.ScopeManage)
	require.Empty(t, e.auditRows("acme.scoped"))
	// ...but may read
	res = e.callCtx(e.principal(adminID, mcp.GrantedScopes(mcptools.ScopeRead)), "ext_package_list", struct{}{})
	require.True(t, res.OK)

	// the same account without Extensions.Read is refused by the engine policy
	e.users[readerID] = account(t, e.tenant, readerID)
	e.failed(readerID, "ext_package_list", struct{}{}, mcp.CodeForbidden)
	// revoked rights apply to the next call of a warm credential
	e.users[adminID] = account(t, e.tenant, adminID, corePerms.ExtensionsRead)
	e.failed(adminID, "ext_package_create", map[string]string{"namespace": "acme.revoked"}, mcp.CodeForbidden)

	// no principal, no call; a service subject has no extension rights
	res = e.engine.Call(context.Background(), "ext_package_list", json.RawMessage(`{}`))
	require.False(t, res.OK)
	res = e.engine.Call(mcp.WithPrincipal(context.Background(), mcp.Principal{TenantID: e.tenant, Service: "sync", Scopes: mcp.AllScopes()}), "ext_package_list", json.RawMessage(`{}`))
	require.False(t, res.OK)

	// the engine's own audit names the tool and the real subject
	require.NotEmpty(t, e.audit.entries)
	last := e.audit.entries[len(e.audit.entries)-1]
	require.Equal(t, "ext_package_list", last.Tool)
}

func TestDraftEditingDiagnosticsAndConcurrencyOverMCP(t *testing.T) {
	e := newEnv(t)
	const ns = "acme.edit"
	e.ok(adminID, "ext_package_create", map[string]string{"namespace": ns}, nil)
	for p, c := range files(t, ns, "1.0.0") {
		e.ok(adminID, "ext_file_write", map[string]any{"namespace": ns, "path": p, "content": c}, nil)
	}
	e.ok(adminID, "ext_file_write", map[string]any{"namespace": ns, "path": "docs/new.md", "content": "# new", "create_only": true}, nil)
	// create_only on an existing file is a conflict
	e.failed(adminID, "ext_file_write", map[string]any{"namespace": ns, "path": "server.mjs", "content": "y", "create_only": true}, mcp.CodeConflict)
	e.failed(adminID, "ext_file_write", map[string]any{"namespace": ns, "path": "a.txt", "content": "y", "create_only": true, "expected_hash": "h"}, mcp.CodeInvalidInput)

	var read struct {
		Path, Hash, Encoding, Content string
		Size                          int
	}
	e.ok(readerID, "ext_file_read", map[string]string{"namespace": ns, "path": "server.mjs"}, &read)
	require.Equal(t, "utf-8", read.Encoding)
	require.Contains(t, read.Content, "renew")
	// optimistic concurrency: the stale hash loses
	e.ok(adminID, "ext_file_write", map[string]any{"namespace": ns, "path": "server.mjs", "content": "export const renew = 2\n", "expected_hash": read.Hash}, nil)
	e.failed(adminID, "ext_file_write", map[string]any{"namespace": ns, "path": "server.mjs", "content": "export const renew = 3\n", "expected_hash": read.Hash}, mcp.CodeConflict)
	// binary content round-trips through base64
	e.ok(adminID, "ext_file_write", map[string]any{"namespace": ns, "path": "logo.bin", "content": base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe, 0x00}), "encoding": "base64"}, nil)
	e.ok(readerID, "ext_file_read", map[string]string{"namespace": ns, "path": "logo.bin"}, &read)
	require.Equal(t, "base64", read.Encoding)

	// publish is refused with the diagnostics as issues pointing at file:line
	f := e.failed(adminID, "ext_publish", map[string]string{"namespace": ns, "version": "1.0.0"}, mcp.CodeConflict)
	require.NotEmpty(t, f.Issues)
	require.Equal(t, "src/bad.ts:1", f.Issues[0].Path)
	require.Equal(t, "ts.2345", f.Issues[0].Code)

	e.ok(adminID, "ext_file_move", map[string]string{"namespace": ns, "from": "src/bad.ts", "to": "src/worse.ts"}, nil)
	e.ok(adminID, "ext_file_delete", map[string]string{"namespace": ns, "path": "src/worse.ts"}, nil)
	var list struct{ Files []manage.File }
	e.ok(readerID, "ext_file_list", map[string]string{"namespace": ns}, &list)
	require.NotEmpty(t, list.Files)
	var pk manage.Package
	e.ok(readerID, "ext_package_get", map[string]string{"namespace": ns}, &pk)
	require.Equal(t, ns, pk.Namespace)
	e.failed(readerID, "ext_package_get", map[string]string{"namespace": "acme.none"}, mcp.CodeNotFound)

	// invalid input is refused by the schema before the service runs
	e.failed(adminID, "ext_file_write", map[string]any{"namespace": ns, "path": "x"}, mcp.CodeInvalidInput)
	e.failed(adminID, "ext_file_write", map[string]any{"namespace": ns, "path": "x", "content": "", "bogus": true}, mcp.CodeInvalidInput)

	// versions, activations, bindings, owners, kit and audit
	var vs struct{ Versions []manage.Version }
	e.ok(readerID, "ext_version_list", map[string]string{"namespace": ns}, &vs)
	require.Empty(t, vs.Versions)
	var kit struct {
		Hash    string
		Modules []struct {
			Name         string
			DownloadPath string `json:"download_path"`
		}
	}
	e.ok(readerID, "ext_kit_info", struct{}{}, &kit)
	require.NotEmpty(t, kit.Hash)
	require.NotEmpty(t, kit.Modules)
	var tgz struct {
		DataBase64 string `json:"data_base64"`
		Filename   string
	}
	e.ok(readerID, "ext_kit_module", map[string]string{"module": "core"}, &tgz)
	require.NotEmpty(t, tgz.DataBase64)
	var binds struct{ Bindings []manage.Binding }
	e.ok(readerID, "ext_binding_list", struct{}{}, &binds)
	var owners struct{ Owners []manage.TriggerOwner }
	e.ok(readerID, "ext_owner_list", struct{}{}, &owners)
	var au struct{ Entries []manage.AuditEntry }
	e.ok(readerID, "ext_audit_list", map[string]any{"namespace": ns, "action": manage.ActionPutFile}, &au)
	require.NotEmpty(t, au.Entries)
	for _, a := range au.Entries {
		require.Equal(t, "mcp:test-client", a.Credential)
		require.EqualValues(t, adminID, a.ActorUserID)
	}

	// workflows need the engine, which this env does not attach: the same failed precondition as HTTP
	e.failed(readerID, "ext_workflow_list", struct{}{}, mcp.CodeConflict)
	e.failed(adminID, "ext_workflow_get", map[string]string{"id": "not-a-uuid"}, mcp.CodeInvalidInput)
	e.failed(adminID, "ext_workflow_terminate", map[string]string{"id": uuid.NewString()}, mcp.CodeConflict)
	e.failed(readerID, "ext_workflow_terminate", map[string]string{"id": uuid.NewString()}, mcp.CodeForbidden)
	e.failed(adminID, "ext_workflow_signal", map[string]any{"id": uuid.NewString(), "name": "go", "payload": map[string]any{"a": 1}}, mcp.CodeConflict)
	e.failed(adminID, "ext_workflow_start_v2", map[string]any{"id": uuid.NewString()}, mcp.CodeConflict)
	e.failed(adminID, "ext_workflow_resume", map[string]any{"id": uuid.NewString()}, mcp.CodeConflict)
	e.failed(adminID, "ext_workflow_retry", map[string]any{"id": uuid.NewString(), "name": "step", "n": 0}, mcp.CodeConflict)
	e.failed(adminID, "ext_workflow_resolve", map[string]any{"id": uuid.NewString(), "name": "step", "n": 0, "result": 1}, mcp.CodeConflict)
	e.failed(adminID, "ext_workflow_decide_task", map[string]any{"task_id": uuid.NewString(), "decision": "approve"}, mcp.CodeConflict)
}

func TestResourceDiscoveryDocumentsContracts(t *testing.T) {
	e := newEnv(t)
	const ns = "acme.catalog"
	e.ok(adminID, "ext_package_create", map[string]string{"namespace": ns}, nil)
	for p, c := range files(t, ns, "1.0.0") {
		if p == "src/bad.ts" {
			continue
		}
		e.ok(adminID, "ext_file_write", map[string]string{"namespace": ns, "path": p, "content": c}, nil)
	}
	e.ok(adminID, "ext_publish", map[string]string{"namespace": ns, "version": "1.0.0"}, nil)
	e.ok(adminID, "ext_review", map[string]any{"namespace": ns, "version": "1.0.0", "approve": true}, nil)
	e.ok(adminID, "ext_activate", map[string]string{"namespace": ns, "version": "1.0.0"}, nil)

	var list struct {
		Resources []struct {
			ID, Kind, Package, Description string
			Native                         bool
		}
	}
	e.ok(readerID, "ext_resource_list", map[string]string{"package": ns}, &list)
	require.Len(t, list.Resources, 2)
	e.ok(readerID, "ext_resource_list", map[string]string{"query": "renews a policy"}, &list)
	require.Len(t, list.Resources, 1)
	require.Equal(t, ns+".renewPolicy", list.Resources[0].ID)
	e.ok(readerID, "ext_resource_list", map[string]string{"package": "native", "kind": "entity"}, &list)
	require.NotEmpty(t, list.Resources, "native (core) entities are discoverable too")
	for _, r := range list.Resources {
		require.True(t, r.Native)
	}

	var doc struct {
		ID, Description, Permission string
		Invocable                   bool
		InputSchema                 map[string]any `json:"input_schema"`
		OutputSchema                map[string]any `json:"output_schema"`
		RowSchema                   map[string]any `json:"row_schema"`
	}
	e.ok(readerID, "ext_resource_get", map[string]string{"id": ns + ".renewPolicy"}, &doc)
	require.True(t, doc.Invocable)
	require.Equal(t, "Policy.Renew", doc.Permission)
	require.Equal(t, "object", doc.InputSchema["type"])
	require.Equal(t, []any{"number"}, doc.InputSchema["required"])
	require.Contains(t, doc.InputSchema["properties"], "note")
	require.Contains(t, doc.OutputSchema["properties"], "ok")
	e.ok(readerID, "ext_resource_get", map[string]string{"id": ns + ".policy"}, &doc)
	require.False(t, doc.Invocable)
	require.Contains(t, doc.RowSchema["properties"], "number")
	e.failed(readerID, "ext_resource_get", map[string]string{"id": ns + ".nope"}, mcp.CodeNotFound)
	e.failed(readerID, "ext_resource_get", map[string]string{"id": ns + ".policy", "x": "y"}, mcp.CodeInvalidInput)
}

func TestInvokeRunsAsTheCallingUserThroughTheHost(t *testing.T) {
	e := newEnv(t)
	const ns = "acme.invoke"
	e.ok(adminID, "ext_package_create", map[string]string{"namespace": ns}, nil)
	for p, c := range files(t, ns, "1.0.0") {
		if p != "src/bad.ts" {
			e.ok(adminID, "ext_file_write", map[string]string{"namespace": ns, "path": p, "content": c}, nil)
		}
	}
	e.ok(adminID, "ext_publish", map[string]string{"namespace": ns, "version": "1.0.0"}, nil)
	e.ok(adminID, "ext_review", map[string]any{"namespace": ns, "version": "1.0.0", "approve": true}, nil)
	e.ok(adminID, "ext_activate", map[string]string{"namespace": ns, "version": "1.0.0"}, nil)

	e.invoke.result = host.Result{InvocationID: "inv-1", Output: json.RawMessage(`{"ok":true}`), Binding: host.Binding{Package: ns, Version: "1.0.0"}, ContractHash: "h"}
	var out struct {
		InvocationID string          `json:"invocation_id"`
		Output       json.RawMessage `json:"output"`
		Package      string
	}
	ctx := mcp.WithRequestID(e.principal(readerID, mcp.AllScopes()), "req-7")
	res := e.callCtx(ctx, "ext_invoke", map[string]any{"resource": ns + ".renewPolicy", "input": map[string]string{"number": "P-1"}})
	require.True(t, res.OK, "%+v", res.Error)
	require.NoError(t, json.Unmarshal(res.Data, &out))
	require.Equal(t, "inv-1", out.InvocationID)
	require.JSONEq(t, `{"ok":true}`, string(out.Output))
	require.Equal(t, ns, out.Package)

	require.Len(t, e.invoke.calls, 1)
	req := e.invoke.calls[0].req
	require.Equal(t, ns+".renewPolicy", req.Contract)
	require.JSONEq(t, `{"number":"P-1"}`, string(req.Input))
	require.EqualValues(t, readerID, req.Subject.UserID, "the host acts as the calling user, never as a model-supplied identity")
	require.Equal(t, e.tenant, req.Subject.TenantID)
	require.Equal(t, spi.SubjectInitiator, req.Subject.Kind)
	require.Equal(t, "mcp:test-client", req.Subject.Credential)
	require.Equal(t, []string{manage.PermRead}, req.Subject.Permissions)
	require.Equal(t, host.TriggerMCP, req.Trigger.Kind)
	require.Contains(t, req.Trigger.Ref, "req-7")

	// the model cannot name a user: the schema has no such field
	e.failed(readerID, "ext_invoke", map[string]any{"resource": ns + ".renewPolicy", "user_id": 1}, mcp.CodeInvalidInput)
	// entities are not callable
	f := e.failed(readerID, "ext_invoke", map[string]any{"resource": ns + ".policy"}, mcp.CodeInvalidInput)
	require.Equal(t, "kind", f.Issues[0].Code)
	e.failed(readerID, "ext_invoke", map[string]any{"resource": ns + ".missing"}, mcp.CodeNotFound)
	require.Len(t, e.invoke.calls, 1)

	// the host's refusals and the package's typed errors keep their meaning
	e.invoke.err = &pool.WireError{Kind: pool.ErrKindDenied, Code: "runtime.permissionDenied", Message: "permission Policy.Renew is required for " + ns + ".renewPolicy"}
	f = e.failed(readerID, "ext_invoke", map[string]any{"resource": ns + ".renewPolicy", "input": map[string]string{"number": "P-1"}}, mcp.CodeForbidden)
	require.Contains(t, f.Message, "Policy.Renew")
	e.invoke.err = &pool.WireError{Kind: pool.ErrKindValidation, Issues: []pool.Issue{{Code: "policy.expired", Path: json.RawMessage(`["number"]`), Params: json.RawMessage(`{"on":"2025-01-01"}`)}}}
	f = e.failed(readerID, "ext_invoke", map[string]any{"resource": ns + ".renewPolicy", "input": map[string]string{"number": "P-1"}}, mcp.CodeInvalidInput)
	require.Equal(t, "policy.expired", f.Issues[0].Code)
	e.invoke.err = host.ErrNoBinding
	e.failed(readerID, "ext_invoke", map[string]any{"resource": ns + ".renewPolicy", "input": map[string]string{"number": "P-1"}}, mcp.CodeConflict)

	// a read-only or manage-only grant cannot invoke
	res = e.callCtx(e.principal(readerID, mcp.GrantedScopes(mcptools.ScopeRead, mcptools.ScopeManage)), "ext_invoke", map[string]any{"resource": ns + ".renewPolicy"})
	require.False(t, res.OK)
	assert.Equal(t, mcp.CodeForbidden, res.Error.Code)
}
