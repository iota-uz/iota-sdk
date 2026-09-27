package domain

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

const invalidUTF8Replacement = "�"

func normalizeAssistantMessageUTF8(spec AssistantMessageSpec) AssistantMessageSpec {
	spec.Content = validUTF8(spec.Content)
	spec.ToolCalls = normalizeToolCallsUTF8(spec.ToolCalls)
	spec.DebugTrace = normalizeDebugTraceUTF8(spec.DebugTrace)
	return spec
}

func validUTF8(value string) string {
	return strings.ToValidUTF8(value, invalidUTF8Replacement)
}

func normalizeToolCallsUTF8(toolCalls []types.ToolCall) []types.ToolCall {
	if toolCalls == nil {
		return nil
	}

	normalized := make([]types.ToolCall, len(toolCalls))
	for i, toolCall := range toolCalls {
		normalized[i] = types.ToolCall{
			ID:         validUTF8(toolCall.ID),
			Name:       validUTF8(toolCall.Name),
			Arguments:  validUTF8(toolCall.Arguments),
			Result:     validUTF8(toolCall.Result),
			Error:      validUTF8(toolCall.Error),
			DurationMs: toolCall.DurationMs,
		}
	}
	return normalized
}

func normalizeDebugToolCallsUTF8(toolCalls []types.DebugToolCall) []types.DebugToolCall {
	if toolCalls == nil {
		return nil
	}

	normalized := make([]types.DebugToolCall, len(toolCalls))
	for i, toolCall := range toolCalls {
		normalized[i] = types.DebugToolCall{
			CallID:     validUTF8(toolCall.CallID),
			Name:       validUTF8(toolCall.Name),
			Arguments:  validUTF8(toolCall.Arguments),
			Result:     validUTF8(toolCall.Result),
			Error:      validUTF8(toolCall.Error),
			DurationMs: toolCall.DurationMs,
		}
	}
	return normalized
}

func normalizeDebugTraceUTF8(trace *types.DebugTrace) *types.DebugTrace {
	if trace == nil {
		return nil
	}

	normalized := *trace
	normalized.SchemaVersion = validUTF8(trace.SchemaVersion)
	normalized.StartedAt = validUTF8(trace.StartedAt)
	normalized.CompletedAt = validUTF8(trace.CompletedAt)
	normalized.TraceID = validUTF8(trace.TraceID)
	normalized.TraceURL = validUTF8(trace.TraceURL)
	normalized.SessionID = validUTF8(trace.SessionID)
	normalized.Thinking = validUTF8(trace.Thinking)
	normalized.ObservationReason = validUTF8(trace.ObservationReason)
	normalized.Tools = normalizeDebugToolCallsUTF8(trace.Tools)

	if trace.Attempts != nil {
		normalized.Attempts = make([]types.DebugGeneration, len(trace.Attempts))
		for i, attempt := range trace.Attempts {
			attempt.ID = validUTF8(attempt.ID)
			attempt.RequestID = validUTF8(attempt.RequestID)
			attempt.Model = validUTF8(attempt.Model)
			attempt.Provider = validUTF8(attempt.Provider)
			attempt.FinishReason = validUTF8(attempt.FinishReason)
			attempt.Input = validUTF8(attempt.Input)
			attempt.Output = validUTF8(attempt.Output)
			attempt.Thinking = validUTF8(attempt.Thinking)
			attempt.ObservationReason = validUTF8(attempt.ObservationReason)
			attempt.StartedAt = validUTF8(attempt.StartedAt)
			attempt.CompletedAt = validUTF8(attempt.CompletedAt)
			attempt.ToolCalls = normalizeDebugToolCallsUTF8(attempt.ToolCalls)
			normalized.Attempts[i] = attempt
		}
	}

	if trace.Spans != nil {
		normalized.Spans = make([]types.DebugSpan, len(trace.Spans))
		for i, span := range trace.Spans {
			span.ID = validUTF8(span.ID)
			span.ParentID = validUTF8(span.ParentID)
			span.GenerationID = validUTF8(span.GenerationID)
			span.Name = validUTF8(span.Name)
			span.Type = validUTF8(span.Type)
			span.Status = validUTF8(span.Status)
			span.Level = validUTF8(span.Level)
			span.CallID = validUTF8(span.CallID)
			span.ToolName = validUTF8(span.ToolName)
			span.Input = validUTF8(span.Input)
			span.Output = validUTF8(span.Output)
			span.Error = validUTF8(span.Error)
			span.StartedAt = validUTF8(span.StartedAt)
			span.CompletedAt = validUTF8(span.CompletedAt)
			normalized.Spans[i] = span
		}
	}

	if trace.Events != nil {
		normalized.Events = make([]types.DebugEvent, len(trace.Events))
		for i, event := range trace.Events {
			event.ID = validUTF8(event.ID)
			event.Name = validUTF8(event.Name)
			event.Type = validUTF8(event.Type)
			event.Level = validUTF8(event.Level)
			event.Message = validUTF8(event.Message)
			event.Reason = validUTF8(event.Reason)
			event.SpanID = validUTF8(event.SpanID)
			event.GenerationID = validUTF8(event.GenerationID)
			event.Timestamp = validUTF8(event.Timestamp)
			normalized.Events[i] = event
		}
	}

	return &normalized
}
