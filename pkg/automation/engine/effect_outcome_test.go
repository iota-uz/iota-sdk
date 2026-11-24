package engine_test

// Effect nodes that pick their outcome (OutcomeEffectNode), preview themselves in a dry run
// (EffectPreviewer) and check the graph that uses them (GraphChecker).

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// seen is what a pick node saw of the step it applied.
type seen struct {
	key     string
	node    string
	trigger *engine.TriggerEvent
}

// pickNode takes the outcome its "pick" param names, and fails when it names "boom".
type pickNode struct {
	specNode
	mu   sync.Mutex
	seen []seen
}

func (n *pickNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (map[string]any, error) {
	res, err := n.ApplyEffectResult(ctx, tx, run, node, e)
	return res.Output, err
}

func (n *pickNode) ApplyEffectResult(_ context.Context, _ pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (engine.EffectResult, error) {
	n.mu.Lock()
	n.seen = append(n.seen, seen{key: run.StepKey(), node: run.NodeID, trigger: run.Trigger})
	n.mu.Unlock()
	pick, _ := e.Params["pick"].(string)
	if pick == "boom" {
		return engine.EffectResult{}, &engine.StepError{Code: "unavailable", Message: "the platform is down"}
	}
	return engine.EffectResult{Outcome: pick, Output: map[string]any{"picked": pick}}, nil
}

func (n *pickNode) PreviewEffect(_ context.Context, run engine.StepContext, _ graph.Node, e graph.Effect) (engine.EffectResult, error) {
	n.mu.Lock()
	n.seen = append(n.seen, seen{key: "preview", node: run.NodeID, trigger: run.Trigger})
	n.mu.Unlock()
	pick, _ := e.Params["pick"].(string)
	switch pick {
	case "boom":
		return engine.EffectResult{}, errors.New("cannot preview")
	case "none":
		return engine.EffectResult{Outcome: "none", Output: map[string]any{"previewed": true}}, nil
	case "huge":
		return engine.EffectResult{Outcome: "sent", Output: map[string]any{"picked": strings.Repeat("x", 2<<20)}}, nil
	}
	return engine.EffectResult{Outcome: "sent", Output: map[string]any{"previewed": true}}, nil
}

func (n *pickNode) CheckNode(g graph.Graph, index int, _ catalog.View) []graph.GraphError {
	if bad, _ := g.Nodes[index].Params["pick"].(string); bad == "refused" {
		return []graph.GraphError{{Path: "nodes[0].params.pick", Code: graph.CodeParams, Message: "refused by the node's own rule"}}
	}
	return nil
}

func (n *pickNode) steps() []seen {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]seen(nil), n.seen...)
}

func newPickEnv(t *testing.T) (*env, *pickNode) {
	t.Helper()
	e := newEnv(t)
	n := &pickNode{specNode: specNode{graph.NodeSpec{Type: "acme.pick_outcome", Title: "Pick", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunCapture,
		Outcomes: []string{"sent", "none", "failed"}, DoneOutcome: "sent", FailedOutcome: "failed",
		Params: graph.MustSchemaParams(`{"type":"object","properties":{"pick":{"type":"string"}},"required":["pick"],"additionalProperties":false}`),
		Output: expr.Fields{{Name: "picked", Field: expr.OptionalOf(expr.StringField)}, {Name: "previewed", Field: expr.OptionalOf(expr.BoolField)}}}}}
	require.NoError(t, e.reg.RegisterNode(n))
	return e, n
}

