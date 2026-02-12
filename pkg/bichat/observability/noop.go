package observability

import "context"

// NoOpProvider is a zero-overhead observability provider that does nothing.
// Use this as the default when observability is disabled.
type NoOpProvider struct{}

// NewNoOpProvider creates a no-op observability provider.
func NewNoOpProvider() Provider {
	return &NoOpProvider{}
}

// RecordGeneration does nothing.
func (p *NoOpProvider) RecordGeneration(ctx context.Context, obs GenerationObservation) error {
	return nil
}

// RecordSpan does nothing.
func (p *NoOpProvider) RecordSpan(ctx context.Context, obs SpanObservation) error {
	return nil
}

// RecordEvent does nothing.
func (p *NoOpProvider) RecordEvent(ctx context.Context, obs EventObservation) error {
	return nil
}

// RecordTrace does nothing.
func (p *NoOpProvider) RecordTrace(ctx context.Context, obs TraceObservation) error {
	return nil
}
