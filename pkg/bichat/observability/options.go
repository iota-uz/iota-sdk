package observability

import (
	"time"

	"github.com/google/uuid"
)

// GenerationOption configures a GenerationObservation.
type GenerationOption func(*GenerationObservation)

// WithGenerationID sets the generation ID.
func WithGenerationID(id string) GenerationOption {
	return func(g *GenerationObservation) {
		g.ID = id
	}
}

// WithGenerationTraceID sets the trace ID.
func WithGenerationTraceID(traceID string) GenerationOption {
	return func(g *GenerationObservation) {
		g.TraceID = traceID
	}
}

// WithGenerationTenantID sets the tenant ID.
func WithGenerationTenantID(tenantID uuid.UUID) GenerationOption {
	return func(g *GenerationObservation) {
		g.TenantID = tenantID
	}
}

// WithGenerationSessionID sets the session ID.
func WithGenerationSessionID(sessionID uuid.UUID) GenerationOption {
	return func(g *GenerationObservation) {
		g.SessionID = sessionID
	}
}

// WithGenerationTimestamp sets the timestamp.
func WithGenerationTimestamp(timestamp time.Time) GenerationOption {
	return func(g *GenerationObservation) {
		g.Timestamp = timestamp
	}
}

// WithModel sets the model name.
func WithModel(model string) GenerationOption {
	return func(g *GenerationObservation) {
		g.Model = model
	}
}

// WithGenerationProvider sets the provider name.
func WithGenerationProvider(provider string) GenerationOption {
	return func(g *GenerationObservation) {
		g.Provider = provider
	}
}

// WithPromptMessages sets the number of prompt messages.
func WithPromptMessages(count int) GenerationOption {
	return func(g *GenerationObservation) {
		g.PromptMessages = count
	}
}

// WithPromptTokens sets the prompt token count.
func WithPromptTokens(tokens int) GenerationOption {
	return func(g *GenerationObservation) {
		g.PromptTokens = tokens
	}
}

// WithTools sets the number of tools.
func WithTools(count int) GenerationOption {
	return func(g *GenerationObservation) {
		g.Tools = count
	}
}

// WithPromptContent sets the prompt content.
func WithPromptContent(content string) GenerationOption {
	return func(g *GenerationObservation) {
		g.PromptContent = content
	}
}

// WithCompletionTokens sets the completion token count.
func WithCompletionTokens(tokens int) GenerationOption {
	return func(g *GenerationObservation) {
		g.CompletionTokens = tokens
	}
}

// WithTotalTokens sets the total token count.
func WithTotalTokens(tokens int) GenerationOption {
	return func(g *GenerationObservation) {
		g.TotalTokens = tokens
	}
}

// WithLatencyMs sets the latency in milliseconds.
func WithLatencyMs(latency int64) GenerationOption {
	return func(g *GenerationObservation) {
		g.LatencyMs = latency
	}
}

// WithFinishReason sets the finish reason.
func WithFinishReason(reason string) GenerationOption {
	return func(g *GenerationObservation) {
		g.FinishReason = reason
	}
}

// WithToolCalls sets the number of tool calls.
func WithToolCalls(count int) GenerationOption {
	return func(g *GenerationObservation) {
		g.ToolCalls = count
	}
}

// WithCompletionText sets the completion text.
func WithCompletionText(text string) GenerationOption {
	return func(g *GenerationObservation) {
		g.CompletionText = text
	}
}

// WithGenerationDuration sets the duration.
func WithGenerationDuration(duration time.Duration) GenerationOption {
	return func(g *GenerationObservation) {
		g.Duration = duration
	}
}

// WithGenerationAttributes sets custom attributes.
func WithGenerationAttributes(attrs map[string]interface{}) GenerationOption {
	return func(g *GenerationObservation) {
		if g.Attributes == nil {
			g.Attributes = make(map[string]interface{})
		}
		for k, v := range attrs {
			g.Attributes[k] = v
		}
	}
}

