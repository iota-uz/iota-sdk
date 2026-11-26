package mcpops_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	autoperms "github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/automation/backtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/automation/mcpops"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
	"github.com/iota-uz/iota-sdk/pkg/mcp/lookup"
	"github.com/iota-uz/iota-sdk/pkg/mcp/rbacauth"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type client struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

const (
	evClient   = "acme.client.created.v1"
	permClient = "Client.Read"
)

var clientCreated = native.Event[client](evClient, native.EventSpec{Subject: "acme.client", Sensitive: map[string][]string{"phone": {permClient}}})

type nativeEvents []native.EventDefinition

func (n nativeEvents) Events() []native.EventDefinition { return n }

type specNode struct{ spec graph.NodeSpec }

func (n specNode) Spec() graph.NodeSpec              { return n.spec }
func (n specNode) Requires() []catalog.ConnectorKind { return nil }

// noteNode records its text: an effect in the step's transaction.
type noteNode struct{ specNode }

func (noteNode) ApplyEffect(_ context.Context, _ pgx.Tx, _ engine.StepContext, _ graph.Node, e graph.Effect) (map[string]any, error) {
	text, _ := e.Params["text"].(string)
	return map[string]any{"text": text}, nil
}

// approveNode is a human task.
type approveNode struct{ specNode }

func (approveNode) Human() bool { return true }

// taskTitle is the title of the test tasks.
const taskTitle = "Call +998900000001"

type tasks struct{}

func (tasks) Begin(_ context.Context, _ pgx.Tx, _ engine.StepContext, _ graph.Node, _ json.RawMessage) (engine.WaitSpec, error) {
	owner := int64(ownerID)
	// The title stands for one rendered from a sensitive field.
	return engine.WaitSpec{Kind: "task", Task: &workflow.TaskSpec{Title: manifest.Label{"en": taskTitle},
		Assignees: []workflow.TaskAssignee{{Kind: workflow.TaskAssignUser, UserID: &owner}}}}, nil
}

type urls struct{}

func (urls) Editor(id uuid.UUID, version int) string {
	return fmt.Sprintf("https://app.test/automations/%s?version=%d", id, version)
}

func (urls) Run(id, runID uuid.UUID) string {
	return fmt.Sprintf("https://app.test/automations/%s?tab=runs&run=%s", id, runID)
}

func (urls) Automations() string { return "https://app.test/automations" }

func (urls) Connections() string { return "https://app.test/settings/integrations" }

// links are the engine's run links: the run page of urls.
type links struct{}

func (links) SubjectURL(context.Context, uuid.UUID, engine.SubjectRef) (string, error) {
	return "", nil
}

func (links) RunURL(_, automationID, runID uuid.UUID) string { return urls{}.Run(automationID, runID) }

// counted is a call node that counts its real calls; a decision node when decides.
type counted struct {
	specNode
	calls *atomic.Int64
}

func (n counted) Idempotent() bool { return true }
func (n counted) Execute(context.Context, catalog.CallInput) (catalog.CallResult, error) {
	n.calls.Add(1)
	return catalog.CallResult{Outcome: "done", Output: map[string]any{"label": "real"}}, nil
}

type decides struct{ counted }

