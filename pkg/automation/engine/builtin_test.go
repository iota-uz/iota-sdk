package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

var (
	sandboxOnce sync.Once
	sandboxEng  sandbox.Engine
	errSandbox  error
)

func testSandbox(t *testing.T) sandbox.Engine {
	t.Helper()
	sandboxOnce.Do(func() { sandboxEng, errSandbox = sandbox.NewEngine(context.Background()) })
	require.NoError(t, errSandbox)
	return sandboxEng
}

// fixedSecrets are a tenant's secrets by name.
type fixedSecrets map[string]string

func (s fixedSecrets) Secret(_ context.Context, _ uuid.UUID, _ spi.Subject, name string) (string, error) {
	if v, ok := s[name]; ok {
		return v, nil
	}
	return "", access.ErrSecretMissing
}

// permBlock is what the test action needs.
const permBlock = "Users.Block"

// blockAction records the users it blocked: its effect.
type blockAction struct {
	mu      sync.Mutex
	blocked []string
	calls   atomic.Int32
}

func (a *blockAction) Key() string { return "test.user.block" }
func (a *blockAction) Spec() actions.ActionSpec {
	return actions.ActionSpec{
		Label:      manifest.Label{"en": "Block user", "ru": "Заблокировать пользователя"},
		Group:      "users",
		Permission: permBlock,
		Args: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			{Name: "user_id", Type: manifest.Type{Kind: manifest.KindString}},
			{Name: "days", Type: manifest.Type{Kind: manifest.KindInt, Optional: true}},
		}},
		Output: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{{Name: "blocked", Type: manifest.Type{Kind: manifest.KindString}}}},
	}
}

func (a *blockAction) Run(ctx context.Context, actor spi.Subject, args json.RawMessage) (string, json.RawMessage, error) {
	a.calls.Add(1)
	var in struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", nil, actions.ErrInvalidArgs
	}
	if actions.InvocationID(ctx) == "" {
		return "", nil, errors.New("no invocation id")
	}
	a.mu.Lock()
	a.blocked = append(a.blocked, in.UserID)
	a.mu.Unlock()
	out, err := json.Marshal(map[string]string{"blocked": in.UserID})
	return "done", out, err
}

// builtinEnv is an env whose catalog has the built-in nodes.
type builtinEnv struct {
	*env
	rt     atomic.Pointer[engine.Runtime]
	host   *commandHost
	action *blockAction
}

func newBuiltinEnv(t *testing.T) *builtinEnv {
	t.Helper()
	be := &builtinEnv{env: newEnv(t), action: &blockAction{}}
	require.NoError(t, builtin.Register(be.reg, builtin.Deps{
		Engine:  testSandbox(t),
		Secrets: fixedSecrets{"API_KEY": "sk-live-7f3a9c1d"},
		Runs: func() (engine.RunStarter, error) {
			if rt := be.rt.Load(); rt != nil {
				return rt, nil
			}
			return nil, errors.New("no runtime yet")
		},
		Actions: []actions.Action{be.action},
	}))
	return be
}

func (be *builtinEnv) proc(tune ...func(*engine.Config)) *proc {
	idx, err := actions.NewIndex(be.action)
	require.NoError(be.t, err)
	be.host = &commandHost{idx: idx}
	p := be.env.proc(append([]func(*engine.Config){func(c *engine.Config) { c.Commands = be.host }}, tune...)...)
	be.rt.Store(p.rt)
	return p
}

// commandHost is the command host double: it runs an action's command body as the request's subject
// and keeps the outcome of each invocation ID, as the host's invocation record does.
type commandHost struct {
	idx  *actions.Index
	mu   sync.Mutex
	seen map[string]json.RawMessage
	ids  []string
}

