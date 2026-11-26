package automations_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/graphs"
	"github.com/iota-uz/iota-sdk/pkg/automation/backtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// The section «Backtests» (#1389) through the application: its page and methods need Automations.Author;
// a dataset with a synthetic event and its expected outcome, a person's label over it, a queued backtest
// and its cancel; the editor's count follows.
func TestBacktestsSection(t *testing.T) {
	s, container := app(t, allAutomations...)
	var created graphs.SaveResult
	require.Nil(t, rpc(t, s, graphs.MethodCreate, map[string]any{"graph": codeGraph("Notes", "return 1"), "layout": map[string]any{}}, &created).Error)
	require.True(t, created.OK)

	s.GET("/automations/" + created.ID + "/backtests").Expect(t).Status(http.StatusOK).Contains(`"view":"backtests"`).Contains(`"backtests":true`)

	var ds graphs.IDResponse
	require.Nil(t, rpc(t, s, graphs.MethodDatasetCreate, map[string]any{"automation_id": created.ID, "name": "Hand-made"}, &ds).Error)
	var ex graphs.IDResponse
	require.Nil(t, rpc(t, s, graphs.MethodDatasetSynthetic, map[string]any{"dataset_id": ds.ID, "kind": engine.EventManual,
		"payload": map[string]any{"input": map[string]any{"x": 1}}, "title": "By hand", "expected": map[string]any{"code": "done"}}, &ex).Error)
	var est graphs.EstimateResponse
	require.Nil(t, rpc(t, s, graphs.MethodBacktestEstimate, map[string]any{"dataset_id": ds.ID, "decisions": "live"}, &est).Error)
	assert.Equal(t, 1, est.Examples)
	assert.Empty(t, est.DecisionNodes)
	var run graphs.IDResponse
	require.Nil(t, rpc(t, s, graphs.MethodBacktestRun, map[string]any{"dataset_id": ds.ID}, &run).Error)

	// The module composes the backtest service and its worker (the worker runs it in the background;
	// pkg/automation/backtest tests what a run does).
	svc, err := automations.Backtests(container)
	require.NoError(t, err)
	_, ok := svc.(*backtest.Svc)
	require.True(t, ok)
	var rep graphs.ReportResponse
	require.Nil(t, rpc(t, s, graphs.MethodBacktestReport, map[string]any{"backtest_id": run.ID}, &rep).Error)
	assert.Equal(t, graphs.BacktestQueued, rep.Backtest.Status)
	assert.Equal(t, 1, rep.Backtest.Total)
	var page graphs.ExamplesResponse
	require.Nil(t, rpc(t, s, graphs.MethodDatasetExamples, map[string]any{"dataset_id": ds.ID}, &page).Error)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "By hand", page.Items[0].Title)
	assert.Equal(t, graphs.LabelView{Outcome: "done", Source: graphs.LabelExpected}, page.Items[0].Labels["code"])
	require.Nil(t, rpc(t, s, graphs.MethodExampleLabel, map[string]any{"example_id": ex.ID, "node": "code", "outcome": "failed"}, nil).Error)
	require.Nil(t, rpc(t, s, graphs.MethodDatasetExamples, map[string]any{"dataset_id": ds.ID}, &page).Error)
	assert.Equal(t, graphs.LabelView{Outcome: "failed", Source: graphs.LabelManual, Expected: "done"}, page.Items[0].Labels["code"])
	require.Nil(t, rpc(t, s, graphs.MethodBacktestCancel, map[string]any{"backtest_id": run.ID}, nil).Error)
	r := rpc(t, s, graphs.MethodBacktestCancel, map[string]any{"backtest_id": run.ID}, nil)
	require.NotNil(t, r.Error)
	assert.Equal(t, string(graphs.ErrBacktestFinished), r.Error.Code)

	var count graphs.BacktestCountResponse
	require.Nil(t, rpc(t, s, graphs.MethodBacktestCount, map[string]any{"automation_id": created.ID}, &count).Error)
	assert.Equal(t, 1, count.Count)

	// A dataset of another tenant is not found.
	other, _ := app(t, allAutomations...)
	r = rpc(t, other, graphs.MethodDatasetExamples, map[string]any{"dataset_id": ds.ID}, nil)
	require.NotNil(t, r.Error)
	assert.Equal(t, "not_found", r.Error.Code)

	// Without Automations.Author neither the page nor its methods (the suite now acts as a reader).
	reader := s.AsUser(sameTenant(s, permissions.AutomationsRead))
	reader.GET("/automations/" + created.ID + "/backtests").Expect(t).Status(http.StatusForbidden)
	r = rpc(t, reader, graphs.MethodBacktests, map[string]any{"automation_id": created.ID}, nil)
	require.NotNil(t, r.Error)
	assert.Equal(t, "forbidden", r.Error.Code)
}