func (decides) DecisionKey(in catalog.CallInput) (string, bool) {
	text, _ := in.Params["text"].(string)
	return engine.DecisionKey("decide", text, "test-1", nil)
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

func (a *recordingAuditor) outcomes(tool string) []mcp.Code {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []mcp.Code
	for _, e := range a.entries {
		if e.Tool == tool {
			out = append(out, e.Outcome)
		}
	}
	return out
}

const (
	ownerID  = 1 // a person: every permission
	agentID  = 2 // the agent's user: reads, authors, reads clients
	blindID  = 3 // reads and authors, may not read clients
	readerID = 4 // reads only
)

var clientRead = permission.MustCreate(uuid.MustParse("6a1d0c5e-6f43-4f4b-9b55-3a9d2c1b7e01"), permClient, "client", permission.ActionRead, permission.ModifierAll)

type env struct {
	t        *testing.T
	db       *pgxpool.Pool
	tenant   uuid.UUID
	wf       *workflow.Engine
	rt       *engine.Runtime
	disp     *events.Dispatcher
	mcp      *mcp.Engine
	audit    *recordingAuditor
	changes  *changes.Service
	authz    mcp.Authorizer
	users    map[uint]user.User
	subjects *hosttest.Subjects
	reg      catalog.Registry
	decided  *atomic.Int64 // real calls of acme.decide
	// backtests is the backtest service whose tools are registered (mcpops.WithBacktests).
	backtests *backtest.Svc
	fetched   *atomic.Int64 // real calls of acme.fetch
}

func account(tenant uuid.UUID, id uint, perms ...permission.Permission) user.User {
	email, err := internet.NewEmail(fmt.Sprintf("u%d@example.com", id))
	if err != nil {
		panic(err)
	}
	r := role.New("staff", role.WithID(id), role.WithPermissions(perms), role.WithTenantID(tenant))
	return user.New("", "", email, "", user.WithID(id), user.WithTenantID(tenant), user.WithRoles([]role.Role{r}))
}

func newEnv(t *testing.T) *env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_mcpops",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	e := &env{t: t, db: scope.Pool, tenant: scope.Tenant.ID, subjects: hosttest.NewSubjects(), audit: &recordingAuditor{}, decided: &atomic.Int64{}, fetched: &atomic.Int64{}}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	all := []string{engine.PermRead, engine.PermAuthor, engine.PermEnable, engine.PermOperate, permClient}
	e.subjects.Grant(ownerID, all...)
	e.subjects.Grant(agentID, engine.PermRead, engine.PermAuthor, permClient)

	reg := catalog.NewRegistry(nil)
	require.NoError(t, reg.RegisterTriggers(engine.Triggers()))
	require.NoError(t, reg.RegisterTriggers(catalog.FromNativeEvents(nativeEvents{clientCreated})))
	for _, spec := range append(graph.BuiltinNodes(nil, []string{evClient}), graph.FlowNodes(nil)...) {
		require.NoError(t, reg.RegisterNode(specNode{spec}))
	}
	require.NoError(t, reg.RegisterNode(noteNode{specNode{graph.NodeSpec{Type: "acme.note", Title: "Note", Behavior: graph.BehaviorEffect,
		DryRun: graph.DryRunCapture, FailedOutcome: "failed", Outcomes: []string{"done", "failed"},
		Params:    graph.MustSchemaParams(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"],"additionalProperties":false}`),
		Templates: []string{"text"}, Output: expr.Fields{{Name: "text", Field: expr.StringField}}}}}))
	require.NoError(t, reg.RegisterNode(approveNode{specNode{graph.NodeSpec{Type: "acme.approve", Title: "Approve", Behavior: graph.BehaviorCall,
		DryRun: graph.DryRunCapture, Outcomes: []string{"approve", "reject"},
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}}}))

	callSpec := func(typ string) graph.NodeSpec {
		return graph.NodeSpec{Type: typ, Title: typ, Behavior: graph.BehaviorCall, DryRun: graph.DryRunExecute,
			Params:   graph.MustSchemaParams(`{"type":"object","properties":{"text":{"type":"string"}},"additionalProperties":false}`),
			Outcomes: []string{"done", "failed"}, FailedOutcome: "failed", Templates: []string{"text"},
			Output: expr.Fields{{Name: "label", Field: expr.OptionalOf(expr.StringField)}}}
	}
	require.NoError(t, reg.RegisterNode(decides{counted{specNode{callSpec("acme.decide")}, e.decided}}))
	require.NoError(t, reg.RegisterNode(counted{specNode{callSpec("acme.fetch")}, e.fetched}))
	e.reg = reg

	wf, err := workflow.New(workflow.Config{
		DB: e.db, Subjects: e.subjects,
		NativeSource: func() (map[string]workflow.NativeContract, error) { return e.rt.Natives(), nil },
		LeaseTTL:     3 * time.Second, Heartbeat: time.Second, PollInterval: 50 * time.Millisecond, RetryBackoff: 10 * time.Millisecond,
		MaxRunFailures: 20, Logger: logger,
	})
	require.NoError(t, err)
	t.Cleanup(wf.Shutdown)
	rt, err := engine.NewRuntime(engine.Config{DB: e.db, Workflows: wf, Users: e.subjects, Catalog: reg, Tasks: tasks{}, Links: links{}, Logger: logger})
	require.NoError(t, err)
	e.wf, e.rt = wf, rt
	d, err := events.NewDispatcher(events.Config{
		DB: e.db, Consumers: events.StaticConsumers(rt.Consumer()), Subjects: e.subjects, PollInterval: 50 * time.Millisecond,
		BackoffBase: time.Millisecond, BackoffMax: 2 * time.Millisecond, MaxAttempts: 3, BlockedRecheck: time.Millisecond, Logger: logger,
	})
	require.NoError(t, err)
	t.Cleanup(d.Stop)
	e.disp = d

	e.users = map[uint]user.User{
		ownerID: account(e.tenant, ownerID, autoperms.AutomationsRead, autoperms.AutomationsAuthor, autoperms.AutomationsEnable, autoperms.AutomationsOperate, clientRead,
			corePerms.ExtensionsRead, corePerms.ExtensionsOperate),
		agentID:  account(e.tenant, agentID, autoperms.AutomationsRead, autoperms.AutomationsAuthor, clientRead),
		blindID:  account(e.tenant, blindID, autoperms.AutomationsRead, autoperms.AutomationsAuthor),
		readerID: account(e.tenant, readerID, autoperms.AutomationsRead),
	}
	authz := rbacauth.New(func(_ context.Context, tenant uuid.UUID, id uint) (user.User, error) {
		u, ok := e.users[id]
		if !ok || tenant != e.tenant {
			return nil, fmt.Errorf("unknown account")
		}
		return u, nil
	})
	e.authz = authz
	registry := mcp.NewRegistry()
	e.backtests, err = backtest.New(backtest.Config{DB: e.db, Automations: rt.Service(), Authoring: rt.Authoring(), Catalog: reg, Users: e.subjects, Logger: logger})
	require.NoError(t, err)
	tools, err := mcpops.Tools(rt.Service(), rt.Authoring(), reg, urls{}, mcpops.WithMemory(rt.Memory()), mcpops.WithBacktests(e.backtests))
	require.NoError(t, err)
	require.NoError(t, registry.Register(tools...))
	e.changes, err = changes.NewService(changes.Config{
		Store: changes.NewPostgresStore(e.db), Authorizer: authz,
		Kinds:       mcpops.Kinds(rt.Service(), rt.Authoring(), urls{}, mcpops.WithMemory(rt.Memory())),
		ApprovalURL: func(id string) string { return "https://app.test/approvals/" + id },
	})
	require.NoError(t, err)
	require.NoError(t, registry.Register(e.changes.Tools()...))
	e.mcp, err = mcp.NewEngine(registry, authz, mcp.WithAuditor(e.audit))
	require.NoError(t, err)
	lk, err := lookup.New(e.mcp, lookup.Config{})
	require.NoError(t, err)
	require.NoError(t, registry.Register(lk))
	return e
}

func (e *env) ctx(id uint, scopes mcp.Scopes) context.Context {
	return mcp.WithPrincipal(context.Background(), mcp.Principal{TenantID: e.tenant, UserID: id, ClientID: "test-agent", Scopes: scopes})
}

func (e *env) callAs(ctx context.Context, tool string, input any) mcp.Result {
	e.t.Helper()
	raw, err := json.Marshal(input)
	require.NoError(e.t, err)
	return e.mcp.Call(ctx, tool, raw)
}

func (e *env) call(id uint, tool string, input any) mcp.Result {
	e.t.Helper()
	return e.callAs(e.ctx(id, mcp.AllScopes()), tool, input)
}

func (e *env) ok(id uint, tool string, input any, into any) {
	e.t.Helper()
	res := e.call(id, tool, input)
	require.True(e.t, res.OK, "%s: %+v", tool, res.Error)
	if into != nil {
		require.NoError(e.t, json.Unmarshal(res.Data, into))
	}
}

func (e *env) failure(id uint, tool string, input any, code mcp.Code) *mcp.Failure {
	e.t.Helper()
	res := e.call(id, tool, input)
	require.False(e.t, res.OK, "%s should fail: %s", tool, res.Data)
	require.Equal(e.t, code, res.Error.Code, "%s: %+v", tool, res.Error)
	return res.Error
}

func (e *env) human() engine.Actor {
	return engine.Actor{TenantID: e.tenant, UserID: ownerID, Origin: engine.OriginHuman,
		Subject: spi.Subject{TenantID: e.tenant, UserID: ownerID, Kind: spi.SubjectInitiator,
			Permissions: []string{engine.PermRead, engine.PermAuthor, engine.PermEnable, engine.PermOperate, permClient}}}
}

// enable enables a version as a person.
func (e *env) enable(id uuid.UUID, version int) {
	e.t.Helper()
	ctx := context.Background()
	rv, err := e.rt.Service().EnableReview(ctx, e.human(), id, version)
	require.NoError(e.t, err)
	_, err = e.rt.Service().Enable(ctx, e.human(), id, engine.EnableInput{Version: version, CodeDigest: rv.CodeDigest})
	require.NoError(e.t, err)
}

func (e *env) publish(key string, c client) {
	e.t.Helper()
	ctx := context.Background()
	tx, err := e.db.Begin(ctx)
	require.NoError(e.t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = events.Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: e.tenant}}, clientCreated, &spi.ObjectRef{Entity: "acme.client", Key: key}, c)
	require.NoError(e.t, err)
	require.NoError(e.t, tx.Commit(ctx))
}

