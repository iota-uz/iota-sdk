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

	hitlUsage := &JudgeUsage{
		PromptTokens:     12,
		CompletionTokens: 4,
		TotalTokens:      16,
		Cost:             0.0008,
	}

	metrics := buildEvalMetrics(toolCalls, sseRes, verdict, hitlUsage)
	require.Equal(t, 3, metrics.ToolUseEfficiency)
	require.Equal(t, 2, metrics.UniqueToolsUsed)
	require.Equal(t, 142, metrics.InputTokens)
	require.Equal(t, 54, metrics.OutputTokens)
	require.Equal(t, 196, metrics.TotalTokens)
	require.InDelta(t, 0.0158, metrics.Cost, 0.0000001)
	require.Equal(t, 12, metrics.HITLInputTokens)
	require.Equal(t, 4, metrics.HITLOutputTokens)
	require.Equal(t, 16, metrics.HITLTotalTokens)
	require.InDelta(t, 0.0008, metrics.HITLCost, 0.0000001)
}

func TestAggregateEvalMetricsFromTurns(t *testing.T) {
	t.Parallel()

	turns := []TurnReport{
		{Metrics: EvalMetrics{ToolUseEfficiency: 2, InputTokens: 50, OutputTokens: 20, TotalTokens: 70, Cost: 0.01, HITLInputTokens: 5, HITLOutputTokens: 2, HITLTotalTokens: 7, HITLCost: 0.0003}},
		{Metrics: EvalMetrics{ToolUseEfficiency: 1, InputTokens: 30, OutputTokens: 10, TotalTokens: 40, Cost: 0.005, HITLInputTokens: 3, HITLOutputTokens: 1, HITLTotalTokens: 4, HITLCost: 0.0002}},
	}

	agg := aggregateEvalMetricsFromTurns(turns)
	require.Equal(t, 3, agg.ToolUseEfficiency)
	require.Equal(t, 80, agg.InputTokens)
	require.Equal(t, 30, agg.OutputTokens)
	require.Equal(t, 110, agg.TotalTokens)
	require.InDelta(t, 0.015, agg.Cost, 0.0000001)
	require.Equal(t, 8, agg.HITLInputTokens)
	require.Equal(t, 3, agg.HITLOutputTokens)
	require.Equal(t, 11, agg.HITLTotalTokens)
	require.InDelta(t, 0.0005, agg.HITLCost, 0.0000001)
}
