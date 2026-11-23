package lookup_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/execute"
	"github.com/iota-uz/iota-sdk/pkg/mcp/lookup"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/lifecycle"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/manager"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/pykerneltest"
)

const (
	permReport = "Report.Read"
	permSecret = "Secret.Read"
	permWrite  = "Plan.Write"
)

type reportIn struct {
	Report string `json:"report"`
	From   string `json:"from,omitempty"`
}

type reportOut struct {
	Report string   `json:"report"`
	Tenant string   `json:"tenant"`
	Rows   []string `json:"rows"`
}

type prepareOut struct {
	PlanID string `json:"plan_id"`
}

type fixture struct {
	engine   *mcp.Engine
	registry *mcp.Registry
	granted  map[string]bool
	prepared *atomic.Int32
	selected *atomic.Value
}

// newFixture registers an application catalog the way an app does: its
// operations, describe with app selectors, and the two entry points.
func newFixture(t *testing.T, withRuntime bool) *fixture {
	t.Helper()
	f := &fixture{
		registry: mcp.NewRegistry(),
		granted:  map[string]bool{permReport: true, permWrite: true},
		prepared: &atomic.Int32{},
		selected: &atomic.Value{},
	}
	require.NoError(t, f.registry.Register(
		mcp.MustNew(mcp.Spec{
			Name: "report", Description: "A finance report. Numbers match the dashboard.",
			Annotations: mcp.Annotations{Title: "Report", ReadOnly: true, Idempotent: true},
			Policy:      mcp.Policy{Scopes: []string{"app:read"}, Permissions: []string{permReport}},
		}, func(ctx context.Context, in reportIn) (reportOut, error) {
			if in.Report == "missing" {
				return reportOut{}, mcp.Fail(mcp.CodeNotFound, mcp.RecoveryFixInput, "no such report")
			}
			p, _ := mcp.PrincipalFrom(ctx)
			return reportOut{Report: in.Report, Tenant: p.TenantID.String(), Rows: []string{"a", "b"}}, nil
		}),
		mcp.MustNew(mcp.Spec{
			Name: "secret", Description: "Hidden from this account.",
			Annotations: mcp.Annotations{ReadOnly: true},
			Policy:      mcp.Policy{Permissions: []string{permSecret}},
		}, func(context.Context, struct{}) (struct{}, error) { return struct{}{}, nil }),
		mcp.MustNew(mcp.Spec{
			Name: "prepare_changes", Description: "Prepare a plan for a human to approve.",
			Policy: mcp.Policy{Scopes: []string{"app:write"}, Permissions: []string{permWrite}},
		}, func(context.Context, struct{}) (prepareOut, error) {
			f.prepared.Add(1)
			return prepareOut{PlanID: "plan-1"}, nil
		}),
	))
	engine, err := mcp.NewEngine(f.registry, mcp.AuthorizerFunc(func(ctx context.Context, p mcp.Principal, policy mcp.Policy) (context.Context, error) {
		if missing := p.Scopes.Missing(policy.Scopes); len(missing) > 0 {
			return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "missing scope")
		}
		for _, perm := range policy.Permissions {
			if !f.granted[perm] {
				return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryRequestAccess, "missing permission "+perm)
			}
		}
		return ctx, nil
	}))
	require.NoError(t, err)
	f.engine = engine

	describe, err := lookup.Describe(engine, lookup.DescribeConfig{
		Policy:    mcp.Policy{Scopes: []string{"app:read"}},
		Selectors: map[string]*jsonschema.Schema{"report": {Type: "string"}},
		Select: func(_ context.Context, sel map[string]json.RawMessage) (any, error) {
			f.selected.Store(string(sel["report"]))
			return map[string]any{"report": sel["report"], "params": []string{"from"}}, nil
		},
		Catalog: func(context.Context) (any, error) { return map[string]any{"reports": []string{"pnl"}}, nil },
	})
	require.NoError(t, err)
	entry, err := lookup.New(engine, lookup.Config{Policy: mcp.Policy{Scopes: []string{"app:read"}}, About: "Test ERP."})
	require.NoError(t, err)
	require.NoError(t, f.registry.Register(describe, entry))

	cfg := execute.Config{
		Engine: engine, WorkRoot: t.TempDir(),
		Limits: execute.Limits{
			DefaultTimeout: 10 * time.Second, MaxTimeout: 30 * time.Second,
			DefaultOutputBytes: 8192, MaxOutputBytes: 16384, MaxCodeBytes: 10_000,
		},
		Policy: mcp.Policy{Scopes: []string{"app:read"}},
	}
	if withRuntime {
		if _, err := exec.LookPath("python3"); err != nil {
			t.Skip("python3 not available")
		}
		m, err := manager.New(manager.Config{
			Policy: lifecycle.WarmPool(time.Minute, 4), Sandbox: pykerneltest.Sandbox(t),
			EnvAllowlist: []string{"PATH"}, DisposeGrace: time.Second,
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
		cfg.Manager = m
	}
	runner, err := execute.NewRunner(cfg)
	require.NoError(t, err)
	require.NoError(t, f.registry.Register(runner.Tool()))
	return f
}

func caller(tenant uuid.UUID, scopes ...string) context.Context {
	return mcp.WithPrincipal(context.Background(), mcp.Principal{
		TenantID: tenant, UserID: 7, Scopes: mcp.GrantedScopes(scopes...),
	})
}

func (f *fixture) lookup(t *testing.T, ctx context.Context, method string, input any) mcp.Result {
	t.Helper()
	args := map[string]any{"method": method}
	if input != nil {
		args["input"] = input
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	return f.engine.Call(ctx, lookup.ToolName, raw)
}

func (f *fixture) execute(t *testing.T, ctx context.Context, code string) execute.Result {
	t.Helper()
	raw, err := json.Marshal(execute.Input{Code: code})
	require.NoError(t, err)
	res := f.engine.Call(ctx, execute.ToolName, raw)
	require.True(t, res.OK, "execute failed: %+v", res.Error)
	var out execute.Result
	require.NoError(t, json.Unmarshal(res.Data, &out))
	require.Equal(t, execute.StatusSucceeded, out.Status, "run failed: %+v %s", out.Error, out.Stderr)
	return out
}

func failure(t *testing.T, res mcp.Result) *mcp.Failure {
	t.Helper()
	require.False(t, res.OK, "expected a failure, got %s", res.Data)
	return res.Error
}

func TestRegistry_PublishesOnlyEntryPoints(t *testing.T) {
	f := newFixture(t, false)
	names := []string{}
	for _, spec := range f.registry.Published() {
		names = append(names, spec.Name)
		assert.True(t, spec.HostOnly, "entry point %s must stay out of the Python namespace", spec.Name)
	}
	assert.Equal(t, []string{"execute", "lookup"}, names)
}

func TestLookup_RunsReadMethodThroughEngine(t *testing.T) {
	f := newFixture(t, false)
	ctx := caller(uuid.New(), "app:read")

	res := f.lookup(t, ctx, "report", map[string]any{"report": "pnl"})
	require.True(t, res.OK, "%+v", res.Error)
	assert.JSONEq(t, `{"report":"pnl","tenant":"`+mustPrincipal(t, ctx).TenantID.String()+`","rows":["a","b"]}`, string(res.Data))

	// An operation failure is passed through unchanged in the shared Result.
	assert.Equal(t, mcp.CodeNotFound, failure(t, f.lookup(t, ctx, "report", map[string]any{"report": "missing"})).Code)
}

func TestLookup_Rejections(t *testing.T) {
	f := newFixture(t, false)
	ctx := caller(uuid.New(), "app:read", "app:write")

	cases := []struct {
		name   string
		method string
		input  any
		code   mcp.Code
		issue  string
	}{
		{name: "unknown method", method: "nope", input: map[string]any{}, code: mcp.CodeNotFound, issue: "/method"},
		{name: "method outside the caller's rights", method: "secret", input: map[string]any{}, code: mcp.CodeNotFound, issue: "/method"},
		{name: "entry point is not a method", method: "execute", input: map[string]any{"code": "1"}, code: mcp.CodeNotFound, issue: "/method"},
		{name: "lookup cannot recurse", method: "lookup", input: map[string]any{"method": "report", "input": map[string]any{}}, code: mcp.CodeNotFound, issue: "/method"},
		{name: "write method is refused before its handler", method: "prepare_changes", input: map[string]any{}, code: mcp.CodeInvalidInput, issue: "/method"},
		{name: "input is required", method: "report", input: nil, code: mcp.CodeInvalidInput, issue: ""},
		{name: "input must match the method schema", method: "report", input: map[string]any{"report": 1}, code: mcp.CodeInvalidInput, issue: "/report"},
		{name: "unknown arguments are refused", method: "describe", input: map[string]any{"bogus": true}, code: mcp.CodeInvalidInput, issue: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := failure(t, f.lookup(t, ctx, tc.method, tc.input))
			assert.Equal(t, tc.code, got.Code, got.Message)
			if tc.issue != "" {
				require.NotEmpty(t, got.Issues)
				assert.Equal(t, tc.issue, got.Issues[0].Path)
			}
		})
	}
	assert.Zero(t, f.prepared.Load(), "lookup must never reach a write handler")

	side := failure(t, f.lookup(t, ctx, "prepare_changes", map[string]any{}))
	assert.Contains(t, side.Message, "execute")
	assert.Equal(t, "side_effects", side.Issues[0].Code)
}

func TestLookup_RequiresItsOwnPolicyAndIdentity(t *testing.T) {
	f := newFixture(t, false)
	assert.Equal(t, mcp.CodeForbidden, failure(t, f.lookup(t, caller(uuid.New()), "describe", map[string]any{})).Code)
	assert.Equal(t, mcp.CodeForbidden, failure(t, f.lookup(t, context.Background(), "describe", map[string]any{})).Code)
}

func TestDescribe_CatalogFollowsTheCaller(t *testing.T) {
	f := newFixture(t, false)

	readOnly := decodeCatalog(t, f.lookup(t, caller(uuid.New(), "app:read"), "describe", map[string]any{}))
	assert.Equal(t, []string{"describe", "report"}, operationNames(readOnly), "a read-only grant sees no writes")
	report := operationNamed(readOnly, "report")
	assert.Equal(t, "A finance report.", report.Summary)
	assert.Equal(t, []string{"lookup", "execute"}, report.Via)
	assert.False(t, report.SideEffects)
	assert.NotEmpty(t, readOnly.Usage.Execute)
	assert.JSONEq(t, `{"reports":["pnl"]}`, mustJSON(t, readOnly.Domain))

	writer := decodeCatalog(t, f.lookup(t, caller(uuid.New(), "app:read", "app:write"), "describe", map[string]any{}))
	assert.Equal(t, []string{"describe", "prepare_changes", "report"}, operationNames(writer))
	prepare := operationNamed(writer, "prepare_changes")
	assert.True(t, prepare.SideEffects)
	assert.Equal(t, []string{"execute"}, prepare.Via)

	// RBAC is read on every call: a revoked permission leaves the catalog.
	f.granted[permReport] = false
	revoked := decodeCatalog(t, f.lookup(t, caller(uuid.New(), "app:read"), "describe", map[string]any{}))
	assert.Equal(t, []string{"describe"}, operationNames(revoked))
}

func TestDescribe_OneOperationAndSelectors(t *testing.T) {
	f := newFixture(t, false)
	ctx := caller(uuid.New(), "app:read", "app:write")

	res := f.lookup(t, ctx, "describe", map[string]any{"tool": "report"})
	require.True(t, res.OK, "%+v", res.Error)
	var detail lookup.Detail
	require.NoError(t, json.Unmarshal(res.Data, &detail))
	assert.Equal(t, "report", detail.Name)
	assert.Equal(t, "A finance report. Numbers match the dashboard.", detail.Description)
	require.NotNil(t, detail.InputSchema)
	assert.Contains(t, detail.InputSchema.Properties, "report")
	require.NotNil(t, detail.OutputSchema)
	assert.Contains(t, detail.OutputSchema.Properties, "rows")

	for _, hidden := range []string{"secret", "lookup", "execute", "nope"} {
		got := failure(t, f.lookup(t, ctx, "describe", map[string]any{"tool": hidden}))
		assert.Equal(t, mcp.CodeNotFound, got.Code, hidden)
	}

	sel := f.lookup(t, ctx, "describe", map[string]any{"report": "pnl"})
	require.True(t, sel.OK, "%+v", sel.Error)
	assert.JSONEq(t, `{"report":"pnl","params":["from"]}`, string(sel.Data))
	assert.Equal(t, `"pnl"`, f.selected.Load())

	both := failure(t, f.lookup(t, ctx, "describe", map[string]any{"tool": "report", "report": "pnl"}))
	assert.Equal(t, mcp.CodeInvalidInput, both.Code)
}

func TestDescribe_WithoutExecuteListsOnlyReads(t *testing.T) {
	reg := mcp.NewRegistry()
	engine, err := mcp.NewEngine(reg, mcp.AuthorizerFunc(func(ctx context.Context, _ mcp.Principal, _ mcp.Policy) (context.Context, error) {
		return ctx, nil
	}))
	require.NoError(t, err)
	describe, err := lookup.Describe(engine, lookup.DescribeConfig{})
	require.NoError(t, err)
	entry, err := lookup.New(engine, lookup.Config{})
	require.NoError(t, err)
	require.NoError(t, reg.Register(describe, entry, mcp.MustNew(mcp.Spec{Name: "write"},
		func(context.Context, struct{}) (struct{}, error) { return struct{}{}, nil })))

	res := engine.Call(caller(uuid.New()), lookup.ToolName, json.RawMessage(`{"method":"describe","input":{}}`))
	catalog := decodeCatalog(t, res)
	assert.Equal(t, []string{"describe"}, operationNames(catalog))
	assert.Empty(t, catalog.Usage.Execute)
}

func TestDescribe_ConfigValidation(t *testing.T) {
	engine, err := mcp.NewEngine(mcp.NewRegistry(), mcp.AuthorizerFunc(func(ctx context.Context, _ mcp.Principal, _ mcp.Policy) (context.Context, error) {
		return ctx, nil
	}))
	require.NoError(t, err)
	_, err = lookup.Describe(engine, lookup.DescribeConfig{Selectors: map[string]*jsonschema.Schema{"report": {Type: "string"}}})
	require.Error(t, err)
	_, err = lookup.Describe(engine, lookup.DescribeConfig{
		Selectors: map[string]*jsonschema.Schema{"tool": {Type: "string"}},
		Select:    func(context.Context, map[string]json.RawMessage) (any, error) { return struct{}{}, nil },
	})
	require.Error(t, err)
	_, err = lookup.Describe(nil, lookup.DescribeConfig{})
	require.Error(t, err)
	_, err = lookup.New(nil, lookup.Config{})
	require.Error(t, err)
}

// The same read returns the same Result through lookup and inside execute,
// and execute still reaches writes that lookup refuses.
func TestLookup_ResultParityWithExecute(t *testing.T) {
	f := newFixture(t, true)
	ctx := caller(uuid.New(), "app:read", "app:write")

	direct := f.lookup(t, ctx, "report", map[string]any{"report": "pnl", "from": "2026-01-01"})
	missing := f.lookup(t, ctx, "report", map[string]any{"report": "missing"})
	described := f.lookup(t, ctx, "describe", map[string]any{"tool": "report"})

	out := f.execute(t, ctx, `
r = await tools.report(report="pnl", **{"from": "2026-01-01"})
m = await tools.report(report="missing")
d = await tools.describe(tool="report")
w = await tools.prepare_changes()
emit(r)
emit(m)
emit(d)
emit(w)
`)
	require.Len(t, out.Emitted, 4)
	assert.JSONEq(t, mustJSON(t, direct), string(out.Emitted[0]))
	assert.JSONEq(t, mustJSON(t, missing), string(out.Emitted[1]))
	assert.JSONEq(t, mustJSON(t, described), string(out.Emitted[2]))
	assert.JSONEq(t, `{"ok":true,"data":{"plan_id":"plan-1"}}`, string(out.Emitted[3]))
	assert.EqualValues(t, 1, f.prepared.Load())

	// Python sees neither entry point.
	hidden := f.execute(t, ctx, `emit([hasattr(tools, "lookup"), hasattr(tools, "execute")])`)
	assert.JSONEq(t, `[false,false]`, string(hidden.Emitted[0]))
}

func mustPrincipal(t *testing.T, ctx context.Context) mcp.Principal {
	t.Helper()
	p, ok := mcp.PrincipalFrom(ctx)
	require.True(t, ok)
	return p
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

func decodeCatalog(t *testing.T, res mcp.Result) lookup.Catalog {
	t.Helper()
	require.True(t, res.OK, "%+v", res.Error)
	var c lookup.Catalog
	require.NoError(t, json.Unmarshal(res.Data, &c))
	return c
}

func operationNames(c lookup.Catalog) []string {
	names := make([]string, 0, len(c.Operations))
	for _, op := range c.Operations {
		names = append(names, op.Name)
	}
	slices.Sort(names)
	return names
}

func operationNamed(c lookup.Catalog, name string) lookup.Operation {
	for _, op := range c.Operations {
		if op.Name == name {
			return op
		}
	}
	return lookup.Operation{}
}
