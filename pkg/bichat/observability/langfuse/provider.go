package langfuse

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/henomis/langfuse-go/model"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/sirupsen/logrus"
)

// LangfuseProvider implements the observability.Provider interface using Langfuse.
// It sends observations to Langfuse for tracing, debugging, and cost analysis.
//
// All errors are logged but not propagated - observability failures should not
// break the application.
type LangfuseProvider struct {
	client LangfuseClient
	config Config
	state  *state
	rng    *rand.Rand
	log    *logrus.Logger
}

// NewLangfuseProvider creates a new Langfuse observability provider with dependency injection.
//
// The client parameter must be a configured LangfuseClient instance (e.g., *langfuse.Langfuse).
// For production use, create the client with proper credentials:
//
//	client := langfuse.New(ctx).WithFlushInterval(flushInterval)
//	provider, err := NewLangfuseProvider(client, config)
//
// Environment variables must be set BEFORE creating the client:
//   - LANGFUSE_HOST (Langfuse API endpoint)
//   - LANGFUSE_PUBLIC_KEY (required)
//   - LANGFUSE_SECRET_KEY (required)
//
// For testing, inject a mock client implementing LangfuseClient interface.
//
// Returns an error if client is nil or configuration is invalid.
func NewLangfuseProvider(client LangfuseClient, config Config) (*LangfuseProvider, error) {
	// Validate client
	if client == nil {
		return nil, fmt.Errorf("langfuse client cannot be nil")
	}

	// Validate and apply defaults
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid langfuse config: %w", err)
	}

	// Create logger
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	return &LangfuseProvider{
		client: client,
		config: config,
		state:  newState(),
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
		log:    log,
	}, nil
}

// RecordGeneration records a completed LLM generation to Langfuse.
// Non-blocking - logs errors instead of returning them.
func (p *LangfuseProvider) RecordGeneration(ctx context.Context, obs observability.GenerationObservation) error {
	// Check if disabled
	if !p.config.Enabled {
		return nil
	}

	// Apply sampling
	if !p.shouldSample() {
		return nil
	}

	// Ensure trace exists first
	if err := p.ensureTrace(ctx, obs.SessionID.String(), obs.TenantID.String()); err != nil {
		p.log.Errorf("langfuse: failed to ensure trace: %v", err)
		return nil // Non-blocking
	}

	// Extract metadata
	metadata := mapGenerationToLangfuse(obs)

	// Calculate cost
	cost := p.calculateCost(obs)

	// Build usage
	usage := model.Usage{
		Input:            obs.PromptTokens,
		Output:           obs.CompletionTokens,
		Total:            obs.TotalTokens,
		Unit:             model.ModelUsageUnitTokens,
		PromptTokens:     obs.PromptTokens,
		CompletionTokens: obs.CompletionTokens,
		TotalTokens:      obs.TotalTokens,
	}

	// Add cost if calculated
	if cost > 0 {
		usage.TotalCost = cost
	}

	// Extract cache tokens from metadata if present
	if obs.Attributes != nil {
		if cacheWrite, ok := obs.Attributes["cache_write_tokens"].(int); ok && cacheWrite > 0 {
			// Store in metadata since Usage struct doesn't have cache fields
			metadata["cache_write_tokens"] = cacheWrite
		}
		if cacheRead, ok := obs.Attributes["cache_read_tokens"].(int); ok && cacheRead > 0 {
			metadata["cache_read_tokens"] = cacheRead
		}
	}

	// Create generation
	generation := &model.Generation{
		ID:                  obs.ID,
		TraceID:             obs.SessionID.String(),
		Name:                obs.Model,
		StartTime:           &obs.Timestamp,
		Model:               obs.Model,
		Metadata:            metadata,
		Usage:               usage,
		Level:               model.ObservationLevelDefault,
		CompletionStartTime: timePtr(obs.Timestamp.Add(obs.Duration)),
	}

	// Create generation in Langfuse
	if _, err := p.client.Generation(generation, nil); err != nil {
		p.log.Errorf("langfuse: failed to create generation: %v", err)
		return nil // Non-blocking
	}

	// Store generation ID mapping
	p.state.setGenerationID(obs.ID, obs.ID)

	// End generation
	generation.EndTime = timePtr(obs.Timestamp.Add(obs.Duration))
	if _, err := p.client.GenerationEnd(generation); err != nil {
		p.log.Errorf("langfuse: failed to end generation: %v", err)
		return nil // Non-blocking
	}

	// Log success
	p.log.Debugf("langfuse: recorded generation %s (model=%s, tokens=%d, cost=%.6f)",
		obs.ID, obs.Model, obs.TotalTokens, cost)

	return nil
}