func pickGraph(pick string) string {
	return `{"name": "Pick", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "pick", "type": "acme.pick_outcome", "params": {"pick": "` + pick + `"}},
	  {"id": "sent", "type": "acme.note", "params": {"text": "sent"}},
	  {"id": "none", "type": "acme.note", "params": {"text": "none {{ nodes.pick.output.picked }}"}},
	  {"id": "failed", "type": "acme.note", "params": {"text": "failed {{ nodes.pick.output.error.code }}"}},
	  {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "pick"},
	  {"from": {"node": "pick", "outcome": "sent"}, "to": "sent"}, {"from": {"node": "pick", "outcome": "none"}, "to": "none"},
	  {"from": {"node": "pick", "outcome": "failed"}, "to": "failed"},
	  {"from": {"node": "sent", "outcome": "done"}, "to": "fin"}, {"from": {"node": "sent", "outcome": "failed"}, "to": "fin"},
	  {"from": {"node": "none", "outcome": "done"}, "to": "fin"}, {"from": {"node": "none", "outcome": "failed"}, "to": "fin"},
	  {"from": {"node": "failed", "outcome": "done"}, "to": "fin"}, {"from": {"node": "failed", "outcome": "failed"}, "to": "fin"}]}`
}

func TestEffectNodeTakesTheOutcomeItPicks(t *testing.T) {
	for _, tc := range []struct{ pick, effect string }{
		{"sent", "sent:sent"},
		{"none", "none:none none"},
		{"boom", "failed:failed unavailable"},
	} {
		t.Run(tc.pick, func(t *testing.T) {
			e, n := newPickEnv(t)
			p := e.proc()
			a := e.human()
			e.create(p, a, pickGraph(tc.pick))
			e.publish(e.tenant, orderCreated, "o-1", order{Amount: 7}, 2)
			run := onlyRun(t, e, p, a)
			p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
			assert.Equal(t, []string{tc.effect}, e.effects(run.RunID))

			steps := n.steps()
			require.Len(t, steps, 1)
			assert.Equal(t, "pick", steps[0].node)
			assert.Contains(t, steps[0].key, run.RunID.String()+":pick:", "the key of the step names the run and the node")
			require.NotNil(t, steps[0].trigger, "a run on an event knows it")
			assert.Equal(t, evCreated, steps[0].trigger.Key)
			assert.NotEmpty(t, steps[0].trigger.ID)
			assert.JSONEq(t, `{"amount": 7, "note": "", "secret": ""}`, string(steps[0].trigger.Payload), "the engine's own fields are left out")
			assert.EqualValues(t, 2, steps[0].trigger.ActorUserID, "the initiator of the event is the actor of the trigger")
		})
	}
}

func TestEffectNodeCannotTakeAnOutcomeItDoesNotDeclare(t *testing.T) {
	e, _ := newPickEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, pickGraph("sent")) // a valid graph…
	// … and a node that names an outcome outside its declaration fails like any effect error.
	res, err := p.rt.Authoring().TestNode(context.Background(), a, engine.TestNodeInput{Graph: json.RawMessage(pickGraph("elsewhere")), NodeID: "pick"})
	require.NoError(t, err)
	require.NotNil(t, res.Error)
	assert.Equal(t, engine.CodeEffectFailed, res.Error.Code)
}

// A failure is an error, so it rolls the effect back and carries output.error: a node that names its failed
// outcome without one is refused.
func TestEffectNodeCannotPickItsFailedOutcome(t *testing.T) {
	e, _ := newPickEnv(t)
	p := e.proc()
	res, err := p.rt.Authoring().TestNode(context.Background(), e.human(), engine.TestNodeInput{Graph: json.RawMessage(pickGraph("failed")), NodeID: "pick"})
	require.NoError(t, err)
	require.NotNil(t, res.Error)
	assert.Equal(t, engine.CodeEffectFailed, res.Error.Code)
}