func (e *env) pass() {
	e.t.Helper()
	ctx := context.Background()
	_, err := e.disp.RunOnce(ctx)
	require.NoError(e.t, err)
	_, err = e.wf.RunOnce(ctx)
	require.NoError(e.t, err)
	require.NoError(e.t, e.rt.Maintain(ctx))
}

func (e *env) until(cond func() bool, msg string) {
	e.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		e.pass()
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Fail(e.t, "condition not met: "+msg)
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.db.QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

func noteGraph(name, text string) map[string]any {
	var g map[string]any
	if err := json.Unmarshal([]byte(`{"name": "`+name+`", "triggers": [{"id": "created", "event": "acme.client.created.v1"}],
	 "nodes": [{"id": "note", "type": "acme.note", "params": {"text": "`+text+`"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "note"},
	  {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}]}`), &g); err != nil {
		panic(err)
	}
	return g
}

type prepared struct {
	Status   string                 `json:"status"`
	PlanID   string                 `json:"plan_id"`
	PlanHash string                 `json:"plan_hash"`
	Errors   []changes.IndexedIssue `json:"errors"`
	Approval changes.ApprovalState  `json:"approval"`
}

type operation struct {
	OperationID string          `json:"operation_id"`
	Status      string          `json:"status"`
	Receipt     changes.Receipt `json:"receipt"`
	Error       *mcp.Failure    `json:"error"`
}

func (e *env) prepare(id uint, change map[string]any) prepared {
	e.t.Helper()
	var p prepared
	e.ok(id, changes.ToolPrepare, map[string]any{"changes": []any{change}}, &p)
	return p
}

func (e *env) apply(id uint, p prepared, key string) operation {
	e.t.Helper()
	require.Equal(e.t, changes.StatusReady, p.Status, "plan errors: %+v", p.Errors)
	var o operation
	e.ok(id, changes.ToolApply, map[string]any{"plan_id": p.PlanID, "plan_hash": p.PlanHash, "idempotency_key": key}, &o)
	return o
}

type saved struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Enabled   any    `json:"enabled"`
	EditorURL string `json:"editor_url"`
}

