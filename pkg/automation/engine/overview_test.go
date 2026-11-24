package engine_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// A layout save names the revision it was made over: a save from a stale tab is refused with the layout
// of now, and a version saved with a layout moves the revision as well.
func TestSaveLayoutAtRefusesAStaleRevision(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	svc := p.rt.Service()
	am := e.create(p, human, noteGraph("x"))
	assert.Equal(t, 0, am.LayoutRevision)

	rev, err := svc.SaveLayoutAt(ctx, human, am.ID, 0, graph.Layout{"note": {X: 10, Y: 20}})
	require.NoError(t, err)
	assert.Equal(t, 1, rev, "the new tab saved")

	_, err = svc.SaveLayoutAt(ctx, human, am.ID, 0, graph.Layout{"note": {X: 99, Y: 99}})
	var conflict *engine.LayoutConflictError
	require.ErrorAs(t, err, &conflict, "the old tab is refused")
	require.ErrorIs(t, err, engine.ErrVersionConflict)
	assert.Equal(t, 1, conflict.Revision)
	assert.Equal(t, graph.XY{X: 10, Y: 20}, conflict.Layout["note"], "the positions of the new tab stay")

	got, err := svc.Get(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Equal(t, graph.XY{X: 10, Y: 20}, got.Layout["note"])

	_, err = svc.SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(noteGraph("y")), Layout: graph.Layout{"fin": {X: 1, Y: 1}}})
	require.NoError(t, err)
	got, err = svc.Get(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.LayoutRevision, "a version saved with a layout is a layout change")

	_, err = svc.SaveLayoutAt(ctx, human, am.ID, -1, graph.Layout{})
	require.ErrorIs(t, err, engine.ErrInvalidInput)
	reader := e.actor(e.tenant, 1, engine.OriginHuman, engine.PermRead)
	_, err = svc.SaveLayoutAt(ctx, reader, am.ID, 2, graph.Layout{})
	require.ErrorIs(t, err, engine.ErrForbidden)
}

