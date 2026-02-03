package observability

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNoOpProvider(t *testing.T) {
	t.Parallel()

	provider := NewNoOpProvider()
	ctx := context.Background()

	// Test RecordGeneration (should not error)
	genObs := GenerationObservation{
		ID:               uuid.New().String(),
		TraceID:          uuid.New().String(),
		TenantID:         uuid.New(),
		SessionID:        uuid.New(),
		Timestamp:        time.Now(),
		Model:            "gpt-4",
		Provider:         "openai",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Attributes:       make(map[string]interface{}),
	}

	if err := provider.RecordGeneration(ctx, genObs); err != nil {
		t.Errorf("NoOpProvider.RecordGeneration() returned error: %v", err)
	}

	// Test RecordSpan (should not error)
	spanObs := SpanObservation{
		ID:         uuid.New().String(),
		TraceID:    uuid.New().String(),
		TenantID:   uuid.New(),
		SessionID:  uuid.New(),
		Timestamp:  time.Now(),
		Name:       "test.span",
		Type:       "custom",
		Attributes: make(map[string]interface{}),
	}

	if err := provider.RecordSpan(ctx, spanObs); err != nil {
		t.Errorf("NoOpProvider.RecordSpan() returned error: %v", err)
	}

	// Test RecordEvent (should not error)
	eventObs := EventObservation{
		ID:         uuid.New().String(),
		TraceID:    uuid.New().String(),
		TenantID:   uuid.New(),
		SessionID:  uuid.New(),
		Timestamp:  time.Now(),
		Name:       "test.event",
		Type:       "custom",
		Attributes: make(map[string]interface{}),
	}

	if err := provider.RecordEvent(ctx, eventObs); err != nil {
		t.Errorf("NoOpProvider.RecordEvent() returned error: %v", err)
	}

	// Test RecordTrace (should not error)
	traceObs := TraceObservation{
		ID:         uuid.New().String(),
		TenantID:   uuid.New(),
		SessionID:  uuid.New(),
		Timestamp:  time.Now(),
		Name:       "test.trace",
		Attributes: make(map[string]interface{}),
	}

	if err := provider.RecordTrace(ctx, traceObs); err != nil {
		t.Errorf("NoOpProvider.RecordTrace() returned error: %v", err)
	}
}