func receiptResult(t *testing.T, o operation) saved {
	t.Helper()
	require.Equal(t, changes.OperationSucceeded, o.Status, "%+v", o.Error)
	require.Len(t, o.Receipt.Items, 1)
	var s saved
	require.NoError(t, json.Unmarshal(o.Receipt.Items[0].Result, &s))
	return s
}

type got struct {
	ID             string `json:"id"`
	Enabled        bool   `json:"enabled"`
	EnabledVersion *int   `json:"enabled_version"`
	Version        int    `json:"version"`
	SavedOrigin    string `json:"saved_origin"`
	Revision       string `json:"revision"`
	LayoutRevision string `json:"layout_revision"`
	Versions       []struct {
		Version     int    `json:"version"`
		SavedOrigin string `json:"saved_origin"`
	} `json:"versions"`
	Layout    graph.Layout `json:"layout"`
	EditorURL string       `json:"editor_url"`
}

func TestAgentCreatesAndEditsGraphsThatOnlyAPersonEnables(t *testing.T) {
	e := newEnv(t)

	p := e.prepare(agentID, map[string]any{"op": "create", "entity": "automation", "fields": map[string]any{
		"graph": noteGraph("Welcome", "hello"), "layout": map[string]any{"note": map[string]any{"x": 10, "y": 20}}}})
	require.False(t, p.Approval.Required, "saving never makes a graph run: no approval")
	o := e.apply(agentID, p, "create-1")
	res := receiptResult(t, o)
	assert.Equal(t, 1, res.Version)
	assert.Equal(t, false, res.Enabled)
	assert.Contains(t, res.EditorURL, res.ID)

	// A repeated apply after a lost response returns the same operation and saves nothing again.
	again := e.apply(agentID, p, "create-1")
	assert.Equal(t, o.OperationID, again.OperationID)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.automations`))
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.versions`))

	var g got
	e.ok(agentID, mcpops.ToolGet, map[string]any{"id": res.ID}, &g)
	assert.False(t, g.Enabled)
	assert.Equal(t, "agent", g.SavedOrigin)
	assert.Equal(t, "1", g.Revision)
	assert.Equal(t, graph.Layout{"note": {X: 10, Y: 20}}, g.Layout)
	id := uuid.MustParse(res.ID)
	am, err := e.rt.Service().Get(context.Background(), e.human(), id)
	require.NoError(t, err)
	assert.Equal(t, engine.OriginAgent, am.CreatedOrigin)
	assert.Nil(t, am.EnabledVersion)

	// Events do not start the agent's graph.
	e.publish("c-1", client{Name: "Ann"})
	for range 5 {
		e.pass()
	}
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.runs`))

	// A person enables version 1; the agent saves version 2, which does not replace it.
	e.enable(id, 1)
	up := e.prepare(agentID, map[string]any{"op": "update", "entity": "automation", "id": res.ID, "expected_revision": "1",
		"fields": map[string]any{"graph": noteGraph("Welcome", "hello again"), "note": "friendlier"}})
	o = e.apply(agentID, up, "update-1")
	res = receiptResult(t, o)
	assert.Equal(t, 2, res.Version)
	assert.EqualValues(t, 1, res.Enabled, "the version that keeps running")
	again = e.apply(agentID, up, "update-1")
	assert.Equal(t, o.OperationID, again.OperationID)
	assert.Equal(t, 2, e.count(`SELECT count(*) FROM automation.versions`), "the retry saves no third version")
	e.ok(agentID, mcpops.ToolGet, map[string]any{"id": res.ID}, &g)
	require.NotNil(t, g.EnabledVersion)
	assert.Equal(t, 1, *g.EnabledVersion)
	assert.Equal(t, 2, g.Version)
	assert.Equal(t, "2", g.Revision)

	// A stale expected_revision is the shared conflict with the current revision: re-read and prepare again.
	stale := e.prepare(agentID, map[string]any{"op": "update", "entity": "automation", "id": res.ID, "expected_revision": "1",
		"fields": map[string]any{"graph": noteGraph("Welcome", "third")}})
	require.Equal(t, changes.StatusInvalid, stale.Status)
	require.NotEmpty(t, stale.Errors)
	assert.Equal(t, "revision_mismatch", stale.Errors[len(stale.Errors)-1].Code)
	assert.Contains(t, stale.Errors[len(stale.Errors)-1].Message, "2")

	// Layout: based on its own revision.
	lp := e.prepare(agentID, map[string]any{"op": "layout", "entity": "automation",
		"fields": map[string]any{"id": res.ID, "base_revision": atoi(t, g.LayoutRevision), "positions": map[string]any{"fin": map[string]any{"x": 5, "y": 6}}}})
	o = e.apply(agentID, lp, "layout-1")
	require.Equal(t, changes.OperationSucceeded, o.Status, "%+v", o.Error)
	var after got
	e.ok(agentID, mcpops.ToolGet, map[string]any{"id": res.ID}, &after)
	assert.NotEqual(t, g.LayoutRevision, after.LayoutRevision)
	assert.Equal(t, graph.XY{X: 5, Y: 6}, after.Layout["fin"])
	staleLayout := e.prepare(agentID, map[string]any{"op": "layout", "entity": "automation",
		"fields": map[string]any{"id": res.ID, "base_revision": atoi(t, g.LayoutRevision), "positions": map[string]any{"fin": map[string]any{"x": 1, "y": 1}}}})
	require.Equal(t, changes.StatusInvalid, staleLayout.Status)
	require.Len(t, staleLayout.Errors, 1)
	assert.Equal(t, "VERSION_CONFLICT", staleLayout.Errors[0].Code)
	assert.Equal(t, "/changes/0/fields/base_revision", staleLayout.Errors[0].Path)
	assert.Contains(t, staleLayout.Errors[0].Message, after.LayoutRevision)

	// Every applied change is an entry of the MCP audit.
	assert.Equal(t, []mcp.Code{""}, e.audit.outcomes("changes:automation.create"))
	assert.Equal(t, []mcp.Code{""}, e.audit.outcomes("changes:automation.update"))
	assert.Contains(t, e.audit.outcomes("changes:automation.layout"), mcp.Code(""))

	// The list shows the graph, enabled on version 1 with version 2 the latest.
	var list struct {
		Items []struct {
			ID             string `json:"id"`
			Enabled        bool   `json:"enabled"`
			EnabledVersion *int   `json:"enabled_version"`
			LatestVersion  int    `json:"latest_version"`
		} `json:"items"`
		NextCursor *string `json:"next_cursor"`
	}
	e.ok(readerID, mcpops.ToolList, map[string]any{"status": "enabled"}, &list)
	require.Len(t, list.Items, 1)
	assert.Equal(t, 2, list.Items[0].LatestVersion)
	assert.Nil(t, list.NextCursor)
	e.ok(readerID, mcpops.ToolList, map[string]any{"status": "disabled"}, &list)
	assert.Empty(t, list.Items)
	e.ok(readerID, mcpops.ToolList, map[string]any{"status": "attention"}, &list)
	assert.Empty(t, list.Items, "no run failed or is blocked")
}

func TestEnablingIsHumanRequiredInPlan(t *testing.T) {
	e := newEnv(t)
	p := e.prepare(agentID, map[string]any{"op": "create", "entity": "automation", "fields": map[string]any{
		"graph": noteGraph("Welcome", "hello"), "enabled": true}})
	require.Equal(t, changes.StatusInvalid, p.Status)
	require.Len(t, p.Errors, 1)
	assert.Equal(t, mcpops.IssueHumanRequired, p.Errors[0].Code)
	assert.Equal(t, "/changes/0/fields/enabled", p.Errors[0].Path)
	assert.Contains(t, p.Errors[0].Message, urls{}.Automations(), "the refusal names the page where a person acts")
	assert.Equal(t, []mcp.Code{mcp.CodeApprovalRequired}, e.audit.outcomes("changes:automation.create"), "the refusal is audited")
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.automations`))

	// false is refused too: an agent neither enables nor disables.
	p = e.prepare(agentID, map[string]any{"op": "create", "entity": "automation", "fields": map[string]any{
		"graph": noteGraph("Welcome", "hello"), "enabled": false}})
	require.Equal(t, changes.StatusInvalid, p.Status)

	// Managing connections and deciding tasks are a person's too.
	p = e.prepare(agentID, map[string]any{"op": "create", "entity": "automation", "fields": map[string]any{
		"graph": noteGraph("Welcome", "hello"), "connections": []any{map[string]any{"kind": "telegram_bot"}}, "decision": map[string]any{"outcome": "approve"}}})
	require.Equal(t, changes.StatusInvalid, p.Status)
	byPath := map[string]string{}
	for _, iss := range p.Errors {
		assert.Equal(t, mcpops.IssueHumanRequired, iss.Code, iss.Path)
		byPath[iss.Path] = iss.Message
	}
	assert.Contains(t, byPath["/changes/0/fields/connections"], urls{}.Connections())
	assert.Contains(t, byPath["/changes/0/fields/decision"], urls{}.Automations())

	// Other unknown fields are invalid input of the tool.
	_ = e.failure(agentID, changes.ToolPrepare, map[string]any{"changes": []any{map[string]any{"op": "create", "entity": "automation",
		"fields": map[string]any{"graph": noteGraph("Welcome", "hello"), "colour": "red"}}}}, mcp.CodeInvalidInput)
}

