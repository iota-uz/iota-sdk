package agents_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/require"
)

func TestDefaultReminderRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rule  agents.ReminderRule
		state agents.ReminderState
		want  string
	}{
		{
			name: "single tool batch",
			rule: agents.NewSingleToolBatchReminderRule(),
			state: agents.ReminderState{
				Batch: []agents.ToolCallOutcome{{Name: "schema_describe"}},
				TurnToolCounts: map[string]int{
					"schema_describe": 1,
				},
				BatchToolNames: map[string]string{
					"schema_describe": "schema_describe_batch",
				},
			},
			want: "This turn used one schema_describe call, but schema_describe_batch is available. When you need related lookups, prefer batching them in one call.",
		},
		{
			name: "iteration budget pressure",
			rule: agents.NewIterationBudgetReminderRule(),
			state: agents.ReminderState{
				Iteration:     80,
				MaxIterations: 100,
			},
			want: "You've used 80/100 steps. Prioritize the shortest path to a final answer over more exploratory tool calls.",
		},
		{
			name: "sql truncation",
			rule: agents.NewSQLTruncationReminderRule(),
			state: agents.ReminderState{
				Batch: []agents.ToolCallOutcome{{Name: "sql_execute", Truncated: true}},
			},
			want: "The SQL result was truncated. Tighten the WHERE clause, aggregate at the needed grain, or select fewer columns instead of rerunning the same broad query.",
		},
		{
			name: "empty result",
			rule: agents.NewEmptyResultReminderRule(),
			state: agents.ReminderState{
				Batch: []agents.ToolCallOutcome{{Name: "sql_execute", RowCount: 0}},
			},
			want: "The result returned 0 rows. Recheck filters, broaden exact matches, or inspect candidate values before concluding nothing exists.",
		},
		{
			name: "slow tool",
			rule: agents.NewSlowToolReminderRule(100, "web_fetch"),
			state: agents.ReminderState{
				Batch: []agents.ToolCallOutcome{{Name: "web_fetch", DurationMs: 101}},
			},
			want: "The last web_fetch call was slow. Avoid repeating identical calls; reuse that result or narrow the next request.",
		},
		{
			name: "tool error",
			rule: agents.NewToolErrorReminderRule(),
			state: agents.ReminderState{
				Batch: []agents.ToolCallOutcome{{Name: "web_fetch", Err: errors.New("boom")}},
			},
			want: "A tool call failed. Use the error details to correct arguments, syntax, or filters before retrying; avoid repeating the same failing call unchanged.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.rule.Eval(context.Background(), tt.state))
		})
	}
}

func TestExecutor_ReminderMessageAfterToolBatch(t *testing.T) {
	t.Parallel()

	for _, specEnabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "sync", true: "speculative"}[specEnabled], func(t *testing.T) {
			ctx := context.Background()
			tool := agents.NewTool(
				"do_thing",
				"do_thing",
				map[string]any{},
				func(ctx context.Context, input string) (string, error) { return "ok", nil },
			)
			agent := newMockAgent("reminder-agent", tool)
			model := newMockModel(
				mockResponse{
					content:      "Checking.",
					toolCalls:    []types.ToolCall{{ID: "c1", Name: "do_thing", Arguments: "{}"}},
					finishReason: "tool_calls",
				},
				mockResponse{
					content:      "Done.",
					finishReason: "stop",
				},
			)

			executor := agents.NewExecutor(
				agent,
				model,
				agents.WithSpeculativeTools(specEnabled),
				agents.WithReminderRules(staticReminderRule{name: "test", text: "remember this"}),
			)
			gen := executor.Execute(ctx, agents.Input{
				Messages:  []types.Message{types.UserMessage("go")},
				SessionID: uuidForTest(t),
				TenantID:  uuidForTest(t),
			})
			defer gen.Close()
			drainExecutor(t, ctx, gen)

			requests := model.capturedRequests()
			require.Len(t, requests, 2)
			secondMessages := requests[1].Messages
			require.GreaterOrEqual(t, len(secondMessages), 4)
			require.Equal(t, types.RoleTool, secondMessages[len(secondMessages)-2].Role())
			require.Equal(t, types.RoleSystem, secondMessages[len(secondMessages)-1].Role())
			require.Equal(t, "<system-reminder>\nremember this\n</system-reminder>", secondMessages[len(secondMessages)-1].Content())
		})
	}
}

func TestExecutor_RemindersDeduplicateWithinTurn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tool := agents.NewTool(
		"do_thing",
		"do_thing",
		map[string]any{},
		func(ctx context.Context, input string) (string, error) { return "ok", nil },
	)
	agent := newMockAgent("dedupe-agent", tool)
	model := newMockModel(
		mockResponse{
			toolCalls:    []types.ToolCall{{ID: "c1", Name: "do_thing", Arguments: "{}"}},
			finishReason: "tool_calls",
		},
		mockResponse{
			toolCalls:    []types.ToolCall{{ID: "c2", Name: "do_thing", Arguments: "{}"}},
			finishReason: "tool_calls",
		},
		mockResponse{
			content:      "Done.",
			finishReason: "stop",
		},
	)

	executor := agents.NewExecutor(
		agent,
		model,
		agents.WithSpeculativeTools(false),
		agents.WithReminderRules(staticReminderRule{name: "test", text: "remember this"}),
	)
	gen := executor.Execute(ctx, agents.Input{
		Messages:  []types.Message{types.UserMessage("go")},
		SessionID: uuidForTest(t),
		TenantID:  uuidForTest(t),
	})
	defer gen.Close()
	drainExecutor(t, ctx, gen)

	requests := model.capturedRequests()
	require.Len(t, requests, 3)
	count := 0
	for _, msg := range requests[2].Messages {
		if msg.Role() == types.RoleSystem && msg.Content() == "<system-reminder>\nremember this\n</system-reminder>" {
			count++
		}
	}
	require.Equal(t, 1, count)
}

type staticReminderRule struct {
	name string
	text string
}

func (r staticReminderRule) Name() string {
	return r.name
}

func (r staticReminderRule) Eval(ctx context.Context, s agents.ReminderState) string {
	return r.text
}

func uuidForTest(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}

func drainExecutor(t *testing.T, ctx context.Context, gen types.Generator[agents.ExecutorEvent]) {
	t.Helper()
	for {
		ev, err := gen.Next(ctx)
		if err != nil {
			if errors.Is(err, types.ErrGeneratorDone) {
				return
			}
			t.Fatalf("unexpected generator error: %v", err)
		}
		if ev.Type == agents.EventTypeError {
			t.Fatalf("unexpected executor error: %v", ev.Error)
		}
	}
}
