package agents

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

const (
	defaultSlowToolThresholdMs = int64(10_000)
	defaultMaxRemindersPerTurn = 8
)

// ReminderRule emits an optional mid-loop behavioral reminder after a tool batch.
type ReminderRule interface {
	Name() string
	Eval(ctx context.Context, s ReminderState) string
}

// ReminderState captures the executor state visible to a reminder rule.
type ReminderState struct {
	Iteration      int
	MaxIterations  int
	Batch          []ToolCallOutcome
	TurnToolCounts map[string]int
	BatchToolNames map[string]string
}

// ToolCallOutcome describes one completed tool call in the latest batch.
type ToolCallOutcome struct {
	Name       string
	Arguments  string
	DurationMs int64
	Err        error
	Truncated  bool
	RowCount   int
}

// NewDefaultReminderRules returns the generic reminder rules shipped by the SDK.
func NewDefaultReminderRules() []ReminderRule {
	return []ReminderRule{
		NewSingleToolBatchReminderRule(),
		NewIterationBudgetReminderRule(),
		NewSQLTruncationReminderRule(),
		NewEmptyResultReminderRule(),
		NewSlowToolReminderRule(defaultSlowToolThresholdMs, "web_fetch", "web_search", "sql_execute"),
		NewToolErrorReminderRule(),
	}
}

// NewSingleToolBatchReminderRule nudges single-call turns toward available batch tools.
func NewSingleToolBatchReminderRule() ReminderRule {
	return reminderRule{
		name: "single_tool_batch",
		eval: func(ctx context.Context, s ReminderState) string {
			if len(s.Batch) != 1 || totalToolCalls(s.TurnToolCounts) != 1 {
				return ""
			}
			toolName := s.Batch[0].Name
			batchName := s.BatchToolNames[toolName]
			if batchName == "" {
				return ""
			}
			return fmt.Sprintf("This turn used one %s call, but %s is available. When you need related lookups, prefer batching them in one call.", toolName, batchName)
		},
	}
}

// NewIterationBudgetReminderRule nudges the model to converge near the iteration cap.
func NewIterationBudgetReminderRule() ReminderRule {
	return reminderRule{
		name: "iteration_budget",
		eval: func(ctx context.Context, s ReminderState) string {
			if s.MaxIterations <= 0 {
				return ""
			}
			ratio := float64(s.Iteration) / float64(s.MaxIterations)
			if ratio >= 0.95 {
				return fmt.Sprintf("You've used %d/%d steps. Stop exploring, converge on the best supported answer, and clearly state any remaining uncertainty.", s.Iteration, s.MaxIterations)
			}
			if ratio >= 0.80 {
				return fmt.Sprintf("You've used %d/%d steps. Prioritize the shortest path to a final answer over more exploratory tool calls.", s.Iteration, s.MaxIterations)
			}
			return ""
		},
	}
}

// NewSQLTruncationReminderRule nudges the model to narrow truncated SQL results.
func NewSQLTruncationReminderRule() ReminderRule {
	return reminderRule{
		name: "sql_truncation",
		eval: func(ctx context.Context, s ReminderState) string {
			for _, outcome := range s.Batch {
				if outcome.Truncated {
					return "The SQL result was truncated. Tighten the WHERE clause, aggregate at the needed grain, or select fewer columns instead of rerunning the same broad query."
				}
			}
			return ""
		},
	}
}

// NewEmptyResultReminderRule nudges the model to revisit filters after empty results.
func NewEmptyResultReminderRule() ReminderRule {
	return reminderRule{
		name: "empty_result",
		eval: func(ctx context.Context, s ReminderState) string {
			for _, outcome := range s.Batch {
				if outcome.RowCount == 0 {
					return "The result returned 0 rows. Recheck filters, broaden exact matches, or inspect candidate values before concluding nothing exists."
				}
			}
			return ""
		},
	}
}

// NewSlowToolReminderRule nudges the model to avoid redundant slow calls.
func NewSlowToolReminderRule(thresholdMs int64, toolNames ...string) ReminderRule {
	if thresholdMs <= 0 {
		thresholdMs = defaultSlowToolThresholdMs
	}
	tools := make(map[string]struct{}, len(toolNames))
	for _, name := range toolNames {
		name = strings.TrimSpace(name)
		if name != "" {
			tools[name] = struct{}{}
		}
	}
	return reminderRule{
		name: "slow_tool",
		eval: func(ctx context.Context, s ReminderState) string {
			for _, outcome := range s.Batch {
				if len(tools) > 0 {
					if _, ok := tools[outcome.Name]; !ok {
						continue
					}
				}
				if outcome.DurationMs > thresholdMs {
					return fmt.Sprintf("The last %s call was slow. Avoid repeating identical calls; reuse that result or narrow the next request.", outcome.Name)
				}
			}
			return ""
		},
	}
}