func TestInvalidGraphIsReportedUnderFieldsGraph(t *testing.T) {
	e := newEnv(t)
	bad := noteGraph("Broken", "x")
	bad["nodes"].([]any)[0].(map[string]any)["type"] = "acme.nope"
	p := e.prepare(agentID, map[string]any{"op": "create", "entity": "automation", "fields": map[string]any{"graph": bad}})
	require.Equal(t, changes.StatusInvalid, p.Status)
	require.NotEmpty(t, p.Errors)
	assert.True(t, strings.HasPrefix(p.Errors[0].Path, "/changes/0/fields/graph/nodes/0"), p.Errors[0].Path)

	var v struct {
		OK     bool               `json:"ok"`
		Errors []graph.GraphError `json:"errors"`
	}
	// validate is read-only: lookup runs it.
	res := e.call(agentID, lookup.ToolName, map[string]any{"method": mcpops.ToolValidate, "input": map[string]any{"graph": bad}})
	require.True(t, res.OK, "%+v", res.Error)
	require.NoError(t, json.Unmarshal(res.Data, &v))
	assert.False(t, v.OK)
	assert.NotEmpty(t, v.Errors)
	e.ok(agentID, mcpops.ToolValidate, map[string]any{"graph": noteGraph("Fine", "x")}, &v)
	assert.True(t, v.OK)

	// prepare_changes has side effects: lookup refuses it and points to execute.
	f := e.failure(agentID, lookup.ToolName, map[string]any{"method": changes.ToolPrepare, "input": map[string]any{"changes": []any{}}}, mcp.CodeInvalidInput)
	assert.Contains(t, f.Message, "execute")
}