// A node test knows the event the author gave it, as a dry run does.
func TestTestNodeKnowsTheTriggerEvent(t *testing.T) {
	e, n := newPickEnv(t)
	p := e.proc()
	res, err := p.rt.Authoring().TestNode(context.Background(), e.human(), engine.TestNodeInput{Graph: json.RawMessage(pickGraph("sent")), NodeID: "pick",
		Subject: &engine.SubjectRef{Entity: "acme.order", Key: "o-1"},
		Context: map[string]json.RawMessage{"event": json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 9, "note": "", "secret": ""}`)}})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	steps := n.steps()
	require.Len(t, steps, 1)
	require.NotNil(t, steps[0].trigger)
	assert.Equal(t, evCreated, steps[0].trigger.Key)
	assert.JSONEq(t, `{"amount": 9, "note": "", "secret": ""}`, string(steps[0].trigger.Payload))
}

func TestTestNodeShowsThePickedOutcome(t *testing.T) {
	e, _ := newPickEnv(t)
	p := e.proc()
	res, err := p.rt.Authoring().TestNode(context.Background(), e.human(), engine.TestNodeInput{Graph: json.RawMessage(pickGraph("none")), NodeID: "pick"})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	assert.Equal(t, "none", res.Outcome)
	assert.JSONEq(t, `{"picked": "none"}`, string(res.Output))
}

func TestDryRunShowsThePreviewOfAnEffectNode(t *testing.T) {
	for _, tc := range []struct {
		pick    string
		outcome string
		next    string
	}{
		{"sent", "sent", "sent"},
		{"none", "none", "none"},
		{"boom", "failed", "failed"},
	} {
		t.Run(tc.pick, func(t *testing.T) {
			e, n := newPickEnv(t)
			p := e.proc()
			res, err := p.rt.Authoring().DryRun(context.Background(), e.human(), engine.DryRunInput{
				Graph:   json.RawMessage(pickGraph(tc.pick)),
				Subject: engine.SubjectRef{Entity: "acme.order", Key: "o-1"},
				Events:  []json.RawMessage{json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 3, "note": "", "secret": ""}`)},
			})
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(res.Trace), 3)
			step := res.Trace[1]
			assert.Equal(t, "pick", step.Node)
			assert.Equal(t, tc.outcome, step.Outcome)
			assert.Equal(t, "captured", step.Status, "nothing was applied")
			assert.Equal(t, tc.next, res.Trace[2].Node, "the preview's outcome is the one the trace follows")
			if tc.pick != "boom" {
				assert.JSONEq(t, `{"previewed": true}`, string(step.Output))
			}
			steps := n.steps()
			require.Len(t, steps, 1)
			assert.Equal(t, "preview", steps[0].key)
			require.NotNil(t, steps[0].trigger, "the preview knows the event the dry run starts on")
			assert.Equal(t, evCreated, steps[0].trigger.Key)
			assert.Zero(t, e.count(`SELECT count(*) FROM automation.runs`))
		})
	}
}

func TestANodeTypeChecksTheGraphThatUsesIt(t *testing.T) {
	e, _ := newPickEnv(t)
	p := e.proc()
	_, _, err := p.rt.Service().Create(context.Background(), e.human(), engine.CreateInput{Graph: json.RawMessage(pickGraph("refused"))})
	var verr *engine.ValidationError
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors, 1)
	assert.Equal(t, "nodes[0].params.pick", verr.Errors[0].Path)
	assert.Equal(t, graph.CodeParams, verr.Errors[0].Code)
}

// The preview reads the event that started the dry run — the first one the trigger took — and keeps it.
func TestDryRunPreviewReadsTheEventThatStartedTheRun(t *testing.T) {
	e, n := newPickEnv(t)
	p := e.proc()
	g := strings.Replace(pickGraph("sent"), `{"id": "created", "event": "acme.order.created.v1"}`, `{"id": "created", "event": "acme.order.created.v1", "filter": "event.amount > 5"}`, 1)
	res, err := p.rt.Authoring().DryRun(context.Background(), e.human(), engine.DryRunInput{
		Graph:   json.RawMessage(g),
		Subject: engine.SubjectRef{Entity: "acme.order", Key: "o-1"},
		Events: []json.RawMessage{
			json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 3, "note": "first", "secret": ""}`),
			json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 9, "note": "second", "secret": ""}`),
		},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Trace), 2)
	steps := n.steps()
	require.Len(t, steps, 1)
	require.NotNil(t, steps[0].trigger)
	assert.JSONEq(t, `{"amount": 9, "note": "second", "secret": ""}`, string(steps[0].trigger.Payload), "the first event failed the filter and started nothing")
	assert.EqualValues(t, 1, steps[0].trigger.ActorUserID, "the author is the initiator of a dry run")
}