// NewToolErrorReminderRule nudges the model to correct failed tool calls before retrying.
func NewToolErrorReminderRule() ReminderRule {
	return reminderRule{
		name: "tool_error",
		eval: func(ctx context.Context, s ReminderState) string {
			for _, outcome := range s.Batch {
				if outcome.Err != nil {
					return "A tool call failed. Use the error details to correct arguments, syntax, or filters before retrying; avoid repeating the same failing call unchanged."
				}
			}
			return ""
		},
	}
}

type reminderRule struct {
	name string
	eval func(context.Context, ReminderState) string
}

func (r reminderRule) Name() string {
	return r.name
}

func (r reminderRule) Eval(ctx context.Context, s ReminderState) string {
	if r.eval == nil {
		return ""
	}
	return strings.TrimSpace(r.eval(ctx, s))
}

type reminderTracker struct {
	emitted  map[string]struct{}
	lastRule string
	count    int
}

func newReminderTracker() *reminderTracker {
	return &reminderTracker{
		emitted: make(map[string]struct{}),
	}
}

func (t *reminderTracker) allow(ruleName, reminder string) bool {
	reminder = strings.TrimSpace(reminder)
	if reminder == "" || t.count >= defaultMaxRemindersPerTurn {
		return false
	}
	if _, exists := t.emitted[reminder]; exists {
		return false
	}
	if ruleName != "" && ruleName == t.lastRule {
		return false
	}
	t.emitted[reminder] = struct{}{}
	t.lastRule = ruleName
	t.count++
	return true
}

func (e *Executor) reminderMessage(ctx context.Context, iteration, maxIterations int, batch []ToolCallOutcome, counts map[string]int, batchForms map[string]string, tracker *reminderTracker) types.Message {
	if len(e.reminderRules) == 0 || len(batch) == 0 || tracker == nil {
		return nil
	}
	state := ReminderState{
		Iteration:      iteration,
		MaxIterations:  maxIterations,
		Batch:          slices.Clone(batch),
		TurnToolCounts: maps.Clone(counts),
		BatchToolNames: maps.Clone(batchForms),
	}

	reminders := make([]string, 0, len(e.reminderRules))
	for _, rule := range e.reminderRules {
		if rule == nil {
			continue
		}
		text := strings.TrimSpace(rule.Eval(ctx, state))
		if tracker.allow(rule.Name(), text) {
			reminders = append(reminders, text)
		}
	}
	if len(reminders) == 0 {
		return nil
	}
	return types.SystemMessage("<system-reminder>\n" + strings.Join(reminders, "\n") + "\n</system-reminder>")
}

func reminderMetadataFromPayload(payload any) (bool, int) {
	switch p := payload.(type) {
	case types.QueryResultFormatPayload:
		return p.Truncated, p.RowCount
	case *types.QueryResultFormatPayload:
		if p == nil {
			return false, -1
		}
		return p.Truncated, p.RowCount
	case types.ExplainPlanPayload:
		return p.Truncated, -1
	case *types.ExplainPlanPayload:
		if p == nil {
			return false, -1
		}
		return p.Truncated, -1
	default:
		return false, -1
	}
}

func toolCallOutcomeFromResult(call types.ToolCall, result toolExecutionResult, err error, durationMs int64) ToolCallOutcome {
	return ToolCallOutcome{
		Name:       call.Name,
		Arguments:  call.Arguments,
		DurationMs: durationMs,
		Err:        err,
		Truncated:  result.truncated,
		RowCount:   result.rowCount,
	}
}

func totalToolCalls(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func buildBatchToolNames(tools []Tool) map[string]string {
	available := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		available[tool.Name()] = struct{}{}
	}
	batchForms := make(map[string]string)
	for name := range available {
		if strings.HasSuffix(name, "_batch") {
			continue
		}
		batchName := name + "_batch"
		if _, ok := available[batchName]; ok {
			batchForms[name] = batchName
		}
	}
	return batchForms
}