func TestDryRunThroughLookup(t *testing.T) {
	e := newEnv(t)
	input := map[string]any{"graph": noteGraph("Dry", "hi {{ event.name }}"), "subject": map[string]any{"entity": "acme.client", "key": "c-1"},
		"events": []any{map[string]any{"kind": evClient, "name": "Ann", "phone": "+998"}}}
	res := e.call(agentID, lookup.ToolName, map[string]any{"method": mcpops.ToolDryRun, "input": input})
	require.True(t, res.OK, "%+v", res.Error)
	var out struct {
		Trace []struct {
			Node    string            `json:"node"`
			Kind    string            `json:"kind"`
			Status  string            `json:"status"`
			Effects []json.RawMessage `json:"effects"`
		} `json:"trace"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &out))
	require.NotEmpty(t, out.Trace)
	captured := false
	for _, s := range out.Trace {
		if s.Node == "note" {
			captured = s.Status == "captured" && strings.Contains(string(s.Effects[0]), "hi Ann")
		}
	}
	assert.True(t, captured, "the effect is captured, not made: %+v", out.Trace)
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.runs`))

	// live decisions need allow_live.
	input["decisions"] = "live"
	_ = e.failure(agentID, mcpops.ToolDryRun, input, mcp.CodeInvalidInput)
	input["allow_live"] = true
	e.ok(agentID, mcpops.ToolDryRun, input, nil)

	// More than 50 events is refused.
	many := make([]any, 51)
	for i := range many {
		many[i] = map[string]any{"kind": evClient}
	}
	input["events"] = many
	_ = e.failure(agentID, mcpops.ToolDryRun, input, mcp.CodeLimitExceeded)
}