// A live run refuses an output over the bound; so does the preview, which takes the failed outcome.
func TestDryRunPreviewOutputOverTheBoundTakesTheFailedOutcome(t *testing.T) {
	e, _ := newPickEnv(t)
	p := e.proc()
	res, err := p.rt.Authoring().DryRun(context.Background(), e.human(), engine.DryRunInput{
		Graph:   json.RawMessage(pickGraph("huge")),
		Subject: engine.SubjectRef{Entity: "acme.order", Key: "o-1"},
		Events:  []json.RawMessage{json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 3, "note": "", "secret": ""}`)},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Trace), 3)
	assert.Equal(t, "pick", res.Trace[1].Node)
	assert.Equal(t, "failed", res.Trace[1].Outcome)
	assert.Equal(t, "failed", res.Trace[2].Node, "the trace follows the failed edge")
	assert.Less(t, len(res.Trace[1].Output), 4096, "the oversized output is not kept")
}

// preparingNode asks for its preparation: it is prepared outside the step's transaction and applied in it with the
// result.
type preparingNode struct {
	specNode
	mu       sync.Mutex
	prepares int
	stamps   map[string][]string // by step key: params.stamp as PrepareEffect and then ApplyEffectResult saw it
	idleInTx []int               // sessions idle in a transaction while PrepareEffect ran
	applied  []*engine.PreparedEffect
	db       interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}
	prepareTo error
}

func (n *preparingNode) PrepareEffect(ctx context.Context, run engine.StepContext, _ graph.Node, e graph.Effect) (any, error) {
	var idle int
	if err := n.db.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state = 'idle in transaction' AND pid <> pg_backend_pid()`).Scan(&idle); err != nil {
		return nil, err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.prepares++
	stamp, _ := e.Params["stamp"].(string)
	n.stamps[run.StepKey()] = append(n.stamps[run.StepKey()], "prepare:"+stamp)
	n.idleInTx = append(n.idleInTx, idle)
	if n.prepareTo != nil {
		return nil, n.prepareTo
	}
	pick, _ := e.Params["pick"].(string)
	return "prepared:" + pick, nil
}

func (n *preparingNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (map[string]any, error) {
	res, err := n.ApplyEffectResult(ctx, tx, run, node, e)
	return res.Output, err
}

func (n *preparingNode) ApplyEffectResult(_ context.Context, _ pgx.Tx, run engine.StepContext, _ graph.Node, e graph.Effect) (engine.EffectResult, error) {
	n.mu.Lock()
	n.applied = append(n.applied, run.Prepared)
	stamp, _ := e.Params["stamp"].(string)
	n.stamps[run.StepKey()] = append(n.stamps[run.StepKey()], "apply:"+stamp)
	n.mu.Unlock()
	if run.Prepared != nil && run.Prepared.Err != nil {
		return engine.EffectResult{}, run.Prepared.Err
	}
	pick, _ := e.Params["pick"].(string)
	got, _ := run.Prepared.Value.(string)
	return engine.EffectResult{Outcome: pick, Output: map[string]any{"picked": got}}, nil
}

func newPreparingEnv(t *testing.T) (*env, *preparingNode) {
	t.Helper()
	e := newEnv(t)
	n := &preparingNode{db: e.db, stamps: map[string][]string{}, specNode: specNode{graph.NodeSpec{Type: "acme.pick_outcome", Title: "Pick", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunCapture,
		Outcomes: []string{"sent", "none", "failed"}, DoneOutcome: "sent", FailedOutcome: "failed",
		Params:    graph.MustSchemaParams(`{"type":"object","properties":{"pick":{"type":"string"},"stamp":{"type":"string"}},"required":["pick"],"additionalProperties":false}`),
		Templates: []string{"stamp"},
		Output:    expr.Fields{{Name: "picked", Field: expr.OptionalOf(expr.StringField)}, {Name: "previewed", Field: expr.OptionalOf(expr.BoolField)}}}}}
	require.NoError(t, e.reg.RegisterNode(n))
	return e, n
}

// An effect that asks for its preparation is prepared before the step's transaction opens — no transaction is
// held then — and applied atomically with the step once, with the result.
func TestEffectIsPreparedOutsideTheStepTransaction(t *testing.T) {
	e, n := newPreparingEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, pickGraph("sent"))
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 7}, 2)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	assert.Equal(t, []string{"sent:sent"}, e.effects(run.RunID))

	n.mu.Lock()
	defer n.mu.Unlock()
	assert.Equal(t, 1, n.prepares, "prepared once for the step")
	assert.Equal(t, []int{0}, n.idleInTx, "no transaction was open while it prepared")
	require.Len(t, n.applied, 1, "and applied once, in the transaction")
	require.NotNil(t, n.applied[0])
	assert.Equal(t, "prepared:sent", n.applied[0].Value)
}