// NewGenerationObservation creates a new GenerationObservation with options.
func NewGenerationObservation(opts ...GenerationOption) GenerationObservation {
	g := GenerationObservation{
		Timestamp:  time.Now(),
		Attributes: make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(&g)
	}
	return g
}

// SpanOption configures a SpanObservation.
type SpanOption func(*SpanObservation)

// WithObservationSpanID sets the span ID.
func WithObservationSpanID(id string) SpanOption {
	return func(s *SpanObservation) {
		s.ID = id
	}
}

// WithSpanTraceID sets the trace ID.
func WithSpanTraceID(traceID string) SpanOption {
	return func(s *SpanObservation) {
		s.TraceID = traceID
	}
}

// WithSpanParentID sets the parent span ID.
func WithSpanParentID(parentID string) SpanOption {
	return func(s *SpanObservation) {
		s.ParentID = parentID
	}
}

// WithSpanTenantID sets the tenant ID.
func WithSpanTenantID(tenantID uuid.UUID) SpanOption {
	return func(s *SpanObservation) {
		s.TenantID = tenantID
	}
}

// WithSpanSessionID sets the session ID.
func WithSpanSessionID(sessionID uuid.UUID) SpanOption {
	return func(s *SpanObservation) {
		s.SessionID = sessionID
	}
}

// WithSpanTimestamp sets the timestamp.
func WithSpanTimestamp(timestamp time.Time) SpanOption {
	return func(s *SpanObservation) {
		s.Timestamp = timestamp
	}
}

// WithSpanName sets the span name.
func WithSpanName(name string) SpanOption {
	return func(s *SpanObservation) {
		s.Name = name
	}
}

// WithSpanType sets the span type.
func WithSpanType(typ string) SpanOption {
	return func(s *SpanObservation) {
		s.Type = typ
	}
}

// WithSpanInput sets the input.
func WithSpanInput(input string) SpanOption {
	return func(s *SpanObservation) {
		s.Input = input
	}
}

// WithSpanOutput sets the output.
func WithSpanOutput(output string) SpanOption {
	return func(s *SpanObservation) {
		s.Output = output
	}
}

// WithSpanDuration sets the duration.
func WithSpanDuration(duration time.Duration) SpanOption {
	return func(s *SpanObservation) {
		s.Duration = duration
	}
}

// WithSpanStatus sets the status.
func WithSpanStatus(status string) SpanOption {
	return func(s *SpanObservation) {
		s.Status = status
	}
}

// WithSpanToolName sets the tool name.
func WithSpanToolName(toolName string) SpanOption {
	return func(s *SpanObservation) {
		s.ToolName = toolName
	}
}

// WithSpanCallID sets the call ID.
func WithSpanCallID(callID string) SpanOption {
	return func(s *SpanObservation) {
		s.CallID = callID
	}
}

// WithSpanAttributes sets custom attributes.
func WithSpanAttributes(attrs map[string]interface{}) SpanOption {
	return func(s *SpanObservation) {
		if s.Attributes == nil {
			s.Attributes = make(map[string]interface{})
		}
		for k, v := range attrs {
			s.Attributes[k] = v
		}
	}
}

// NewSpanObservation creates a new SpanObservation with options.
func NewSpanObservation(opts ...SpanOption) SpanObservation {
	s := SpanObservation{
		Timestamp:  time.Now(),
		Attributes: make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(&s)
	}
	return s
}

// EventOption configures an EventObservation.
type EventOption func(*EventObservation)

// WithEventID sets the event ID.
func WithEventID(id string) EventOption {
	return func(e *EventObservation) {
		e.ID = id
	}
}

// WithEventTraceID sets the trace ID.
func WithEventTraceID(traceID string) EventOption {
	return func(e *EventObservation) {
		e.TraceID = traceID
	}
}

// WithEventTenantID sets the tenant ID.
func WithEventTenantID(tenantID uuid.UUID) EventOption {
	return func(e *EventObservation) {
		e.TenantID = tenantID
	}
}

