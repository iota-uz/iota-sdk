package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// OTelTraceSpanIDs extracts OpenTelemetry trace and span IDs from context.
// Returns ok=false if no valid span context is present.
func OTelTraceSpanIDs(ctx context.Context) (string, string, bool) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() || !sc.HasTraceID() || !sc.HasSpanID() {
		return "", "", false
	}
	return sc.TraceID().String(), sc.SpanID().String(), true
}
