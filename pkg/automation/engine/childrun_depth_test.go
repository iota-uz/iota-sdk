package engine_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// A conversation's runs that wait for each other (run_automation) may nest down to the cascade limit,
// however deep it is: every waiting ancestor is found, none is taken for another holder of the
// conversation (subject_busy).
func TestRunAutomationNestsOnAConversationDownToTheCascadeLimit(t *testing.T) {
	const depth = 66 // past the 64 levels the walk of the ancestors once stopped at
	ae := newAcmeEnv(t)
	var rt atomic.Pointer[engine.Runtime]
	require.NoError(t, builtin.Register(ae.reg, builtin.Deps{Runs: func() (engine.RunStarter, error) { return rt.Load(), nil }}))
	p := ae.proc(func(c *engine.Config) { c.Limits.MaxDepth = depth })
	rt.Store(p.rt)
	a := ae.human()
	ctx := context.Background()
	// A graph that waits for itself on the caller's conversation: Loop → Loop → ... until the limit.
	self := `{"name": "Loop", "triggers": [{"id": "m", "event": "automation.called.v1"}],
	 "nodes": [{"id": "call", "type": "run_automation", "params": {"automation_id": "SELF"}},
	  {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.call.output.error.code }}"}},
	  {"id": "fin", "type": "end", "params": {"reason": "nested"}}],
	 "edges": [{"from": {"node": "m", "outcome": "fired"}, "to": "call"},
	  {"from": {"node": "call", "outcome": "done"}, "to": "fin"}, {"from": {"node": "call", "outcome": "failed"}, "to": "bad"},
	  {"from": {"node": "call", "outcome": "timeout"}, "to": "bad"},
	  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}]}`
	loop, v, err := p.rt.Service().Create(ctx, a, engine.CreateInput{Graph: json.RawMessage(strings.Replace(self, "SELF", uuid.Nil.String(), 1))})
	require.NoError(t, err)
	v, err = p.rt.Service().SaveVersion(ctx, a, loop.ID, engine.SaveInput{BaseVersion: v.Version, Graph: json.RawMessage(strings.Replace(self, "SELF", loop.ID.String(), 1))})
	require.NoError(t, err)
	_, err = p.rt.Service().Enable(ctx, a, loop.ID, engine.EnableInput{Version: v.Version})
	require.NoError(t, err)
	parent := ae.create(p, a, graphJSON("Inbox", `{"id": "msg", "event": "`+evMessage+`"}`,
		`{"id": "call", "type": "run_automation", "params": {"automation_id": "`+loop.ID.String()+`"}},
		 {"id": "ok", "type": "acme.note", "params": {"text": "{{ nodes.call.output.output.reason }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "msg", "outcome": "fired"}, "to": "call"},
		 {"from": {"node": "call", "outcome": "done"}, "to": "ok"}, {"from": {"node": "call", "outcome": "failed"}, "to": "fin"},
		 {"from": {"node": "call", "outcome": "timeout"}, "to": "fin"},
		 {"from": {"node": "ok", "outcome": "done"}, "to": "fin"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"}`))
	ae.hook(ae.handler(p, nil), ae.hookPath(), acmeSig, acmeUpdate{ID: "u-1", Text: "hello", From: "ann"})

	var codes []string
	deadline := time.Now().Add(3 * time.Minute)
	for {
		p.pass()
		codes = codes[:0]
		for _, r := range ae.runs(p, a, engine.RunFilter{AutomationID: &loop.ID, Limit: 100}) {
			codes = append(codes, ae.effects(r.RunID)...)
		}
		if len(codes) > 0 || time.Now().After(deadline) {
			break
		}
	}
	require.Equal(t, []string{"bad:" + engine.CodeDepthExceeded}, codes, "the deepest run stops at the cascade limit, not on a busy conversation")
	assert.Len(t, ae.runs(p, a, engine.RunFilter{AutomationID: &loop.ID, Limit: 100}), depth, "depths 1..limit run")
	p.until(func() bool {
		pr := ae.runs(p, a, engine.RunFilter{AutomationID: &parent.ID})
		return len(pr) == 1 && len(ae.effects(pr[0].RunID)) == 1
	}, "the chain unwinds to the inbound run")
}