// WithEventSessionID sets the session ID.
func WithEventSessionID(sessionID uuid.UUID) EventOption {
	return func(e *EventObservation) {
		e.SessionID = sessionID
	}
}

// WithEventTimestamp sets the timestamp.
func WithEventTimestamp(timestamp time.Time) EventOption {
	return func(e *EventObservation) {
		e.Timestamp = timestamp
	}
}

// WithEventName sets the event name.
func WithEventName(name string) EventOption {
	return func(e *EventObservation) {
		e.Name = name
	}
}

// WithEventType sets the event type.
func WithEventType(typ string) EventOption {
	return func(e *EventObservation) {
		e.Type = typ
	}
}

// WithEventMessage sets the message.
func WithEventMessage(message string) EventOption {
	return func(e *EventObservation) {
		e.Message = message
	}
}

// WithEventLevel sets the level.
func WithEventLevel(level string) EventOption {
	return func(e *EventObservation) {
		e.Level = level
	}
}

// WithEventAttributes sets custom attributes.
func WithEventAttributes(attrs map[string]interface{}) EventOption {
	return func(e *EventObservation) {
		if e.Attributes == nil {
			e.Attributes = make(map[string]interface{})
		}
		for k, v := range attrs {
			e.Attributes[k] = v
		}
	}
}

// NewEventObservation creates a new EventObservation with options.
func NewEventObservation(opts ...EventOption) EventObservation {
	e := EventObservation{
		Timestamp:  time.Now(),
		Attributes: make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// TraceOption configures a TraceObservation.
type TraceOption func(*TraceObservation)

// WithObservationTraceID sets the trace ID.
func WithObservationTraceID(id string) TraceOption {
	return func(t *TraceObservation) {
		t.ID = id
	}
}

// WithTraceTenantID sets the tenant ID.
func WithTraceTenantID(tenantID uuid.UUID) TraceOption {
	return func(t *TraceObservation) {
		t.TenantID = tenantID
	}
}

// WithTraceSessionID sets the session ID.
func WithTraceSessionID(sessionID uuid.UUID) TraceOption {
	return func(t *TraceObservation) {
		t.SessionID = sessionID
	}
}

// WithTraceTimestamp sets the timestamp.
func WithTraceTimestamp(timestamp time.Time) TraceOption {
	return func(t *TraceObservation) {
		t.Timestamp = timestamp
	}
}

// WithTraceName sets the trace name.
func WithTraceName(name string) TraceOption {
	return func(t *TraceObservation) {
		t.Name = name
	}
}

// WithTraceDuration sets the duration.
func WithTraceDuration(duration time.Duration) TraceOption {
	return func(t *TraceObservation) {
		t.Duration = duration
	}
}

// WithTraceStatus sets the status.
func WithTraceStatus(status string) TraceOption {
	return func(t *TraceObservation) {
		t.Status = status
	}
}

// WithTraceUserID sets the user ID.
func WithTraceUserID(userID uuid.UUID) TraceOption {
	return func(t *TraceObservation) {
		t.UserID = userID
	}
}

// WithTraceTotalCost sets the total cost.
func WithTraceTotalCost(cost float64) TraceOption {
	return func(t *TraceObservation) {
		t.TotalCost = cost
	}
}

// WithTraceTotalTokens sets the total tokens.
func WithTraceTotalTokens(tokens int) TraceOption {
	return func(t *TraceObservation) {
		t.TotalTokens = tokens
	}
}

// WithTraceAttributes sets custom attributes.
func WithTraceAttributes(attrs map[string]interface{}) TraceOption {
	return func(t *TraceObservation) {
		if t.Attributes == nil {
			t.Attributes = make(map[string]interface{})
		}
		for k, v := range attrs {
			t.Attributes[k] = v
		}
	}
}

// NewTraceObservation creates a new TraceObservation with options.
func NewTraceObservation(opts ...TraceOption) TraceObservation {
	t := TraceObservation{
		Timestamp:  time.Now(),
		Attributes: make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(&t)
	}
	return t
}