// The list's overview of a graph: who saved the latest version, its triggers, the last run and the runs
// that need attention; the attention filter keeps only graphs that have some.
func TestOverviewsAndTheAttentionFilter(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	svc := p.rt.Service()

	quiet := e.create(p, human, noteGraph("quiet"))
	failing := e.create(p, human, `{"name": "Filtered", "triggers": [{"id": "created", "event": "acme.order.created.v1", "filter": "10 / event.amount > 1"}],
	 "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "fin"}]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 0}, 0)
	p.until(func() bool {
		return len(e.runs(p, human, engine.RunFilter{AutomationID: &failing.ID, Status: []engine.RunStatus{engine.RunFilterFailed}})) == 1
	}, "the filter fails")
	p.until(func() bool {
		return len(e.runs(p, human, engine.RunFilter{AutomationID: &quiet.ID, Status: []engine.RunStatus{engine.RunDone}})) == 1
	}, "the quiet graph's run is done")

	agent := e.actor(e.tenant, 2, engine.OriginAgent)
	_, err := svc.SaveVersion(ctx, agent, quiet.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(noteGraph("agent"))})
	require.NoError(t, err)

	other := uuid.New()
	ov, err := svc.Overviews(ctx, human, []uuid.UUID{quiet.ID, failing.ID, other})
	require.NoError(t, err)
	require.Len(t, ov, 2, "an unknown id is left out")

	q := ov[quiet.ID]
	assert.Equal(t, int64(2), q.LatestSavedBy)
	assert.Equal(t, engine.OriginAgent, q.LatestSavedOrigin)
	assert.Equal(t, []engine.TriggerReview{{TriggerID: "created", Kind: engine.KindEvent, Event: "acme.order.created.v1"}}, q.Triggers)
	require.NotNil(t, q.LastRun)
	assert.Equal(t, engine.RunDone, q.LastRun.Status)
	assert.Equal(t, 1, q.LastRun.Version)
	assert.Zero(t, q.Attention)

	f := ov[failing.ID]
	require.NotNil(t, f.LastRun)
	assert.Equal(t, engine.RunFilterFailed, f.LastRun.Status)
	assert.Equal(t, 1, f.Attention)

	page, err := svc.List(ctx, human, engine.ListFilter{Attention: true})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, failing.ID, page.Items[0].ID)

	stranger := e.actor(e.newTenant(), 1, engine.OriginHuman)
	ov, err = svc.Overviews(ctx, stranger, []uuid.UUID{quiet.ID})
	require.NoError(t, err)
	assert.Empty(t, ov, "another tenant's graph is not shown")
	_, err = svc.Overviews(ctx, e.actor(e.tenant, 1, engine.OriginHuman, engine.PermAuthor), []uuid.UUID{quiet.ID})
	require.ErrorIs(t, err, engine.ErrForbidden)
}

// CountLive names what a disable cancels: the waiting run and its open wait; nothing once it is done.
func TestCountLive(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, followUp)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 9}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")

	live, err := p.rt.Service().CountLive(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Equal(t, engine.LiveCount{Runs: 1, Waits: 1}, live)

	_, err = p.rt.Service().Disable(ctx, human, am.ID)
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunCancelled }, "disable cancels the waiting run")
	live, err = p.rt.Service().CountLive(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Equal(t, engine.LiveCount{}, live)
	_, err = p.rt.Service().CountLive(ctx, e.actor(e.newTenant(), 1, engine.OriginHuman), am.ID)
	require.ErrorIs(t, err, engine.ErrNotFound)
}

// Enable{Latest} checks the version under the graph's lock: a version saved after the review is not
// skipped. A version saved over a stale layout keeps the positions another tab saved. Runs filter by
// version before the page is cut. The review tells why a version does not enable.
func TestEnableLatestLayoutBaseAndRunsByVersion(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	svc := p.rt.Service()
	am := e.create(p, human, noteGraph("x"))

	rv, err := svc.EnableReview(ctx, human, am.ID, 1)
	require.NoError(t, err)
	assert.Empty(t, rv.Problems)
	_, err = svc.SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(noteGraph("y"))})
	require.NoError(t, err)
	_, err = svc.Enable(ctx, human, am.ID, engine.EnableInput{Version: 1, CodeDigest: rv.CodeDigest, Latest: true})
	require.ErrorIs(t, err, engine.ErrReviewRequired, "v2 was saved after the review of v1")
	_, err = svc.Enable(ctx, human, am.ID, engine.EnableInput{Version: 2, Latest: true})
	require.NoError(t, err)

	got, err := svc.Get(ctx, human, am.ID)
	require.NoError(t, err)
	base := got.LayoutRevision
	_, err = svc.SaveLayoutAt(ctx, human, am.ID, base, graph.Layout{"note": {X: 10, Y: 20}})
	require.NoError(t, err)
	_, err = svc.SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 2, Graph: json.RawMessage(noteGraph("z")),
		Layout: graph.Layout{"note": {X: 99, Y: 99}, "fin": {X: 5, Y: 5}}, LayoutBase: &base})
	require.NoError(t, err)
	got, err = svc.Get(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Equal(t, graph.XY{X: 10, Y: 20}, got.Layout["note"], "the other tab's position stays")

	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	p.until(func() bool { return len(e.runs(p, human, engine.RunFilter{AutomationID: &am.ID})) == 1 }, "a run of v3")
	runs := e.runs(p, human, engine.RunFilter{AutomationID: &am.ID, Version: 1})
	assert.Empty(t, runs)
	runs = e.runs(p, human, engine.RunFilter{AutomationID: &am.ID, Version: 3, Limit: 1})
	require.Len(t, runs, 1)
	assert.Equal(t, 3, runs[0].Version)
}