func (h *commandHost) Invoke(ctx context.Context, req host.Request) (host.Result, error) {
	h.mu.Lock()
	if h.seen == nil {
		h.seen = map[string]json.RawMessage{}
	}
	h.ids = append(h.ids, req.InvocationID)
	if out, ok := h.seen[req.InvocationID]; ok {
		h.mu.Unlock()
		return host.Result{InvocationID: req.InvocationID, Output: out}, nil
	}
	h.mu.Unlock()
	out, err := h.idx.Run(actions.WithInvocation(ctx, req.InvocationID), req.Contract, req.Subject, req.Input)
	if err != nil {
		return host.Result{InvocationID: req.InvocationID}, err
	}
	h.mu.Lock()
	h.seen[req.InvocationID] = out
	h.mu.Unlock()
	return host.Result{InvocationID: req.InvocationID, Output: out}, nil
}

// step is the step of a node of a run.
func (be *builtinEnv) step(p *proc, run uuid.UUID, node string) engine.StepView {
	be.t.Helper()
	for _, s := range be.run(p, be.human(), run).Steps {
		if s.NodeID == node && s.EndedAt != nil {
			return s
		}
	}
	require.Failf(be.t, "no step", "node %s", node)
	return engine.StepView{}
}

func sandboxGraph(params string) string {
	return graphJSON("Code", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "run", "type": "code", "params": `+params+`},
		 {"id": "ok", "type": "acme.note", "params": {"text": "{{ nodes.run.output.output.value }}"}},
		 {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.run.output.error.code }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "run"},
		 {"from": {"node": "run", "outcome": "done"}, "to": "ok"},
		 {"from": {"node": "run", "outcome": "failed"}, "to": "bad"},
		 {"from": {"node": "ok", "outcome": "done"}, "to": "fin"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"},
		 {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}`)
}

func codeParams(src string, extra string) string {
	b, err := json.Marshal(src)
	if err != nil {
		panic(err)
	}
	return `{"lang": "ts", "source": ` + string(b) + extra + `}`
}

func TestCodeNodeReadsTheStepContextAndMasksItsSecret(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	be.create(p, a, sandboxGraph(codeParams(`export default async function run(ctx, env) {
		env.log("key", env.secrets.API_KEY);
		return { outcome: "done", output: { value: String(ctx.event.amount * 2), echo: JSON.stringify({ k: env.secrets.API_KEY }) } };
	}`, `, "secrets": ["API_KEY"]`)))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 21}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the code runs")
	assert.Equal(t, []string{"ok:42"}, be.effects(run.RunID))
	s := be.step(p, run.RunID, "run")
	assert.Equal(t, "done", s.Outcome)
	assert.NotContains(t, string(s.Output), "sk-live-7f3a9c1d", "the secret is masked in the step journal")
	assert.Contains(t, string(s.Output), `key ***`)
	assert.Contains(t, string(s.Output), `***`)
}

func TestCodeNodeInfiniteLoopTimesOut(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	be.create(p, a, sandboxGraph(codeParams(`export default async function run() { while (true) {} }`, `, "timeout": "1s"`)))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the loop is stopped")
	assert.Equal(t, []string{"bad:timeout"}, be.effects(run.RunID))
	s := be.step(p, run.RunID, "run")
	require.NotNil(t, s.EndedAt)
	// The call took its timeout, not more (the child instance starts after the step's commit).
	var started, ended time.Time
	require.NoError(t, be.db.QueryRow(context.Background(), `SELECT min(o.created_at), max(o.updated_at) FROM extruntime.workflow_occurrences o
		JOIN extruntime.workflow_instances i ON i.id = o.instance_id WHERE i.workflow = 'automation.call' AND o.name = 'call'`).Scan(&started, &ended))
	assert.Less(t, ended.Sub(started), 2*time.Second, "within the timeout and a second")
}

func TestCodeNodeCompileErrorIsAValidationError(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	_, _, err := p.rt.Service().Create(context.Background(), be.human(), engine.CreateInput{Graph: json.RawMessage(sandboxGraph(codeParams("export default async function run( {\n  return 1;\n}", "")))})
	var ve *engine.ValidationError
	require.ErrorAs(t, err, &ve)
	require.NotEmpty(t, ve.Errors)
	found := false
	for _, ge := range ve.Errors {
		if ge.Path == "nodes[0].params.source" {
			found = true
			assert.Positive(t, ge.Line)
			assert.Positive(t, ge.Column)
		}
	}
	assert.True(t, found, "the error names the source: %v", ve.Errors)

	_, _, err = p.rt.Service().Create(context.Background(), be.human(), engine.CreateInput{Graph: json.RawMessage(sandboxGraph(codeParams(`import x from "y"; export default async () => ({outcome: x})`, "")))})
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "nodes[0].params.source", ve.Errors[0].Path)
}

func TestCodeNodeIsReviewedBeforeEnable(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	am, v, err := p.rt.Service().Create(context.Background(), a, engine.CreateInput{Graph: json.RawMessage(sandboxGraph(codeParams(
		`export default async () => ({ outcome: "done", output: { value: "x" } })`, `, "secrets": ["API_KEY"], "fetch": true`)))})
	require.NoError(t, err)
	rv, err := p.rt.Service().EnableReview(context.Background(), a, am.ID, v.Version)
	require.NoError(t, err)
	require.Len(t, rv.CodeNodes, 1)
	assert.Equal(t, []string{"API_KEY"}, rv.CodeNodes[0].Secrets)
	assert.True(t, rv.CodeNodes[0].Fetch)
	assert.NotEmpty(t, rv.CodeDigest)
	_, err = p.rt.Service().Enable(context.Background(), a, am.ID, engine.EnableInput{Version: v.Version})
	require.ErrorIs(t, err, engine.ErrReviewRequired)
}

func TestSetNodeValuesReachTheNextNode(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	be.create(p, a, graphJSON("Set", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "calc", "type": "set", "params": {"values": {"total": "event.amount * 3", "label": "'order ' + string(event.amount)"}}},
		 {"id": "ok", "type": "acme.note", "params": {"text": "{{ nodes.calc.output.label }} = {{ nodes.calc.output.total }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "calc"},
		 {"from": {"node": "calc", "outcome": "done"}, "to": "ok"},
		 {"from": {"node": "ok", "outcome": "done"}, "to": "fin"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"}`))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 5}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the run ends")
	assert.Equal(t, []string{"ok:order 5 = 15"}, be.effects(run.RunID))

	// A value of a wrong expression is refused when the graph is saved, at its path.
	_, _, err := p.rt.Service().Create(context.Background(), a, engine.CreateInput{Graph: json.RawMessage(graphJSON("Set", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "calc", "type": "set", "params": {"values": {"total": "event.nope * 3"}}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "calc"}, {"from": {"node": "calc", "outcome": "done"}, "to": "fin"}`))})
	var ve *engine.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "nodes[0].params.values.total", ve.Errors[0].Path)
}

func actionGraph() string {
	return graphJSON("Block", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "block", "type": "action", "params": {"action": "test.user.block", "args": {"user_id": "u-{{ event.amount }}", "days": "event.amount + 1"}}},
		 {"id": "ok", "type": "acme.note", "params": {"text": "{{ nodes.block.output.blocked }}"}},
		 {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.block.output.error.code }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "block"},
		 {"from": {"node": "block", "outcome": "done"}, "to": "ok"},
		 {"from": {"node": "block", "outcome": "failed"}, "to": "bad"},
		 {"from": {"node": "ok", "outcome": "done"}, "to": "fin"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"},
		 {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}`)
}

func TestActionRunsAsTheRunAs(t *testing.T) {
	be := newBuiltinEnv(t)
	be.subjects.Grant(1, append(append([]string{}, allPerms...), permBlock)...)
	p := be.proc()
	a := be.actor(be.tenant, 1, engine.OriginHuman, append(append([]string{}, allPerms...), permBlock)...)
	be.create(p, a, actionGraph())
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 9}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the action runs")
	assert.Equal(t, []string{"ok:u-9"}, be.effects(run.RunID))
	assert.Equal(t, []string{"u-9"}, be.action.blocked)
	// The action went through the command host with the call's invocation ID.
	require.Len(t, be.host.ids, 1)
	_, err := uuid.Parse(be.host.ids[0])
	require.NoError(t, err)
	s := be.step(p, run.RunID, "block")
	assert.Equal(t, "done", s.Outcome)
}

