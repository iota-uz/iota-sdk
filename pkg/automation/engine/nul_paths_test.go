package engine_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// failingNode fails every call with err; it may not be repeated.
type failingNode struct {
	specNode
	err error
}

func (failingNode) Idempotent() bool { return false }
func (n failingNode) Execute(context.Context, catalog.CallInput) (catalog.CallResult, error) {
	return catalog.CallResult{}, n.err
}

func registerFailing(t *testing.T, be *builtinEnv, err error) {
	t.Helper()
	spec := graph.NodeSpec{Type: "acme.failing", Title: "Failing", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture, Outcomes: []string{"done", "failed"}, FailedOutcome: "failed",
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`), Output: expr.Fields{{Name: "body", Field: expr.StringField}}}
	require.NoError(t, be.reg.RegisterNode(failingNode{specNode{spec}, err}))
}

// A provider's error message with a NUL byte is recorded (journal, steps, output) with U+FFFD, and a code
// with one is no code of the node's (effect_failed): the call takes its failed outcome.
func TestACallErrorWithANulByteTakesTheFailedOutcome(t *testing.T) {
	be := newBuiltinEnv(t)
	registerFailing(t, be, &catalog.CallError{Code: "bad\x00code", Message: "refused\x00 here"})
	p := be.proc()
	a := be.human()
	be.create(p, a, graphJSON("Nul", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "get", "type": "acme.failing", "params": {}},
		 {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.get.output.error.code }} {{ nodes.get.output.error.message }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "get"},
		 {"from": {"node": "get", "outcome": "done"}, "to": "fin"}, {"from": {"node": "get", "outcome": "failed"}, "to": "bad"},
		 {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}`))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the run ends")
	assert.Equal(t, []string{"bad:" + engine.CodeEffectFailed + " refused\uFFFD here"}, be.effects(run.RunID))
	s := be.step(p, run.RunID, "get")
	require.NotNil(t, s.Error)
	assert.Equal(t, "refused\uFFFD here", s.Error.Message)
}

// A call whose outcome is unknown, with a NUL byte in the provider's message, blocks as ambiguous: the
// instance's blocked detail keeps the message with U+FFFD.
func TestAnUnknownOutcomeWithANulByteBlocks(t *testing.T) {
	be := newBuiltinEnv(t)
	registerFailing(t, be, errors.New("connection reset\x00 by peer"))
	p := be.proc()
	a := be.human()
	be.create(p, a, graphJSON("Nul", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "get", "type": "acme.failing", "params": {}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "get"}, {"from": {"node": "get", "outcome": "done"}, "to": "fin"},
		 {"from": {"node": "get", "outcome": "failed"}, "to": "ignore"}`))
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunBlocked }, "the run blocks on the call")
	assert.Equal(t, 1, be.count(`SELECT count(*) FROM extruntime.workflow_instances
		WHERE workflow = $1 AND status = 'blocked' AND blocked_reason = 'ambiguous' AND blocked_detail LIKE '%reset' || chr(65533) || ' by peer%'`, engine.CallWorkflowID))
}

// An SDK action that refuses its args with a NUL byte in the message fails with invalid_args: the
// workflow journal and the host's record keep the message with U+FFFD.
func TestActionInvalidArgsWithANulByteFails(t *testing.T) {
	be := newBuiltinEnv(t)
	withPerm := append(append([]string{}, allPerms...), permBlock)
	be.subjects.Grant(1, withPerm...)
	be.action.err = fmt.Errorf("%w: user\x00id", actions.ErrInvalidArgs)
	p := be.proc()
	a := be.actor(be.tenant, 1, engine.OriginHuman, withPerm...)
	be.create(p, a, actionGraph())
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 9}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the run ends by failed")
	assert.Equal(t, []string{"bad:invalid_args"}, be.effects(run.RunID))
	assert.Equal(t, int32(1), be.action.calls.Load(), "the command ran once")
	assert.Equal(t, 1, be.count(`SELECT count(*) FROM extruntime.workflow_occurrences WHERE name = 'command' AND status = 'failed' AND error->>'message' LIKE '%user' || chr(65533) || 'id%'`))
}