// RecordSpan records a completed operation span to Langfuse.
func (p *LangfuseProvider) RecordSpan(ctx context.Context, obs observability.SpanObservation) error {
	// Check if disabled
	if !p.config.Enabled {
		return nil
	}

	// Apply sampling
	if !p.shouldSample() {
		return nil
	}

	// Ensure trace exists first
	if err := p.ensureTrace(ctx, obs.SessionID.String(), obs.TenantID.String()); err != nil {
		p.log.Errorf("langfuse: failed to ensure trace: %v", err)
		return nil // Non-blocking
	}

	// Extract metadata
	metadata := mapSpanToLangfuse(obs)

	// Get parent observation ID if available
	var parentID *string
	if obs.ParentID != "" {
		if spanID := p.state.getSpanID(obs.ParentID); spanID != "" {
			parentID = &spanID
		}
	}

	// Create span
	span := &model.Span{
		ID:        obs.ID,
		TraceID:   obs.SessionID.String(),
		Name:      obs.Name,
		StartTime: &obs.Timestamp,
		Metadata:  metadata,
		Input:     obs.Input,
		Output:    obs.Output,
		Level:     model.ObservationLevelDefault,
	}

	// Create span in Langfuse
	if _, err := p.client.Span(span, parentID); err != nil {
		p.log.Errorf("langfuse: failed to create span: %v", err)
		return nil // Non-blocking
	}

	// Store span ID mapping
	p.state.setSpanID(obs.ID, obs.ID)

	// End span
	span.EndTime = timePtr(obs.Timestamp.Add(obs.Duration))
	if _, err := p.client.SpanEnd(span); err != nil {
		p.log.Errorf("langfuse: failed to end span: %v", err)
		return nil // Non-blocking
	}

	// Log success
	p.log.Debugf("langfuse: recorded span %s (name=%s, duration=%s)",
		obs.ID, obs.Name, obs.Duration)

	return nil
}

// RecordEvent records a point-in-time event to Langfuse.
func (p *LangfuseProvider) RecordEvent(ctx context.Context, obs observability.EventObservation) error {
	// Check if disabled
	if !p.config.Enabled {
		return nil
	}

	// Apply sampling
	if !p.shouldSample() {
		return nil
	}

	// Ensure trace exists first
	if err := p.ensureTrace(ctx, obs.SessionID.String(), obs.TenantID.String()); err != nil {
		p.log.Errorf("langfuse: failed to ensure trace: %v", err)
		return nil // Non-blocking
	}

	// Extract metadata
	metadata := mapEventToLangfuse(obs)

	// Create event
	event := &model.Event{
		ID:        obs.ID,
		TraceID:   obs.SessionID.String(),
		Name:      obs.Name,
		StartTime: &obs.Timestamp,
		Metadata:  metadata,
		Level:     mapLevelToLangfuseModel(obs.Level),
	}

	// Create event in Langfuse
	if _, err := p.client.Event(event, nil); err != nil {
		p.log.Errorf("langfuse: failed to create event: %v", err)
		return nil // Non-blocking
	}

	// Log success
	p.log.Debugf("langfuse: recorded event %s (name=%s, level=%s)",
		obs.ID, obs.Name, obs.Level)

	return nil
}