func TestActionForbiddenForTheRunAsHasNoEffect(t *testing.T) {
	be := newBuiltinEnv(t)
	withPerm := append(append([]string{}, allPerms...), permBlock)
	be.subjects.Grant(1, withPerm...)
	p := be.proc()
	a := be.actor(be.tenant, 1, engine.OriginHuman, withPerm...)
	be.create(p, a, actionGraph())
	// The owner loses the right after enabling: the author's right is not carried to the run.
	be.subjects.Grant(1, allPerms...)
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 9}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the run ends by failed")
	assert.Equal(t, []string{"bad:forbidden"}, be.effects(run.RunID))
	assert.Zero(t, be.action.calls.Load(), "nothing was done")
}

func TestActionIsHiddenFromAnAuthorWithoutItsPermission(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	_, _, err := p.rt.Service().Create(context.Background(), be.human(), engine.CreateInput{Graph: json.RawMessage(actionGraph())})
	var ve *engine.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "nodes[0].type", ve.Errors[0].Path)
}

// child is a graph started by another: a manual trigger, a note of its input, the end.
const childGraph = `{"name": "Child", "triggers": [{"id": "m", "event": "automation.called.v1"}],
 "nodes": [
  {"id": "note", "type": "acme.note", "params": {"text": "child {{ event.input.amount }}"}},
  {"id": "hold", "type": "wait", "params": {"event": "acme.order.paid.v1", "timeout": "1h"}},
  {"id": "fin", "type": "end", "params": {"reason": "child finished"}}
 ],
 "edges": [
  {"from": {"node": "m", "outcome": "fired"}, "to": "note"},
  {"from": {"node": "note", "outcome": "done"}, "to": "hold"},
  {"from": {"node": "note", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "hold", "outcome": "matched"}, "to": "fin"},
  {"from": {"node": "hold", "outcome": "timeout"}, "to": "fin"}
 ]}`

