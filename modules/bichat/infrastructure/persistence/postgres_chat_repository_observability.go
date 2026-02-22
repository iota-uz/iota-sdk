package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

const (
	upsertTraceProjectionQuery = `
		INSERT INTO bichat.traces (
			tenant_id, session_id, message_id, external_trace_id, trace_url,
			status, generation_ms, thinking, observation_reason, metadata,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12
		)
		ON CONFLICT (tenant_id, external_trace_id) DO UPDATE
		SET
			message_id = EXCLUDED.message_id,
			trace_url = COALESCE(EXCLUDED.trace_url, bichat.traces.trace_url),
			status = EXCLUDED.status,
			generation_ms = EXCLUDED.generation_ms,
			thinking = COALESCE(NULLIF(EXCLUDED.thinking, ''), bichat.traces.thinking),
			observation_reason = COALESCE(NULLIF(EXCLUDED.observation_reason, ''), bichat.traces.observation_reason),
			metadata = bichat.traces.metadata || EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	upsertGenerationProjectionQuery = `
		INSERT INTO bichat.generations (
			tenant_id, trace_ref_id, external_generation_id, request_id, model, provider, finish_reason,
			prompt_tokens, completion_tokens, total_tokens, cached_tokens, cost, latency_ms,
			input_text, output_text, thinking, observation_reason, metadata,
			started_at, completed_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20, $21
		)
		ON CONFLICT (tenant_id, trace_ref_id, external_generation_id) DO UPDATE
		SET
			request_id = COALESCE(NULLIF(EXCLUDED.request_id, ''), bichat.generations.request_id),
			model = COALESCE(NULLIF(EXCLUDED.model, ''), bichat.generations.model),
			provider = COALESCE(NULLIF(EXCLUDED.provider, ''), bichat.generations.provider),
			finish_reason = COALESCE(NULLIF(EXCLUDED.finish_reason, ''), bichat.generations.finish_reason),
			prompt_tokens = EXCLUDED.prompt_tokens,
			completion_tokens = EXCLUDED.completion_tokens,
			total_tokens = EXCLUDED.total_tokens,
			cached_tokens = EXCLUDED.cached_tokens,
			cost = EXCLUDED.cost,
			latency_ms = EXCLUDED.latency_ms,
			input_text = COALESCE(NULLIF(EXCLUDED.input_text, ''), bichat.generations.input_text),
			output_text = COALESCE(NULLIF(EXCLUDED.output_text, ''), bichat.generations.output_text),
			thinking = COALESCE(NULLIF(EXCLUDED.thinking, ''), bichat.generations.thinking),
			observation_reason = COALESCE(NULLIF(EXCLUDED.observation_reason, ''), bichat.generations.observation_reason),
			metadata = bichat.generations.metadata || EXCLUDED.metadata,
			started_at = COALESCE(EXCLUDED.started_at, bichat.generations.started_at),
			completed_at = COALESCE(EXCLUDED.completed_at, bichat.generations.completed_at)
	`

	upsertSpanProjectionQuery = `
		INSERT INTO bichat.spans (
			tenant_id, trace_ref_id, external_span_id, parent_external_span_id, generation_external_id,
			name, type, status, level, call_id, tool_name,
			input_text, output_text, error_text, duration_ms, attributes,
			started_at, completed_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19
		)
		ON CONFLICT (tenant_id, trace_ref_id, external_span_id) DO UPDATE
		SET
			parent_external_span_id = COALESCE(NULLIF(EXCLUDED.parent_external_span_id, ''), bichat.spans.parent_external_span_id),
			generation_external_id = COALESCE(NULLIF(EXCLUDED.generation_external_id, ''), bichat.spans.generation_external_id),
			name = EXCLUDED.name,
			type = EXCLUDED.type,
			status = EXCLUDED.status,
			level = COALESCE(NULLIF(EXCLUDED.level, ''), bichat.spans.level),
			call_id = COALESCE(NULLIF(EXCLUDED.call_id, ''), bichat.spans.call_id),
			tool_name = COALESCE(NULLIF(EXCLUDED.tool_name, ''), bichat.spans.tool_name),
			input_text = COALESCE(NULLIF(EXCLUDED.input_text, ''), bichat.spans.input_text),
			output_text = COALESCE(NULLIF(EXCLUDED.output_text, ''), bichat.spans.output_text),
			error_text = COALESCE(NULLIF(EXCLUDED.error_text, ''), bichat.spans.error_text),
			duration_ms = EXCLUDED.duration_ms,
			attributes = bichat.spans.attributes || EXCLUDED.attributes,
			started_at = COALESCE(EXCLUDED.started_at, bichat.spans.started_at),
			completed_at = COALESCE(EXCLUDED.completed_at, bichat.spans.completed_at)
	`

	upsertEventProjectionQuery = `
		INSERT INTO bichat.events (
			tenant_id, trace_ref_id, external_event_id, name, type, level, message, reason,
			span_external_id, generation_external_id, attributes, timestamp, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13
		)
		ON CONFLICT (tenant_id, trace_ref_id, external_event_id) DO UPDATE
		SET
			name = EXCLUDED.name,
			type = EXCLUDED.type,
			level = COALESCE(NULLIF(EXCLUDED.level, ''), bichat.events.level),
			message = COALESCE(NULLIF(EXCLUDED.message, ''), bichat.events.message),
			reason = COALESCE(NULLIF(EXCLUDED.reason, ''), bichat.events.reason),
			span_external_id = COALESCE(NULLIF(EXCLUDED.span_external_id, ''), bichat.events.span_external_id),
			generation_external_id = COALESCE(NULLIF(EXCLUDED.generation_external_id, ''), bichat.events.generation_external_id),
			attributes = bichat.events.attributes || EXCLUDED.attributes,
			timestamp = COALESCE(EXCLUDED.timestamp, bichat.events.timestamp)
	`
)

func (r *PostgresChatRepository) persistDebugTraceProjection(
	ctx context.Context,
	tx repo.Tx,
	tenantID uuid.UUID,
	msg types.Message,
) error {
	trace := msg.DebugTrace()
	if trace == nil {
		return nil
	}

	externalTraceID := strings.TrimSpace(trace.TraceID)
	if externalTraceID == "" {
		return nil
	}

	createdAt := msg.CreatedAt()
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	sessionID := msg.SessionID()
	if sessionID == uuid.Nil {
		if parsed, err := uuid.Parse(strings.TrimSpace(trace.SessionID)); err == nil {
			sessionID = parsed
		}
	}
	if sessionID == uuid.Nil {
		return nil
	}

	traceMetadata, err := marshalJSONB(map[string]interface{}{
		"schema_version": strings.TrimSpace(trace.SchemaVersion),
		"started_at":     strings.TrimSpace(trace.StartedAt),
		"completed_at":   strings.TrimSpace(trace.CompletedAt),
		"tool_count":     len(trace.Tools),
		"span_count":     len(trace.Spans),
		"event_count":    len(trace.Events),
	})
	if err != nil {
		return err
	}

	status := "completed"
	if strings.TrimSpace(trace.ObservationReason) != "" {
		status = "interrupted"
	}

	var traceRefID uuid.UUID
	if err := tx.QueryRow(
		ctx,
		upsertTraceProjectionQuery,
		tenantID,
		sessionID,
		msg.ID(),
		externalTraceID,
		nullableString(trace.TraceURL),
		status,
		trace.GenerationMs,
		nullableString(trace.Thinking),
		nullableString(trace.ObservationReason),
		traceMetadata,
		createdAt,
		time.Now(),
	).Scan(&traceRefID); err != nil {
		return err
	}

	attempts := trace.Attempts
	if len(attempts) == 0 {
		fallback := types.DebugGeneration{
			ID:                externalTraceID + ":final",
			PromptTokens:      0,
			CompletionTokens:  0,
			TotalTokens:       0,
			CachedTokens:      0,
			Cost:              0,
			LatencyMs:         trace.GenerationMs,
			Output:            msg.Content(),
			Thinking:          trace.Thinking,
			ObservationReason: trace.ObservationReason,
			StartedAt:         trace.StartedAt,
			CompletedAt:       trace.CompletedAt,
			ToolCalls:         trace.Tools,
		}
		if trace.Usage != nil {
			fallback.PromptTokens = trace.Usage.PromptTokens
			fallback.CompletionTokens = trace.Usage.CompletionTokens
			fallback.TotalTokens = trace.Usage.TotalTokens
			fallback.CachedTokens = trace.Usage.CachedTokens
			fallback.Cost = trace.Usage.Cost
		}
		attempts = []types.DebugGeneration{fallback}
	}

	for idx, attempt := range attempts {
		generationID := strings.TrimSpace(attempt.ID)
		if generationID == "" {
			generationID = fmt.Sprintf("%s:gen:%d", externalTraceID, idx+1)
		}

		startedAt := parseRFC3339(attempt.StartedAt)
		completedAt := parseRFC3339(attempt.CompletedAt)
		attemptMetadata, err := marshalJSONB(map[string]interface{}{
			"tool_call_count": len(attempt.ToolCalls),
		})
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			ctx,
			upsertGenerationProjectionQuery,
			tenantID,
			traceRefID,
			generationID,
			nullableString(attempt.RequestID),
			nullableString(attempt.Model),
			nullableString(attempt.Provider),
			nullableString(attempt.FinishReason),
			attempt.PromptTokens,
			attempt.CompletionTokens,
			attempt.TotalTokens,
			attempt.CachedTokens,
			attempt.Cost,
			attempt.LatencyMs,
			nullableString(attempt.Input),
			nullableString(attempt.Output),
			nullableString(attempt.Thinking),
			nullableString(attempt.ObservationReason),
			attemptMetadata,
			startedAt,
			completedAt,
			createdAt,
		); err != nil {
			return err
		}
	}

	spans := trace.Spans
	if len(spans) == 0 && len(trace.Tools) > 0 {
		spans = make([]types.DebugSpan, 0, len(trace.Tools))
		for idx, tool := range trace.Tools {
			spanID := strings.TrimSpace(tool.CallID)
			if spanID == "" {
				spanID = fmt.Sprintf("%s:tool:%d", externalTraceID, idx+1)
			}
			status := "success"
			if strings.TrimSpace(tool.Error) != "" {
				status = "error"
			}
			spans = append(spans, types.DebugSpan{
				ID:         spanID,
				Name:       "tool.execute",
				Type:       "tool",
				Status:     status,
				CallID:     tool.CallID,
				ToolName:   tool.Name,
				Input:      tool.Arguments,
				Output:     tool.Result,
				Error:      tool.Error,
				DurationMs: tool.DurationMs,
			})
		}
	}

	for idx, span := range spans {
		spanID := strings.TrimSpace(span.ID)
		if spanID == "" {
			spanID = fmt.Sprintf("%s:span:%d", externalTraceID, idx+1)
		}
		attrs, err := marshalJSONB(span.Attributes)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			ctx,
			upsertSpanProjectionQuery,
			tenantID,
			traceRefID,
			spanID,
			nullableString(span.ParentID),
			nullableString(span.GenerationID),
			nonEmptyOrDefault(span.Name, "span"),
			nonEmptyOrDefault(span.Type, "span"),
			nonEmptyOrDefault(span.Status, "success"),
			nullableString(span.Level),
			nullableString(span.CallID),
			nullableString(span.ToolName),
			nullableString(span.Input),
			nullableString(span.Output),
			nullableString(span.Error),
			span.DurationMs,
			attrs,
			parseRFC3339(span.StartedAt),
			parseRFC3339(span.CompletedAt),
			createdAt,
		); err != nil {
			return err
		}
	}

	for idx, event := range trace.Events {
		eventID := strings.TrimSpace(event.ID)
		if eventID == "" {
			eventID = fmt.Sprintf("%s:event:%d", externalTraceID, idx+1)
		}
		attrs, err := marshalJSONB(event.Attributes)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			ctx,
			upsertEventProjectionQuery,
			tenantID,
			traceRefID,
			eventID,
			nonEmptyOrDefault(event.Name, "event"),
			nonEmptyOrDefault(event.Type, "event"),
			nullableString(event.Level),
			nullableString(event.Message),
			nullableString(event.Reason),
			nullableString(event.SpanID),
			nullableString(event.GenerationID),
			attrs,
			parseRFC3339(event.Timestamp),
			createdAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func marshalJSONB(value interface{}) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || string(data) == "null" {
		return []byte("{}"), nil
	}
	return data, nil
}

func nullableString(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func nonEmptyOrDefault(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func parseRFC3339(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil
	}
	return parsed
}
