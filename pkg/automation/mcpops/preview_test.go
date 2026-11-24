package mcpops_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/automation/mcpops"
)

// packagePreview previews by running what stands for a package's code (a contact resolver): it must never
// run in an agent's dry run.
type packagePreview struct {
	specNode
	calls *atomic.Int64
}

func (p packagePreview) PreviewEffect(context.Context, engine.StepContext, graph.Node, graph.Effect) (engine.EffectResult, error) {
	p.calls.Add(1)
	return engine.EffectResult{Outcome: "done", Output: map[string]any{"by": "package"}}, nil
}

// localPreview previews from the engine's own data and honors StepContext.Offline.
type localPreview struct {
	specNode
	offline *atomic.Int64
}

func (p localPreview) PreviewEffect(_ context.Context, run engine.StepContext, _ graph.Node, _ graph.Effect) (engine.EffectResult, error) {
	if run.Offline {
		p.offline.Add(1)
	}
	return engine.EffectResult{Outcome: "done", Output: map[string]any{"by": "local"}}, nil
}

func (localPreview) PreviewsOffline() {}

func previewGraph() map[string]any {
	var g map[string]any
	if err := json.Unmarshal([]byte(`{"name": "P", "triggers": [{"id": "created", "event": "acme.client.created.v1"}],
	 "nodes": [{"id": "pkg", "type": "acme.pkgpreview", "params": {}}, {"id": "loc", "type": "acme.localpreview", "params": {}},
	  {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "pkg"}, {"from": {"node": "pkg", "outcome": "done"}, "to": "loc"},
	  {"from": {"node": "loc", "outcome": "done"}, "to": "fin"}]}`), &g); err != nil {
		panic(err)
	}
	return g
}

func TestAgentDryRunPreviewsOnlyOffline(t *testing.T) {
	e := newEnv(t)
	pkgCalls, offline := &atomic.Int64{}, &atomic.Int64{}
	spec := func(typ string) graph.NodeSpec {
		return graph.NodeSpec{Type: typ, Title: typ, Behavior: graph.BehaviorEffect, DryRun: graph.DryRunCapture, Outcomes: []string{"done"},
			Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}
	}
	require.NoError(t, e.reg.RegisterNode(packagePreview{specNode{spec("acme.pkgpreview")}, pkgCalls}))
	require.NoError(t, e.reg.RegisterNode(localPreview{specNode{spec("acme.localpreview")}, offline}))

	for _, decisions := range []string{"scripted", "cached"} {
		e.ok(agentID, mcpops.ToolDryRun, map[string]any{"graph": previewGraph(), "subject": map[string]any{"entity": "acme.client", "key": "c-1"},
			"events": []any{map[string]any{"kind": evClient}}, "decisions": decisions}, nil)
	}
	assert.Zero(t, pkgCalls.Load(), "no package code runs in an agent's dry run")
	assert.EqualValues(t, 2, offline.Load(), "the local preview runs, offline")

	// The editor's dry run (all real calls) previews as before.
	_, err := e.rt.Authoring().DryRun(context.Background(), e.human(), engine.DryRunInput{Graph: mustJSON(previewGraph()),
		Subject: engine.SubjectRef{Entity: "acme.client", Key: "c-1"}, Events: []json.RawMessage{mustJSON(map[string]any{"kind": evClient})}})
	require.NoError(t, err)
	assert.EqualValues(t, 1, pkgCalls.Load())
}