func parentGraph(nodeType string, child uuid.UUID) string {
	done := `{"from": {"node": "call", "outcome": "done"}, "to": "ok"},
		 {"from": {"node": "call", "outcome": "timeout"}, "to": "bad"},`
	if nodeType == "start_automation" {
		done = `{"from": {"node": "call", "outcome": "started"}, "to": "ok"},`
	}
	text := "{{ nodes.call.output.output.reason }}"
	if nodeType == "start_automation" {
		text = "started"
	}
	return graphJSON("Parent", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "call", "type": "`+nodeType+`", "params": {"automation_id": "`+child.String()+`", "input": {"amount": "event.amount"}, "subject": "{'entity': 'acme.order', 'key': 'child-' + string(event.amount)}"}},
		 {"id": "ok", "type": "acme.note", "params": {"text": "`+text+`"}},
		 {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.call.output.error.code }}"}},
		 {"id": "hold", "type": "wait", "params": {"event": "acme.order.paid.v1", "timeout": "1h"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "call"},`+done+`
		 {"from": {"node": "call", "outcome": "failed"}, "to": "bad"},
		 {"from": {"node": "ok", "outcome": "done"}, "to": "hold"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"},
		 {"from": {"node": "hold", "outcome": "matched"}, "to": "fin"}, {"from": {"node": "hold", "outcome": "timeout"}, "to": "fin"},
		 {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}`)
}

// runsOf are the runs of a graph.
func (be *builtinEnv) runsOf(p *proc, id uuid.UUID) []engine.RunSummary {
	return be.runs(p, be.human(), engine.RunFilter{AutomationID: &id})
}

func TestRunAutomationReturnsTheChildOutput(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	child := be.create(p, a, childGraph)
	parent := be.create(p, a, parentGraph("run_automation", child.ID))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 4}, 0)
	var kids []engine.RunSummary
	p.until(func() bool {
		kids = be.runsOf(p, child.ID)
		return len(kids) == 1 && kids[0].Status == engine.RunWaiting
	}, "the child runs")
	pr := be.runsOf(p, parent.ID)
	require.Len(t, pr, 1)
	assert.Equal(t, engine.RunWaiting, pr[0].Status, "the parent waits for the child")
	assert.Equal(t, []string{"note:child 4"}, be.effects(kids[0].RunID))
	assert.Equal(t, engine.SubjectRef{Entity: "acme.order", Key: "child-4"}, kids[0].Subject)
	// The child finishes: the parent continues with its output.
	be.publish(be.tenant, orderPaid, "child-4", order{}, 0)
	p.until(func() bool { return len(be.effects(pr[0].RunID)) == 1 }, "the parent continues")
	assert.Equal(t, []string{"ok:child finished"}, be.effects(pr[0].RunID))
	s := be.step(p, pr[0].RunID, "call")
	assert.Equal(t, "done", s.Outcome)
	assert.Contains(t, string(s.Output), kids[0].RunID.String())
	var depth int
	require.NoError(t, be.db.QueryRow(context.Background(), `SELECT depth FROM automation.runs WHERE id = $1`, kids[0].RunID).Scan(&depth))
	assert.Equal(t, 1, depth, "the child counts toward the cascade")
}

func TestCancellingTheParentCancelsTheChildOfRunAutomation(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	child := be.create(p, a, childGraph)
	parent := be.create(p, a, parentGraph("run_automation", child.ID))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 2}, 0)
	var kids []engine.RunSummary
	p.until(func() bool {
		kids = be.runsOf(p, child.ID)
		return len(kids) == 1 && kids[0].Status == engine.RunWaiting
	}, "the child runs")
	pr := be.runsOf(p, parent.ID)
	require.NoError(t, p.rt.Service().Cancel(context.Background(), a, pr[0].RunID))
	p.until(func() bool { return be.status(p, kids[0].RunID) == engine.RunCancelled }, "the child is cancelled with its parent")
	assert.Equal(t, engine.RunCancelled, be.status(p, pr[0].RunID))
}

func TestStartAutomationLeavesTheChildAlone(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	child := be.create(p, a, childGraph)
	parent := be.create(p, a, parentGraph("start_automation", child.ID))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 3}, 0)
	var kids []engine.RunSummary
	p.until(func() bool {
		kids = be.runsOf(p, child.ID)
		return len(kids) == 1 && kids[0].Status == engine.RunWaiting
	}, "the child runs")
	pr := be.runsOf(p, parent.ID)
	p.until(func() bool { return len(be.effects(pr[0].RunID)) == 1 }, "the parent continues at once")
	assert.Equal(t, []string{"ok:started"}, be.effects(pr[0].RunID))
	require.NoError(t, p.rt.Service().Cancel(context.Background(), a, pr[0].RunID))
	p.until(func() bool { return be.status(p, pr[0].RunID) == engine.RunCancelled }, "the parent is cancelled")
	p.pass()
	assert.Equal(t, engine.RunWaiting, be.status(p, kids[0].RunID), "the child of start_automation lives on")
}

func TestStartingADisabledGraphFailsNotEnabled(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	child := be.create(p, a, childGraph)
	_, err := p.rt.Service().Disable(context.Background(), a, child.ID)
	require.NoError(t, err)
	parent := be.create(p, a, parentGraph("start_automation", child.ID))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 3}, 0)
	var pr []engine.RunSummary
	p.until(func() bool { pr = be.runsOf(p, parent.ID); return len(pr) == 1 && len(be.effects(pr[0].RunID)) == 1 }, "the parent takes failed")
	assert.Equal(t, []string{"bad:not_enabled"}, be.effects(pr[0].RunID))
	assert.Empty(t, be.runsOf(p, child.ID))
}

func TestAutomationCycleStopsAtTheCascadeLimit(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc(func(c *engine.Config) { c.Limits.MaxDepth = 3 })
	a := be.human()
	// A graph that starts itself from its manual trigger: A → A → A ...
	ctx := context.Background()
	self := `{"name": "Loop", "triggers": [{"id": "m", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "call", "type": "start_automation", "params": {"automation_id": "SELF"}},
	  {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.call.output.error.code }}"}},
	  {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "m", "outcome": "fired"}, "to": "call"},
	  {"from": {"node": "call", "outcome": "started"}, "to": "fin"}, {"from": {"node": "call", "outcome": "failed"}, "to": "bad"},
	  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}]}`
	placeholder := strings.Replace(self, "SELF", uuid.Nil.String(), 1)
	am, v, err := p.rt.Service().Create(ctx, a, engine.CreateInput{Graph: json.RawMessage(placeholder)})
	require.NoError(t, err)
	v, err = p.rt.Service().SaveVersion(ctx, a, am.ID, engine.SaveInput{BaseVersion: v.Version, Graph: json.RawMessage(strings.Replace(self, "SELF", am.ID.String(), 1))})
	require.NoError(t, err)
	_, err = p.rt.Service().Enable(ctx, a, am.ID, engine.EnableInput{Version: v.Version})
	require.NoError(t, err)
	_, err = p.rt.Service().StartManual(ctx, a, am.ID, engine.ManualStart{TriggerID: "m", Input: json.RawMessage(`{}`)})
	require.NoError(t, err)
	var runs []engine.RunSummary
	p.until(func() bool {
		runs = be.runsOf(p, am.ID)
		for _, r := range runs {
			if len(be.effects(r.RunID)) > 0 {
				return true
			}
		}
		return false
	}, "the cascade stops")
	assert.Len(t, runs, 4, "depths 0..3 run, the fifth is refused")
	var codes []string
	for _, r := range runs {
		codes = append(codes, be.effects(r.RunID)...)
	}
	assert.Equal(t, []string{"bad:depth_exceeded"}, codes)
}

func TestStartingAGraphNeedsOperate(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	child := be.create(p, a, childGraph)
	parent := be.create(p, a, parentGraph("start_automation", child.ID))
	// The owner may still run graphs but not start other ones.
	var without []string
	for _, perm := range allPerms {
		if perm != engine.PermOperate {
			without = append(without, perm)
		}
	}
	be.subjects.Grant(1, without...)
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 3}, 0)
	var pr []engine.RunSummary
	p.until(func() bool { pr = be.runsOf(p, parent.ID); return len(pr) == 1 && len(be.effects(pr[0].RunID)) == 1 }, "the parent takes failed")
	assert.Equal(t, []string{"bad:forbidden"}, be.effects(pr[0].RunID))
	assert.Empty(t, be.runsOf(p, child.ID))
}

func TestStartingAGraphWithoutAnEntryFailsNoEntry(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	// An enabled graph with only an event trigger: nothing starts it from another automation.
	target := be.create(p, a, strings.Replace(strings.Replace(childGraph, `"event": "automation.called.v1"`, `"event": "acme.order.created.v1"`, 1),
		`child {{ event.input.amount }}`, `child {{ event.amount }}`, 1))
	parent := be.create(p, a, parentGraph("start_automation", target.ID))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 3}, 0)
	var pr []engine.RunSummary
	p.until(func() bool { pr = be.runsOf(p, parent.ID); return len(pr) == 1 && len(be.effects(pr[0].RunID)) == 1 }, "the parent takes failed")
	assert.Equal(t, []string{"bad:no_entry"}, be.effects(pr[0].RunID))
}

// The step's context a Code node reads is not in the call's workflow instance input, which the workflow
// API shows without the automation's field rules.
func TestCodeContextIsNotInTheInstanceInput(t *testing.T) {
	be := newBuiltinEnv(t)
	p := be.proc()
	a := be.human()
	be.create(p, a, sandboxGraph(codeParams(`export default async function run(ctx) {
		return { outcome: "done", output: { value: ctx.event.secret } };
	}`, "")))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 1, Secret: "classified-77"}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the code runs")
	assert.Equal(t, []string{"ok:classified-77"}, be.effects(run.RunID), "the code read the context")
	assert.Zero(t, be.count(`SELECT count(*) FROM extruntime.workflow_instances WHERE workflow = 'automation.call' AND input::text LIKE '%classified-77%'`))
	assert.Zero(t, be.count(`SELECT count(*) FROM automation.call_contexts`), "the context goes with the call's result")
}