func TestRightsAreScopesAndRBACOnEveryCall(t *testing.T) {
	e := newEnv(t)
	_ = e.failure(readerID, mcpops.ToolValidate, map[string]any{"graph": noteGraph("x", "x")}, mcp.CodeForbidden)
	_ = e.failure(readerID, changes.ToolPrepare, map[string]any{"changes": []any{map[string]any{"op": "create", "entity": "automation",
		"fields": map[string]any{"graph": noteGraph("x", "x")}}}}, mcp.CodeForbidden)
	readOnly := e.ctx(agentID, mcp.GrantedScopes(mcpops.ScopeRead))
	res := e.callAs(readOnly, mcpops.ToolValidate, map[string]any{"graph": noteGraph("x", "x")})
	require.False(t, res.OK)
	assert.Equal(t, mcp.CodeForbidden, res.Error.Code, "the grant lacks automations:author")
	res = e.callAs(readOnly, mcpops.ToolList, map[string]any{})
	assert.True(t, res.OK, "%+v", res.Error)

	// Another tenant's or an unknown graph is not found.
	_ = e.failure(agentID, mcpops.ToolGet, map[string]any{"id": uuid.NewString()}, mcp.CodeNotFound)
	p := e.prepare(agentID, map[string]any{"op": "update", "entity": "automation", "id": uuid.NewString(), "expected_revision": "1",
		"fields": map[string]any{"graph": noteGraph("x", "x")}})
	require.Equal(t, changes.StatusInvalid, p.Status)
	assert.Equal(t, string(mcp.CodeNotFound), p.Errors[0].Code)
}