// RecordTrace records a complete trace to Langfuse.
func (p *LangfuseProvider) RecordTrace(ctx context.Context, obs observability.TraceObservation) error {
	// Check if disabled
	if !p.config.Enabled {
		return nil
	}

	// Apply sampling
	if !p.shouldSample() {
		return nil
	}

	// Extract metadata
	metadata := mapTraceToLangfuse(obs)

	// Add environment and version tags
	if p.config.Environment != "" {
		metadata["environment"] = p.config.Environment
	}
	if p.config.Version != "" {
		metadata["version"] = p.config.Version
	}

	// Create trace
	trace := &model.Trace{
		ID:        obs.SessionID.String(),
		Name:      obs.Name,
		Timestamp: &obs.Timestamp,
		UserID:    obs.UserID.String(),
		SessionID: obs.SessionID.String(),
		Metadata:  metadata,
		Tags:      p.config.Tags,
	}

	// Create trace in Langfuse
	if _, err := p.client.Trace(trace); err != nil {
		p.log.Errorf("langfuse: failed to create trace: %v", err)
		return nil // Non-blocking
	}

	// Store trace ID mapping
	p.state.setTraceID(obs.SessionID.String(), obs.SessionID.String())

	// Log success
	p.log.Debugf("langfuse: recorded trace %s (name=%s, tokens=%d, cost=%.6f)",
		obs.SessionID, obs.Name, obs.TotalTokens, obs.TotalCost)

	return nil
}

// Flush forces all pending observations to be sent to Langfuse.
// This should be called before application shutdown.
func (p *LangfuseProvider) Flush(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	p.log.Debug("langfuse: flushing pending observations")
	p.client.Flush(ctx)
	return nil
}

// Shutdown gracefully shuts down the provider, flushing all pending data.
func (p *LangfuseProvider) Shutdown(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	p.log.Info("langfuse: shutting down provider")

	// Flush pending observations
	if err := p.Flush(ctx); err != nil {
		p.log.Errorf("langfuse: error during flush: %v", err)
	}

	// Clear state
	p.state.clear()

	return nil
}

// ensureTrace creates a trace if it doesn't exist yet.
// This is necessary because Langfuse requires a trace before creating observations.
func (p *LangfuseProvider) ensureTrace(ctx context.Context, sessionID, tenantID string) error {
	// Check if trace already exists in state
	if traceID := p.state.getTraceID(sessionID); traceID != "" {
		return nil
	}

	// Create trace
	trace := &model.Trace{
		ID:        sessionID,
		Name:      "BiChat Session",
		SessionID: sessionID,
		Metadata: map[string]interface{}{
			"tenant_id": tenantID,
		},
	}

	if _, err := p.client.Trace(trace); err != nil {
		return fmt.Errorf("failed to create trace: %w", err)
	}

	// Store trace ID
	p.state.setTraceID(sessionID, sessionID)

	return nil
}

// shouldSample determines if this observation should be sent based on sample rate.
func (p *LangfuseProvider) shouldSample() bool {
	if p.config.SampleRate >= 1.0 {
		return true
	}
	if p.config.SampleRate <= 0.0 {
		return false
	}
	return p.rng.Float64() < p.config.SampleRate
}

// calculateCost calculates the cost of a generation using metadata pricing.
// Returns 0 if pricing is not available.
func (p *LangfuseProvider) calculateCost(obs observability.GenerationObservation) float64 {
	// Check if cost is already provided in attributes
	if obs.Attributes != nil {
		if cost, ok := obs.Attributes["cost"].(float64); ok {
			return cost
		}
	}

	// Check if pricing is available in metadata
	if obs.Attributes == nil {
		return 0
	}

	inputPer1M, hasInputPrice := obs.Attributes["input_price_per_1m"].(float64)
	outputPer1M, hasOutputPrice := obs.Attributes["output_price_per_1m"].(float64)

	if !hasInputPrice || !hasOutputPrice {
		return 0
	}

	// Calculate cost
	inputCost := (float64(obs.PromptTokens) / 1_000_000) * inputPer1M
	outputCost := (float64(obs.CompletionTokens) / 1_000_000) * outputPer1M

	return inputCost + outputCost
}

// mapLevelToLangfuseModel maps BiChat event levels to Langfuse model.ObservationLevel.
func mapLevelToLangfuseModel(level string) model.ObservationLevel {
	switch level {
	case "info":
		return model.ObservationLevelDefault
	case "warn", "warning":
		return model.ObservationLevelWarning
	case "error":
		return model.ObservationLevelError
	case "debug":
		return model.ObservationLevelDebug
	default:
		return model.ObservationLevelDefault
	}
}

// Helper functions

// timePtr returns a pointer to the given time.
func timePtr(t time.Time) *time.Time {
	return &t
}
