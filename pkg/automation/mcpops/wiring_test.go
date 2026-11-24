package mcpops_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/mcpops"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	excatalog "github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage/managetest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/mcptools"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
)

// The workflow tools as the standard wiring registers them (mcptools.Register with the management
// service on the workflow engine, no explicit guard) refuse every action on a real automation.run
// instance, on its human task and on a foreign one, and keep reads.
func TestStandardWorkflowToolsRefuseAutomationInstances(t *testing.T) {
	e := newEnv(t)
	arts, err := artifact.NewStore(e.db, t.TempDir())
	require.NoError(t, err)
	natives := excatalog.NewNativeSet(data.DefaultNatives())
	svc, err := manage.New(manage.Config{DB: e.db, Builder: &managetest.Builder{}, Kits: &managetest.Kits{}, Artifacts: arts,
		Catalog: excatalog.New(e.db, natives), Natives: data.DefaultNatives()})
	require.NoError(t, err)
	svc.SetWorkflows(e.wf)
	registry := mcp.NewRegistry()
	require.NoError(t, mcptools.Register(registry, mcptools.Deps{Manage: svc}))
	tools, err := mcp.NewEngine(registry, e.authz)
	require.NoError(t, err)

	g := map[string]any{"name": "Approve", "triggers": []any{map[string]any{"id": "created", "event": evClient}},
		"nodes": []any{map[string]any{"id": "again", "type": "wait", "params": map[string]any{"event": evClient}},
			map[string]any{"id": "appr", "type": "acme.approve", "params": map[string]any{}}, map[string]any{"id": "fin", "type": "end", "params": map[string]any{}}},
		"edges": []any{map[string]any{"from": map[string]any{"node": "created", "outcome": "fired"}, "to": "again"},
			map[string]any{"from": map[string]any{"node": "again", "outcome": "matched"}, "to": "appr"},
			map[string]any{"from": map[string]any{"node": "appr", "outcome": "approve"}, "to": "fin"},
			map[string]any{"from": map[string]any{"node": "appr", "outcome": "reject"}, "to": "fin"}}}
	am, v, err := e.rt.Service().Create(context.Background(), e.human(), engine.CreateInput{Graph: mustJSON(g)})
	require.NoError(t, err)
	e.enable(am.ID, v.Version)
	e.publish("c-1", client{Name: "Ann"})
	e.until(func() bool { return e.count(`SELECT count(*) FROM automation.waits WHERE status = 'open'`) == 1 }, "the run waits")
	e.publish("c-1", client{Name: "Bob", Phone: "+998900000000"})
	e.until(func() bool { return e.count(`SELECT count(*) FROM extruntime.human_tasks WHERE status = 'open'`) == 1 }, "the task opens")
	var instance, task, runID uuid.UUID
	// The run that waited for the second event (its inbox signal) opened the task.
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT r.instance_id, r.id, t.id FROM extruntime.human_tasks t
		JOIN automation.runs r ON r.instance_id = t.instance_id`).Scan(&instance, &runID, &task))
	runPage := urls{}.Run(am.ID, runID)

	call := func(tool string, input map[string]any) mcp.Result {
		raw, err := json.Marshal(input)
		require.NoError(t, err)
		return tools.Call(e.ctx(ownerID, mcp.AllScopes()), tool, raw)
	}
	id := instance.String()
	for tool, input := range map[string]map[string]any{
		"ext_workflow_signal":      {"id": id, "name": engine.InboxSignal},
		"ext_workflow_terminate":   {"id": id},
		"ext_workflow_start_v2":    {"id": id},
		"ext_workflow_resolve":     {"id": id, "name": "task:x", "n": 0},
		"ext_workflow_retry":       {"id": id, "name": "task:x", "n": 0},
		"ext_workflow_resume":      {"id": id},
		"ext_workflow_decide_task": {"task_id": task.String(), "decision": map[string]any{"outcome": "approve"}},
	} {
		res := call(tool, input)
		require.False(t, res.OK, tool)
		assert.Equal(t, mcp.CodeApprovalRequired, res.Error.Code, "%s: %+v", tool, res.Error)
		require.Len(t, res.Error.Issues, 1, tool)
		assert.Equal(t, mcptools.IssueHumanRequired, res.Error.Issues[0].Code, tool)
		assert.Contains(t, res.Error.Message, runPage, tool)
	}
	res := call("ext_workflow_get", map[string]any{"id": id})
	require.True(t, res.OK, "reads stay: %+v", res.Error)
	// workflow.History has no JSON tags: its fields keep their Go names.
	var h struct {
		Instance struct {
			Workflow string          `json:"Workflow"`
			Input    json.RawMessage `json:"Input"`
		} `json:"Instance"`
		Signals []struct {
			Payload json.RawMessage `json:"Payload"`
		} `json:"Signals"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &h))
	assert.Equal(t, engine.WorkflowID, h.Instance.Workflow)
	assert.True(t, isNull(h.Instance.Input), "no input of an automation instance: %s", h.Instance.Input)
	require.NotEmpty(t, h.Signals)
	for _, s := range h.Signals {
		assert.True(t, isNull(s.Payload), "no signal payload: %s", s.Payload)
	}
	assert.NotContains(t, string(res.Data), "+998900000000", "the event payload is not shown")
	assert.NotContains(t, string(res.Data), taskTitle, "the task title is not shown")
	assert.Contains(t, string(res.Data), task.String(), "the task itself is listed")
	res = call("ext_workflow_list", map[string]any{})
	require.True(t, res.OK, "%+v", res.Error)
	assert.NotContains(t, string(res.Data), "+998900000000")

	// Who may not decide the task is refused as for any task, and is not told the page.
	raw, err := json.Marshal(map[string]any{"task_id": task.String(), "decision": map[string]any{"outcome": "approve"}})
	require.NoError(t, err)
	res = tools.Call(e.ctx(agentID, mcp.AllScopes()), "ext_workflow_decide_task", raw)
	require.False(t, res.OK)
	assert.Equal(t, mcp.CodeForbidden, res.Error.Code, "%+v", res.Error)
	assert.NotContains(t, res.Error.Message, runPage)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.human_tasks WHERE status = 'open'`), "nothing was decided")
}

func isNull(raw json.RawMessage) bool { return len(raw) == 0 || string(raw) == "null" }

func decisionGraph(items int) map[string]any {
	var g map[string]any
	raw := `{"name": "Decide", "triggers": [{"id": "created", "event": "acme.client.created.v1"}],
	 "nodes": [
	  {"id": "each", "type": "foreach", "params": {"items": "` + "[" + strings.TrimSuffix(strings.Repeat("1,", items), ",") + "]" + `", "concurrency": 1}},
	  {"id": "dec", "type": "acme.decide", "params": {"text": "{{ string(foreach.each.index) }}"}},
	  {"id": "fetch", "type": "acme.fetch", "params": {"text": "x"}},
	  {"id": "take", "type": "collect", "params": {"of": "each"}},
	  {"id": "fin", "type": "end", "params": {}}
	 ],
	 "edges": [
	  {"from": {"node": "created", "outcome": "fired"}, "to": "each"},
	  {"from": {"node": "each", "outcome": "item"}, "to": "dec"},
	  {"from": {"node": "dec", "outcome": "done"}, "to": "fetch"},
	  {"from": {"node": "dec", "outcome": "failed"}, "to": "take"},
	  {"from": {"node": "fetch", "outcome": "done"}, "to": "take"},
	  {"from": {"node": "fetch", "outcome": "failed"}, "to": "take"},
	  {"from": {"node": "each", "outcome": "done"}, "to": "fin"},
	  {"from": {"node": "each", "outcome": "partial_failed"}, "to": "fin"},
	  {"from": {"node": "each", "outcome": "failed"}, "to": "fin"}
	 ]}`
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		panic(err)
	}
	return g
}

func TestDryRunCallsOnlyDecisionNodesAndOnlyWithAllowLive(t *testing.T) {
	e := newEnv(t)
	input := func(items int, decisions string, live bool) map[string]any {
		in := map[string]any{"graph": decisionGraph(items), "subject": map[string]any{"entity": "acme.client", "key": "c-1"},
			"events": []any{map[string]any{"kind": evClient, "name": "Ann"}}, "decisions": decisions}
		if live {
			in["allow_live"] = true
		}
		return in
	}
	// Scripted and cached without allow_live make no call at all.
	for _, d := range []string{"scripted", "cached"} {
		e.ok(agentID, mcpops.ToolDryRun, input(2, d, false), nil)
	}
	assert.Zero(t, e.decided.Load())
	assert.Zero(t, e.fetched.Load())
	_ = e.failure(agentID, mcpops.ToolDryRun, input(2, "live", false), mcp.CodeInvalidInput)

	// allow_live: decision nodes call for real, other execute nodes never do.
	e.ok(agentID, mcpops.ToolDryRun, input(2, "live", true), nil)
	assert.EqualValues(t, 2, e.decided.Load())
	assert.Zero(t, e.fetched.Load())

	// A run that would make more than 20 real calls is limit_exceeded, and the 21st call is not made.
	e.decided.Store(0)
	_ = e.failure(agentID, mcpops.ToolDryRun, input(25, "live", true), mcp.CodeLimitExceeded)
	assert.EqualValues(t, mcpops.MaxLiveCalls, e.decided.Load())

	// Events are counted inside batches: two batches of 30 are 60 events.
	batch := make([]any, 30)
	for i := range batch {
		batch[i] = map[string]any{"kind": evClient}
	}
	in := input(1, "scripted", false)
	in["events"] = []any{map[string]any{"events": batch}, map[string]any{"events": batch}}
	_ = e.failure(agentID, mcpops.ToolDryRun, in, mcp.CodeLimitExceeded)
}

// An author who may not read the automation list still dry-runs and edits a saved graph.
func TestAuthorWithoutReadEditsSavedGraphs(t *testing.T) {
	e := newEnv(t)
	authorOnly := uint(9)
	e.users[authorOnly] = account(e.tenant, authorOnly, e.users[agentID].Roles()[0].Permissions()[1:]...)
	require.NotContains(t, permNames(e, authorOnly), engine.PermRead)
	p := e.prepare(authorOnly, map[string]any{"op": "create", "entity": "automation", "fields": map[string]any{"graph": noteGraph("A", "a")}})
	res := receiptResult(t, e.apply(authorOnly, p, "a-1"))
	e.ok(authorOnly, mcpops.ToolDryRun, map[string]any{"id": res.ID, "subject": map[string]any{"entity": "acme.client", "key": "c-1"},
		"events": []any{map[string]any{"kind": evClient}}}, nil)
	up := e.prepare(authorOnly, map[string]any{"op": "update", "entity": "automation", "id": res.ID, "expected_revision": "1",
		"fields": map[string]any{"graph": noteGraph("A", "b")}})
	assert.Equal(t, 2, receiptResult(t, e.apply(authorOnly, up, "a-2")).Version)
	lp := e.prepare(authorOnly, map[string]any{"op": "layout", "entity": "automation",
		"fields": map[string]any{"id": res.ID, "base_revision": 2, "positions": map[string]any{"fin": map[string]any{"x": 1, "y": 1}}}})
	require.Equal(t, changes.OperationSucceeded, e.apply(authorOnly, lp, "a-3").Status)
	_ = e.failure(authorOnly, mcpops.ToolList, map[string]any{}, mcp.CodeForbidden)
}

func permNames(e *env, id uint) []string {
	var out []string
	for _, r := range e.users[id].Roles() {
		for _, p := range r.Permissions() {
			out = append(out, p.Name())
		}
	}
	return out
}

// Applying a change holds one connection for the operation's transaction: every read the engine and
// the kinds make while applying goes through it, so a pool of one connection is enough.
func TestApplyNeedsOneConnection(t *testing.T) {
	e := newEnv(t)
	cfg, err := pgxpool.ParseConfig(e.db.Config().ConnString())
	require.NoError(t, err)
	cfg.MaxConns, cfg.MinConns = 1, 0
	one, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(one.Close)
	rt, err := engine.NewRuntime(engine.Config{DB: one, Workflows: e.wf, Users: e.subjects, Catalog: e.reg, Tasks: tasks{}, Links: links{}})
	require.NoError(t, err)
	svc, err := changes.NewService(changes.Config{Store: changes.NewPostgresStore(one), Authorizer: e.authz,
		Kinds: mcpops.Kinds(rt.Service(), rt.Authoring(), urls{}), ApprovalURL: func(string) string { return "" }})
	require.NoError(t, err)
	registry := mcp.NewRegistry()
	require.NoError(t, registry.Register(svc.Tools()...))
	eng, err := mcp.NewEngine(registry, e.authz)
	require.NoError(t, err)
	call := func(tool string, input any, into any) {
		t.Helper()
		ctx, cancel := context.WithTimeout(e.ctx(agentID, mcp.AllScopes()), 15*time.Second)
		defer cancel()
		res := eng.Call(ctx, tool, mustJSON(input))
		require.True(t, res.OK, "%s: %+v", tool, res.Error)
		require.NoError(t, json.Unmarshal(res.Data, into))
	}
	apply := func(change map[string]any, key string) operation {
		var p prepared
		call(changes.ToolPrepare, map[string]any{"changes": []any{change}}, &p)
		require.Equal(t, changes.StatusReady, p.Status, "%+v", p.Errors)
		var o operation
		call(changes.ToolApply, map[string]any{"plan_id": p.PlanID, "plan_hash": p.PlanHash, "idempotency_key": key}, &o)
		require.Equal(t, changes.OperationSucceeded, o.Status, "%+v", o.Error)
		return o
	}
	created := receiptResult(t, apply(map[string]any{"op": "create", "entity": "automation", "fields": map[string]any{"graph": noteGraph("One", "a")}}, "one-1"))
	apply(map[string]any{"op": "update", "entity": "automation", "id": created.ID, "expected_revision": "1",
		"fields": map[string]any{"graph": noteGraph("One", "b")}}, "one-2")
	apply(map[string]any{"op": "layout", "entity": "automation",
		"fields": map[string]any{"id": created.ID, "base_revision": 2, "positions": map[string]any{"fin": map[string]any{"x": 1, "y": 1}}}}, "one-3")
}