func TestCatalogAndRunsAreProjectedForThePrincipal(t *testing.T) {
	e := newEnv(t)
	type catalogOut struct {
		Events []struct {
			Key     string          `json:"key"`
			Payload json.RawMessage `json:"payload"`
		} `json:"events"`
		Nodes []struct {
			Type     string   `json:"type"`
			Outcomes []string `json:"outcomes"`
		} `json:"nodes"`
		Format json.RawMessage `json:"format"`
	}
	payloadOf := func(id uint) string {
		var c catalogOut
		e.ok(id, mcpops.ToolCatalog, map[string]any{}, &c)
		assert.NotEmpty(t, c.Format)
		for _, ev := range c.Events {
			if ev.Key == evClient {
				return string(ev.Payload)
			}
		}
		t.Fatalf("no %s in the catalog", evClient)
		return ""
	}
	assert.Contains(t, payloadOf(agentID), `"phone"`)
	assert.NotContains(t, payloadOf(blindID), `"phone"`, "the agent without Client.Read does not see the phone")

	// A person's enabled graph reads the phone; the agent without Client.Read sees its steps hidden.
	am, v, err := e.rt.Service().Create(context.Background(), e.human(), engine.CreateInput{Graph: mustJSON(noteGraph("Call", "call {{ event.phone }}"))})
	require.NoError(t, err)
	e.enable(am.ID, v.Version)
	e.publish("c-1", client{Name: "Ann", Phone: "+998901234567"})
	var runs struct {
		Items []struct {
			RunID  string `json:"run_id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	e.until(func() bool {
		e.ok(blindID, mcpops.ToolRuns, map[string]any{"id": am.ID.String()}, &runs)
		return len(runs.Items) == 1 && runs.Items[0].Status == string(engine.RunDone)
	}, "the run ends")
	full := e.call(agentID, mcpops.ToolRunGet, map[string]any{"run_id": runs.Items[0].RunID})
	require.True(t, full.OK, "%+v", full.Error)
	assert.Contains(t, string(full.Data), "+998901234567")
	blind := e.call(blindID, mcpops.ToolRunGet, map[string]any{"run_id": runs.Items[0].RunID})
	require.True(t, blind.OK, "%+v", blind.Error)
	assert.NotContains(t, string(blind.Data), "+998901234567")
	assert.Contains(t, string(blind.Data), `"hidden":true`)
	assert.Contains(t, string(blind.Data), "tab=runs")
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	require.NoError(t, err)
	return n
}
