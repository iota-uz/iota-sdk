package observability

import "context"

// contextKey is the type for context keys used by this package.
type contextKey string

const (
	// providerKey is the context key for the observability provider.
	providerKey contextKey = "bichat.observability.provider"

	// traceIDKey is the context key for the current trace ID.
	traceIDKey contextKey = "bichat.observability.trace_id"

	// spanIDKey is the context key for the current span ID.
	spanIDKey contextKey = "bichat.observability.span_id"
)

// WithProvider adds an observability provider to the context.
func WithProvider(ctx context.Context, provider Provider) context.Context {
	return context.WithValue(ctx, providerKey, provider)
}

// GetProvider retrieves the observability provider from the context.
// If no provider is found, returns a NoOpProvider.
func GetProvider(ctx context.Context) Provider {
	if provider, ok := ctx.Value(providerKey).(Provider); ok {
		return provider
	}
	return NewNoOpProvider()
}

// WithTraceID adds a trace ID to the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// GetTraceID retrieves the trace ID from the context.
func GetTraceID(ctx context.Context) (string, bool) {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID, true
	}
	return "", false
}

// WithSpanID adds a span ID to the context.
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, spanIDKey, spanID)
}

// GetSpanID retrieves the span ID from the context.
func GetSpanID(ctx context.Context) (string, bool) {
	if spanID, ok := ctx.Value(spanIDKey).(string); ok {
		return spanID, true
	}
	return "", false
}
