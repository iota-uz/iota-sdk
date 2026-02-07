package testharness

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/require"
)

func TestBuildEvalMetrics_ComputesToolsTokensCost(t *testing.T) {
	t.Parallel()

	toolCalls := []ToolCall{
		{Name: "sql_query", Arguments: "{\"q\":1}"},
		{Name: "sql_query", Arguments: "{\"q\":2}"},
		{Name: "kb_lookup", Arguments: "{\"k\":\"x\"}"},
	}

	sseRes := &StreamResult{
		Usage: &types.DebugUsage{
			PromptTokens:     100,
			CompletionTokens: 40,
			TotalTokens:      140,
			Cost:             0.012,
		},
	}

	verdict := &JudgeVerdict{
		Usage: &JudgeUsage{
			PromptTokens:     30,
			CompletionTokens: 10,
			TotalTokens:      40,
			Cost:             0.003,
		},
	}

	metrics := buildEvalMetrics(toolCalls, sseRes, verdict)
	require.Equal(t, 3, metrics.ToolUseEfficiency)
	require.Equal(t, 2, metrics.UniqueToolsUsed)
	require.Equal(t, 130, metrics.InputTokens)
	require.Equal(t, 50, metrics.OutputTokens)
	require.Equal(t, 180, metrics.TotalTokens)
	require.InDelta(t, 0.015, metrics.Cost, 0.0000001)
}

func TestAggregateEvalMetricsFromTurns(t *testing.T) {
	t.Parallel()

	turns := []TurnReport{
		{Metrics: EvalMetrics{ToolUseEfficiency: 2, InputTokens: 50, OutputTokens: 20, TotalTokens: 70, Cost: 0.01}},
		{Metrics: EvalMetrics{ToolUseEfficiency: 1, InputTokens: 30, OutputTokens: 10, TotalTokens: 40, Cost: 0.005}},
	}

	agg := aggregateEvalMetricsFromTurns(turns)
	require.Equal(t, 3, agg.ToolUseEfficiency)
	require.Equal(t, 80, agg.InputTokens)
	require.Equal(t, 30, agg.OutputTokens)
	require.Equal(t, 110, agg.TotalTokens)
	require.InDelta(t, 0.015, agg.Cost, 0.0000001)
}
