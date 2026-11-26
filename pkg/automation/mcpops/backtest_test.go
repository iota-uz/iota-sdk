package mcpops_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/mcpops"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

const decideGraph = `{"name": "Decide", "triggers": [{"id": "c", "event": "acme.client.created.v1"}],
 "nodes": [{"id": "dec", "type": "acme.decide", "params": {"text": "{{ event.name }}"}}, {"id": "fin", "type": "end", "params": {}}],
 "edges": [{"from": {"node": "c", "outcome": "fired"}, "to": "dec"}, {"from": {"node": "dec", "outcome": "done"}, "to": "fin"},
  {"from": {"node": "dec", "outcome": "failed"}, "to": "fin"}]}`

// An agent creates a dataset, adds synthetic events with expected outcomes, runs a cached backtest and
// reads its report: no provider is called (the cache lacks the decision, so it is missing), and live
// decisions are not offered.
func TestAgentBacktestTools(t *testing.T) {
	e := newEnv(t)
	am, _, err := e.rt.Service().Create(context.Background(), e.human(), engine.CreateInput{Graph: json.RawMessage(decideGraph)})
	require.NoError(t, err)

	var ds struct {
		DatasetID string  `json:"dataset_id"`
		ImportID  *string `json:"import_id"`
		PageURL   string  `json:"page_url"`
	}
	e.ok(agentID, mcpops.ToolBacktestDataset, map[string]any{"id": am.ID.String(), "name": "Agent's set", "journal_days": 30}, &ds)
	require.NotNil(t, ds.ImportID)
	var added struct {
		ExampleIDs []string `json:"example_ids"`
	}
	e.ok(agentID, mcpops.ToolBacktestExamples, map[string]any{"dataset_id": ds.DatasetID, "events": []any{
		map[string]any{"kind": evClient, "payload": map[string]any{"name": "Ann"}, "title": "Ann", "expected": map[string]any{"dec": "done"}},
	}}, &added)
	require.Len(t, added.ExampleIDs, 1)
	e.ok(agentID, mcpops.ToolBacktestLabel, map[string]any{"example_id": added.ExampleIDs[0], "node": "dec", "outcome": "done"}, nil)

	var run struct {
		BacktestID string `json:"backtest_id"`
	}
	e.ok(agentID, mcpops.ToolBacktestRun, map[string]any{"dataset_id": ds.DatasetID}, &run)
	for {
		worked, err := e.backtests.RunOnce(context.Background())
		require.NoError(t, err)
		if !worked {
			break
		}
	}
	var rep struct {
		Report struct {
			Backtest struct {
				Status    string `json:"status"`
				Decisions string `json:"decisions"`
			} `json:"backtest"`
			Examples    int `json:"examples"`
			Classifiers []struct {
				Node      string `json:"node"`
				Undecided int    `json:"undecided"`
			} `json:"classifiers"`
			Cost struct {
				Missing int `json:"missing"`
			} `json:"cost"`
		} `json:"report"`
	}
	e.ok(agentID, mcpops.ToolBacktestReport, map[string]any{"backtest_id": run.BacktestID}, &rep)
	assert.Equal(t, "done", rep.Report.Backtest.Status)
	assert.Equal(t, "cached", rep.Report.Backtest.Decisions)
	assert.Equal(t, 1, rep.Report.Examples)
	assert.Equal(t, 1, rep.Report.Cost.Missing)
	require.Len(t, rep.Report.Classifiers, 1)
	assert.Equal(t, 1, rep.Report.Classifiers[0].Undecided)
	assert.Zero(t, e.decided.Load(), "an agent's backtest calls no provider")

	var list struct {
		Datasets []struct {
			Examples int `json:"examples"`
		} `json:"datasets"`
		Backtests []struct {
			ID string `json:"id"`
		} `json:"backtests"`
	}
	e.ok(agentID, mcpops.ToolBacktests, map[string]any{"id": am.ID.String()}, &list)
	require.Len(t, list.Datasets, 1)
	assert.Equal(t, 1, list.Datasets[0].Examples)
	require.Len(t, list.Backtests, 1)

	// A reader may not backtest.
	_ = e.failure(readerID, mcpops.ToolBacktestRun, map[string]any{"dataset_id": ds.DatasetID}, mcp.CodeForbidden)
}