// A preparation that failed reaches the effect, which takes its failed outcome like any failure.
func TestFailedPreparationReachesTheEffect(t *testing.T) {
	e, n := newPreparingEnv(t)
	n.prepareTo = &engine.StepError{Code: "unavailable", Message: "the contact lookup is down"}
	p := e.proc()
	a := e.human()
	e.create(p, a, pickGraph("sent"))
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 7}, 2)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	assert.Equal(t, []string{"failed:failed unavailable"}, e.effects(run.RunID))
}

// A test of the node prepares itself before its transaction, as a run does.
func TestTestNodePreparesTheEffect(t *testing.T) {
	e, n := newPreparingEnv(t)
	p := e.proc()
	res, err := p.rt.Authoring().TestNode(context.Background(), e.human(), engine.TestNodeInput{Graph: json.RawMessage(pickGraph("none")), NodeID: "pick"})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	assert.Equal(t, "none", res.Outcome)
	assert.JSONEq(t, `{"picked": "prepared:none"}`, string(res.Output))
	n.mu.Lock()
	defer n.mu.Unlock()
	assert.Equal(t, 1, n.prepares)
}

// What was prepared is what is applied: an effect whose params depend on `now` renders differently when the step is
// taken again, but the step applies the effect the preparation was made for.
func TestStepAppliesTheEffectItWasPreparedFor(t *testing.T) {
	e, n := newPreparingEnv(t)
	p := e.proc()
	a := e.human()
	g := strings.Replace(pickGraph("sent"), `"params": {"pick": "sent"}`, `"params": {"pick": "sent", "stamp": "{{ now.getMilliseconds() }}-{{ now.getSeconds() }}"}`, 1)
	e.create(p, a, g)
	for i := 0; i < 3; i++ {
		e.publish(e.tenant, orderCreated, "o-"+string(rune('1'+i)), order{Amount: 7}, 2)
	}
	p.until(func() bool { return len(e.runs(p, a, engine.RunFilter{})) == 3 }, "the runs start")
	p.until(func() bool {
		for _, r := range e.runs(p, a, engine.RunFilter{}) {
			if e.status(p, r.RunID) != engine.RunDone {
				return false
			}
		}
		return true
	}, "the runs end")

	n.mu.Lock()
	defer n.mu.Unlock()
	require.Len(t, n.stamps, 3, "one step per run")
	for key, seen := range n.stamps {
		// What the step prepared is what it applied; two steps may render the same instant, which proves nothing.
		require.Len(t, seen, 2, key)
		assert.Equal(t, strings.TrimPrefix(seen[0], "prepare:"), strings.TrimPrefix(seen[1], "apply:"), "the step %s applies the effect it prepared", key)
		assert.True(t, strings.HasPrefix(seen[0], "prepare:") && strings.HasPrefix(seen[1], "apply:"), key)
	}
}
