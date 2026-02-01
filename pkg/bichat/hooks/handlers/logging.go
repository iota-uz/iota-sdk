package handlers

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	"github.com/sirupsen/logrus"
)

// LoggingHandler logs events to a logrus logger.
// It formats events nicely for human-readable logs with configurable log level.
type LoggingHandler struct {
	logger *logrus.Logger
	level  logrus.Level
}

// NewLoggingHandler creates a new LoggingHandler that logs events at the specified level.
func NewLoggingHandler(logger *logrus.Logger, level logrus.Level) *LoggingHandler {
	return &LoggingHandler{
		logger: logger,
		level:  level,
	}
}

// Handle implements EventHandler.
func (h *LoggingHandler) Handle(ctx context.Context, event hooks.Event) error {
	fields := logrus.Fields{
		"event_type": event.Type(),
		"session_id": event.SessionID().String(),
		"tenant_id":  event.TenantID().String(),
		"timestamp":  event.Timestamp().Format("2006-01-02 15:04:05"),
	}

	msg := h.formatMessage(event, fields)

	// Log at configured level
	entry := h.logger.WithFields(fields)
	switch h.level {
	case logrus.PanicLevel:
		entry.Panic(msg)
	case logrus.FatalLevel:
		entry.Fatal(msg)
	case logrus.ErrorLevel:
		entry.Error(msg)
	case logrus.WarnLevel:
		entry.Warn(msg)
	case logrus.InfoLevel:
		entry.Info(msg)
	case logrus.DebugLevel:
		entry.Debug(msg)
	case logrus.TraceLevel:
		entry.Trace(msg)
	default:
		entry.Info(msg)
	}

	return nil
}

// formatMessage creates a human-readable message for the event.
func (h *LoggingHandler) formatMessage(event hooks.Event, fields logrus.Fields) string {
	switch e := event.(type) {
	case *events.LLMRequestEvent:
		fields["model"] = e.Model
		fields["provider"] = e.Provider
		fields["messages"] = e.Messages
		fields["tools"] = e.Tools
		fields["estimated_tokens"] = e.EstimatedTokens
		return fmt.Sprintf("LLM request to %s/%s with %d messages, %d tools (~%d tokens)",
			e.Provider, e.Model, e.Messages, e.Tools, e.EstimatedTokens)

	case *events.LLMResponseEvent:
		fields["model"] = e.Model
		fields["provider"] = e.Provider
		fields["prompt_tokens"] = e.PromptTokens
		fields["completion_tokens"] = e.CompletionTokens
		fields["total_tokens"] = e.TotalTokens
		fields["latency_ms"] = e.LatencyMs
		fields["finish_reason"] = e.FinishReason
		fields["tool_calls"] = e.ToolCalls
		return fmt.Sprintf("LLM response from %s/%s: %d tokens (%d prompt + %d completion), %dms, %s",
			e.Provider, e.Model, e.TotalTokens, e.PromptTokens, e.CompletionTokens, e.LatencyMs, e.FinishReason)

	case *events.LLMStreamEvent:
		fields["model"] = e.Model
		fields["provider"] = e.Provider
		fields["index"] = e.Index
		fields["is_final"] = e.IsFinal
		fields["chunk_len"] = len(e.ChunkText)
		finalStr := ""
		if e.IsFinal {
			finalStr = " (final)"
		}
		return fmt.Sprintf("LLM stream chunk #%d from %s/%s: %d chars%s",
			e.Index, e.Provider, e.Model, len(e.ChunkText), finalStr)

	case *events.ToolStartEvent:
		fields["tool_name"] = e.ToolName
		fields["call_id"] = e.CallID
		return fmt.Sprintf("Tool execution started: %s (call_id=%s)", e.ToolName, e.CallID)

	case *events.ToolCompleteEvent:
		fields["tool_name"] = e.ToolName
		fields["call_id"] = e.CallID
		fields["duration_ms"] = e.DurationMs
		fields["result_len"] = len(e.Result)
		return fmt.Sprintf("Tool execution completed: %s (%dms, result=%d chars)",
			e.ToolName, e.DurationMs, len(e.Result))

	case *events.ToolErrorEvent:
		fields["tool_name"] = e.ToolName
		fields["call_id"] = e.CallID
		fields["error"] = e.Error
		fields["duration_ms"] = e.DurationMs
		return fmt.Sprintf("Tool execution failed: %s (%dms, error=%s)",
			e.ToolName, e.DurationMs, e.Error)

	case *events.ContextCompileEvent:
		fields["provider"] = e.Provider
		fields["total_tokens"] = e.TotalTokens
		fields["block_count"] = e.BlockCount
		fields["compacted"] = e.Compacted
		fields["truncated"] = e.Truncated
		fields["excluded_blocks"] = e.ExcludedBlocks
		flags := ""
		if e.Compacted {
			flags += " compacted"
		}
		if e.Truncated {
			flags += " truncated"
		}
		return fmt.Sprintf("Context compiled for %s: %d tokens, %d blocks%s",
			e.Provider, e.TotalTokens, e.BlockCount, flags)

	case *events.ContextCompactEvent:
		fields["original_messages"] = e.OriginalMessages
		fields["compacted_to"] = e.CompactedTo
		fields["tokens_saved"] = e.TokensSaved
		return fmt.Sprintf("Context compacted: %d → %d messages (saved %d tokens)",
			e.OriginalMessages, e.CompactedTo, e.TokensSaved)

	case *events.ContextOverflowEvent:
		fields["requested_tokens"] = e.RequestedTokens
		fields["available_tokens"] = e.AvailableTokens
		fields["overflow_strategy"] = e.OverflowStrategy
		fields["resolved"] = e.Resolved
		resolvedStr := "unresolved"
		if e.Resolved {
			resolvedStr = "resolved"
		}
		return fmt.Sprintf("Context overflow: %d tokens requested but only %d available (strategy=%s, %s)",
			e.RequestedTokens, e.AvailableTokens, e.OverflowStrategy, resolvedStr)

	case *events.SessionCreateEvent:
		fields["user_id"] = e.UserID
		fields["title"] = e.Title
		return fmt.Sprintf("Session created by user %d: %s", e.UserID, e.Title)

	case *events.MessageSaveEvent:
		fields["message_id"] = e.MessageID.String()
		fields["role"] = e.Role
		fields["content_len"] = e.ContentLen
		fields["tool_calls"] = e.ToolCalls
		return fmt.Sprintf("Message saved: %s message (%d chars, %d tool calls)",
			e.Role, e.ContentLen, e.ToolCalls)

	case *events.InterruptEvent:
		fields["interrupt_type"] = e.InterruptType
		fields["agent_name"] = e.AgentName
		fields["checkpoint_id"] = e.CheckpointID
		return fmt.Sprintf("Agent interrupted: %s (%s, checkpoint=%s)",
			e.AgentName, e.InterruptType, e.CheckpointID)

	default:
		return fmt.Sprintf("Event: %s", event.Type())
	}
}
