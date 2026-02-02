package observability

import (
	"context"
	"testing"
)

func TestContextPropagation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Test Provider propagation
	provider := NewNoOpProvider()
	ctx = WithProvider(ctx, provider)

	retrievedProvider := GetProvider(ctx)
	if retrievedProvider == nil {
		t.Error("GetProvider returned nil")
	}

	// Test Provider fallback (no provider in context)
	emptyCtx := context.Background()
	fallbackProvider := GetProvider(emptyCtx)
	if fallbackProvider == nil {
		t.Error("GetProvider should return NoOpProvider as fallback")
	}

	// Test TraceID propagation
	traceID := "trace-123"
	ctx = WithTraceID(ctx, traceID)

	retrievedTraceID, ok := GetTraceID(ctx)
	if !ok || retrievedTraceID != traceID {
		t.Errorf("GetTraceID failed: got %v, %v", retrievedTraceID, ok)
	}

	// Test TraceID fallback (no trace ID in context)
	emptyCtx = context.Background()
	_, ok = GetTraceID(emptyCtx)
	if ok {
		t.Error("GetTraceID should return false when no trace ID in context")
	}

	// Test SpanID propagation
	spanID := "span-456"
	ctx = WithSpanID(ctx, spanID)

	retrievedSpanID, ok := GetSpanID(ctx)
	if !ok || retrievedSpanID != spanID {
		t.Errorf("GetSpanID failed: got %v, %v", retrievedSpanID, ok)
	}

	// Test SpanID fallback (no span ID in context)
	emptyCtx = context.Background()
	_, ok = GetSpanID(emptyCtx)
	if ok {
		t.Error("GetSpanID should return false when no span ID in context")
	}
}
