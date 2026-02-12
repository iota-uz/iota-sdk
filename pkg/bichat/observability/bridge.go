package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/handlers"
)

// simplePendingGeneration tracks an LLM request waiting for its response (simple bridge).
type simplePendingGeneration struct {
	sessionID       uuid.UUID
	timestamp       time.Time
	messages        int
	tools           int
	estimatedTokens int
	userInput       string
}

// BridgeOption configures optional EventBridge behavior.
type BridgeOption func(*EventBridge)

// WithUserIDFromContext sets a function to extract user ID from request context.
// The returned string is used as the Langfuse trace UserID.
func WithUserIDFromContext(fn func(context.Context) string) BridgeOption {
	return func(b *EventBridge) {
		b.userIDFromCtx = fn
	}
}

// WithUserEmailFromContext sets a function to extract user email from request context.
// The returned string is stored in Langfuse trace metadata as "user_email".
func WithUserEmailFromContext(fn func(context.Context) string) BridgeOption {
	return func(b *EventBridge) {
		b.userEmailFromCtx = fn
	}
}

// WithModelPricing sets model pricing for cost calculation on generations.
// Pricing fields are added to GenerationObservation.Attributes so the provider
// can compute cost from token counts.
func WithModelPricing(inputPer1M, outputPer1M, cacheWritePer1M, cacheReadPer1M float64) BridgeOption {
	return func(b *EventBridge) {
		b.pricing = &modelPricing{
			InputPer1M:      inputPer1M,
			OutputPer1M:     outputPer1M,
			CacheWritePer1M: cacheWritePer1M,
			CacheReadPer1M:  cacheReadPer1M,
		}
	}
}

type modelPricing struct {
	InputPer1M      float64
	OutputPer1M     float64
	CacheWritePer1M float64
	CacheReadPer1M  float64
}

// EventBridge connects BiChat's EventBus to observability providers.
// It subscribes to BiChat events and maps them to provider observations.
//
// Features:
// - Non-blocking: Providers wrapped in AsyncHandler to prevent blocking main execution
// - Multi-provider: Supports multiple observability backends simultaneously
// - Request-response correlation: Matches LLM request/response events
// - Graceful shutdown: Flushes pending data before closing
type EventBridge struct {
	eventBus  hooks.EventBus
	providers []Provider
	handlers  []hooks.EventHandler

	// Optional context extractors
	userIDFromCtx    func(context.Context) string
	userEmailFromCtx func(context.Context) string
	pricing          *modelPricing

	// Correlation state
	mu                 sync.RWMutex
	pendingGenerations map[string]*simplePendingGeneration
	agentSpans         map[uuid.UUID]*agentSpanState // sessionID → agent span state
	lastGenerationIDs  map[uuid.UUID]string          // sessionID → last generation span ID
	cleanupStop        chan struct{}
	cleanupDone        chan struct{}
}

// agentSpanState tracks an in-flight agent span.
type agentSpanState struct {
	spanID    string
	startTime time.Time
}

// NewEventBridge creates an EventBridge that connects BiChat events to observability providers.
// Each provider is wrapped in an AsyncHandler with a buffer to prevent blocking.
//
// Usage:
//
//	bridge := observability.NewEventBridge(eventBus, []observability.Provider{
//	    langfuseProvider,
//	    customProvider,
//	})
//	defer bridge.Shutdown(context.Background())
func NewEventBridge(eventBus hooks.EventBus, providers []Provider, opts ...BridgeOption) *EventBridge {
	if eventBus == nil {
		eventBus = hooks.NewEventBus()
	}

	bridge := &EventBridge{
		eventBus:           eventBus,
		providers:          providers,
		handlers:           make([]hooks.EventHandler, 0, len(providers)),
		pendingGenerations: make(map[string]*simplePendingGeneration),
		agentSpans:         make(map[uuid.UUID]*agentSpanState),
		lastGenerationIDs:  make(map[uuid.UUID]string),
		cleanupStop:        make(chan struct{}),
		cleanupDone:        make(chan struct{}),
	}

	for _, opt := range opts {
		opt(bridge)
	}

	// Wrap each provider in AsyncHandler and subscribe to events
	for _, provider := range providers {
		handler := &providerHandler{
			provider: provider,
			bridge:   bridge,
		}

		// Wrap in AsyncHandler to prevent blocking
		asyncHandler := handlers.NewAsyncHandler(handler, 100) // 100-event buffer
		bridge.handlers = append(bridge.handlers, asyncHandler)

		// Subscribe to all BiChat events
		eventBus.SubscribeAll(asyncHandler)
	}

	// Subscribe synchronous handlers for correlation state
	eventBus.Subscribe(&llmRequestHandler{bridge: bridge}, string(hooks.EventLLMRequest))
	eventBus.Subscribe(&agentStartHandler{bridge: bridge}, string(hooks.EventAgentStart))

	// Start cleanup goroutine
	go bridge.cleanupOrphans()

	return bridge
}

// Shutdown gracefully shuts down all providers.
// It flushes pending observations and releases resources.
//
// Shutdown process:
// 1. Stop cleanup goroutine
// 2. Call Flush() on each provider to send pending data
// 3. Call Shutdown() on each provider to release resources
// 4. Return first error encountered (if any)
func (b *EventBridge) Shutdown(ctx context.Context) error {
	// Stop cleanup goroutine FIRST
	if b.cleanupStop != nil {
		close(b.cleanupStop)
		<-b.cleanupDone
	}

	var firstErr error

	for _, provider := range b.providers {
		// Flush pending observations
		if flusher, ok := provider.(interface{ Flush(context.Context) error }); ok {
			if err := flusher.Flush(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}

		// Shutdown provider
		if closer, ok := provider.(interface{ Shutdown(context.Context) error }); ok {
			if err := closer.Shutdown(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// cleanupOrphans runs periodically to remove stale pending observations.
func (b *EventBridge) cleanupOrphans() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	defer close(b.cleanupDone)

	for {
		select {
		case <-ticker.C:
			b.performCleanup()
		case <-b.cleanupStop:
			return
		}
	}
}

// performCleanup removes orphaned pending observations.
func (b *EventBridge) performCleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	orphanedCount := 0

	// Cleanup pending generations older than 5 minutes
	for key, pending := range b.pendingGenerations {
		if now.Sub(pending.timestamp) > 5*time.Minute {
			delete(b.pendingGenerations, key)
			orphanedCount++
		}
	}
}

// llmRequestHandler captures LLM requests for correlation.
type llmRequestHandler struct {
	bridge *EventBridge
}

func (h *llmRequestHandler) Handle(ctx context.Context, event hooks.Event) error {
	llmEvent, ok := event.(*events.LLMRequestEvent)
	if !ok {
		return nil
	}

	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()

	// Create correlation key: sessionID + timestamp (truncated to second)
	correlationKey := fmt.Sprintf("%s-%d", llmEvent.SessionID().String(), llmEvent.Timestamp().Unix())

	h.bridge.pendingGenerations[correlationKey] = &simplePendingGeneration{
		sessionID:       llmEvent.SessionID(),
		timestamp:       llmEvent.Timestamp(),
		messages:        llmEvent.Messages,
		tools:           llmEvent.Tools,
		estimatedTokens: llmEvent.EstimatedTokens,
		userInput:       llmEvent.UserInput,
	}

	return nil
}

// agentStartHandler initializes agent span state once per AgentStart event.
// Runs synchronously (not per-provider) so the span ID is deterministic.
type agentStartHandler struct {
	bridge *EventBridge
}

func (h *agentStartHandler) Handle(_ context.Context, event hooks.Event) error {
	e, ok := event.(*events.AgentStartEvent)
	if !ok {
		return nil
	}
	h.bridge.mu.Lock()
	defer h.bridge.mu.Unlock()
	if _, exists := h.bridge.agentSpans[e.SessionID()]; !exists {
		h.bridge.agentSpans[e.SessionID()] = &agentSpanState{
			spanID:    uuid.New().String(),
			startTime: e.Timestamp(),
		}
	}
	return nil
}

// providerHandler adapts BiChat events to Provider interface
type providerHandler struct {
	provider Provider
	bridge   *EventBridge
}

func (h *providerHandler) Handle(ctx context.Context, event hooks.Event) error {
	// Map BiChat events to Provider observations
	switch e := event.(type) {
	case *events.AgentStartEvent:
		return h.handleAgentStart(ctx, e)
	case *events.AgentCompleteEvent:
		return h.handleAgentComplete(ctx, e)
	case *events.AgentErrorEvent:
		return h.handleAgentError(ctx, e)
	case *events.LLMResponseEvent:
		return h.handleLLMResponse(ctx, e)
	case *events.LLMRequestEvent:
		return h.handleLLMRequest(ctx, e)
	case *events.ToolCompleteEvent:
		return h.handleToolComplete(ctx, e)
	case *events.ToolErrorEvent:
		return h.handleToolError(ctx, e)
	case *events.ToolStartEvent:
		return h.handleToolStart(ctx, e)
	case *events.ContextCompileEvent:
		return h.handleContextCompile(ctx, e)
	case *events.InterruptEvent:
		return h.handleInterrupt(ctx, e)
	case *events.SessionTitleUpdatedEvent:
		return h.handleSessionTitleUpdated(ctx, e)
	default:
		// Convert generic events to EventObservation
		return h.handleGenericEvent(ctx, event)
	}
}

func (h *providerHandler) handleAgentStart(_ context.Context, _ *events.AgentStartEvent) error {
	// No-op: agent span state is initialized by the synchronous agentStartHandler
	// to avoid N providers writing different span IDs for the same session.
	return nil
}

func (h *providerHandler) finalizeAgentSpan(
	ctx context.Context,
	sessionID uuid.UUID,
	tenantID uuid.UUID,
	timestamp time.Time,
	durationMs int64,
	status string,
	output string,
	attrs map[string]interface{},
) error {
	h.bridge.mu.Lock()
	as := h.bridge.agentSpans[sessionID]
	delete(h.bridge.agentSpans, sessionID)
	delete(h.bridge.lastGenerationIDs, sessionID)
	h.bridge.mu.Unlock()

	spanID := ""
	startTime := timestamp
	if as != nil {
		spanID = as.spanID
		startTime = as.startTime
	}
	if spanID == "" {
		spanID = uuid.New().String()
	}

	obs := SpanObservation{
		ID:         spanID,
		TraceID:    sessionID.String(),
		TenantID:   tenantID,
		SessionID:  sessionID,
		Timestamp:  startTime,
		Name:       "agent.execute",
		Type:       "agent",
		Duration:   time.Duration(durationMs) * time.Millisecond,
		Status:     status,
		Output:     output,
		Attributes: attrs,
	}

	return h.provider.RecordSpan(ctx, obs)
}

func (h *providerHandler) handleAgentComplete(ctx context.Context, e *events.AgentCompleteEvent) error {
	return h.finalizeAgentSpan(ctx, e.SessionID(), e.TenantID(), e.Timestamp(), e.DurationMs, "success", "", map[string]interface{}{
		"agent_name":   e.AgentName,
		"iterations":   e.Iterations,
		"total_tokens": e.TotalTokens,
	})
}

func (h *providerHandler) handleAgentError(ctx context.Context, e *events.AgentErrorEvent) error {
	return h.finalizeAgentSpan(ctx, e.SessionID(), e.TenantID(), e.Timestamp(), e.DurationMs, "error", e.Error, map[string]interface{}{
		"agent_name": e.AgentName,
		"iterations": e.Iterations,
		"error":      e.Error,
	})
}

func (h *providerHandler) handleLLMResponse(ctx context.Context, e *events.LLMResponseEvent) error {
	// Find matching pending generation (correlation)
	var matchedGen *simplePendingGeneration

	// Single RLock to read both pending generation correlation and agent span parent.
	h.bridge.mu.RLock()
	// Search within correlation window (30 seconds)
	for _, pending := range h.bridge.pendingGenerations {
		if pending.sessionID == e.SessionID() {
			timeDiff := e.Timestamp().Sub(pending.timestamp)
			if timeDiff >= 0 && timeDiff < 30*time.Second {
				matchedGen = pending
				break
			}
		}
	}
	var agentSpanID string
	if as := h.bridge.agentSpans[e.SessionID()]; as != nil {
		agentSpanID = as.spanID
	}
	h.bridge.mu.RUnlock()

	// NOTE: We don't delete the correlation data here to support multi-provider.
	// Multiple providers can use the same correlation data.
	// Orphan cleanup will remove stale entries after 5 minutes.

	// Populate observation with correlated data (graceful degradation)
	promptMessages := 0
	tools := 0
	userInput := ""
	if matchedGen != nil {
		promptMessages = matchedGen.messages
		tools = matchedGen.tools
		userInput = matchedGen.userInput
	}

	attrs := make(map[string]interface{})
	if e.CacheWriteTokens > 0 {
		attrs["cache_write_tokens"] = e.CacheWriteTokens
	}
	if e.CacheReadTokens > 0 {
		attrs["cache_read_tokens"] = e.CacheReadTokens
	}

	// Add pricing so the provider can calculate cost.
	if h.bridge.pricing != nil {
		attrs["input_price_per_1m"] = h.bridge.pricing.InputPer1M
		attrs["output_price_per_1m"] = h.bridge.pricing.OutputPer1M
		if h.bridge.pricing.CacheWritePer1M > 0 {
			attrs["cache_write_price_per_1m"] = h.bridge.pricing.CacheWritePer1M
		}
		if h.bridge.pricing.CacheReadPer1M > 0 {
			attrs["cache_read_price_per_1m"] = h.bridge.pricing.CacheReadPer1M
		}
	}

	// Resolve user ID and email from context for trace enrichment.
	var userID string
	if h.bridge.userIDFromCtx != nil {
		userID = h.bridge.userIDFromCtx(ctx)
	}
	var userEmail string
	if h.bridge.userEmailFromCtx != nil {
		userEmail = h.bridge.userEmailFromCtx(ctx)
	}

	genID := uuid.New().String()

	obs := GenerationObservation{
		ID:               genID,
		TraceID:          e.SessionID().String(),
		ParentID:         agentSpanID,
		TenantID:         e.TenantID(),
		SessionID:        e.SessionID(),
		UserID:           userID,
		UserEmail:        userEmail,
		Timestamp:        e.Timestamp(),
		Model:            e.Model,
		Provider:         e.Provider,
		PromptMessages:   promptMessages,
		PromptTokens:     e.PromptTokens,
		Tools:            tools,
		CompletionTokens: e.CompletionTokens,
		TotalTokens:      e.TotalTokens,
		LatencyMs:        e.LatencyMs,
		FinishReason:     e.FinishReason,
		ToolCalls:        e.ToolCalls,
		Duration:         time.Duration(e.LatencyMs) * time.Millisecond,
		Input:            userInput,
		Output:           e.ResponseText,
		Attributes:       attrs,
	}

	// Store this generation's ID so tool spans can parent under it
	h.bridge.mu.Lock()
	h.bridge.lastGenerationIDs[e.SessionID()] = genID
	h.bridge.mu.Unlock()

	// Prefer OpenTelemetry trace context if present on ctx.
	if traceID, spanID, ok := OTelTraceSpanIDs(ctx); ok {
		obs.TraceID = traceID
		obs.Attributes["otel.span_id"] = spanID
	}

	return h.provider.RecordGeneration(ctx, obs)
}

func (h *providerHandler) handleToolComplete(ctx context.Context, e *events.ToolCompleteEvent) error {
	// Resolve parent (last generation span) for hierarchical nesting
	h.bridge.mu.RLock()
	parentID := h.bridge.lastGenerationIDs[e.SessionID()]
	h.bridge.mu.RUnlock()

	obs := SpanObservation{
		ID:        uuid.New().String(),
		TraceID:   e.SessionID().String(),
		ParentID:  parentID,
		TenantID:  e.TenantID(),
		SessionID: e.SessionID(),
		Timestamp: e.Timestamp(),
		Name:      "tool.execute",
		Type:      "tool",
		Input:     e.Arguments,
		Output:    e.Result,
		Duration:  time.Duration(e.DurationMs) * time.Millisecond,
		Status:    "success",
		ToolName:  e.ToolName,
		CallID:    e.CallID,
		Attributes: map[string]interface{}{
			"tool_name": e.ToolName,
			"call_id":   e.CallID,
		},
	}

	// Prefer OpenTelemetry trace context if present on ctx.
	// This overrides the bridge-computed ParentID (lastGenerationID) in favor of the OTel span hierarchy.
	if traceID, spanID, ok := OTelTraceSpanIDs(ctx); ok {
		obs.TraceID = traceID
		obs.ParentID = spanID
		obs.Attributes["otel.span_id"] = spanID
	}

	return h.provider.RecordSpan(ctx, obs)
}

func (h *providerHandler) handleToolError(ctx context.Context, e *events.ToolErrorEvent) error {
	// Resolve parent (last generation span) for hierarchical nesting
	h.bridge.mu.RLock()
	parentID := h.bridge.lastGenerationIDs[e.SessionID()]
	h.bridge.mu.RUnlock()

	obs := SpanObservation{
		ID:        uuid.New().String(),
		TraceID:   e.SessionID().String(),
		ParentID:  parentID,
		TenantID:  e.TenantID(),
		SessionID: e.SessionID(),
		Timestamp: e.Timestamp(),
		Name:      "tool.execute",
		Type:      "tool",
		Input:     e.Arguments,
		Output:    e.Error,
		Duration:  time.Duration(e.DurationMs) * time.Millisecond,
		Status:    "error",
		ToolName:  e.ToolName,
		CallID:    e.CallID,
		Attributes: map[string]interface{}{
			"tool_name": e.ToolName,
			"call_id":   e.CallID,
			"error":     e.Error,
		},
	}

	// Prefer OpenTelemetry trace context if present on ctx.
	if traceID, spanID, ok := OTelTraceSpanIDs(ctx); ok {
		obs.TraceID = traceID
		obs.ParentID = spanID
		obs.Attributes["otel.span_id"] = spanID
	}

	return h.provider.RecordSpan(ctx, obs)
}

func (h *providerHandler) handleContextCompile(ctx context.Context, e *events.ContextCompileEvent) error {
	inputSummary := map[string]interface{}{
		"provider":    e.Provider,
		"block_count": e.BlockCount,
	}
	inputJSON, err := json.Marshal(inputSummary)
	if err != nil {
		inputJSON = []byte(`{"error":"marshal"}`)
	}

	outputSummary := map[string]interface{}{
		"total_tokens":    e.TotalTokens,
		"tokens_by_kind":  e.TokensByKind,
		"truncated":       e.Truncated,
		"compacted":       e.Compacted,
		"excluded_blocks": e.ExcludedBlocks,
	}
	outputJSON, err := json.Marshal(outputSummary)
	if err != nil {
		outputJSON = []byte(`{"error":"marshal"}`)
	}

	obs := SpanObservation{
		ID:        uuid.New().String(),
		TraceID:   e.SessionID().String(),
		TenantID:  e.TenantID(),
		SessionID: e.SessionID(),
		Timestamp: e.Timestamp(),
		Name:      "context.compile",
		Type:      "context",
		Input:     string(inputJSON),
		Output:    string(outputJSON),
		Duration:  0, // Context compilation is instantaneous
		Status:    "success",
		Attributes: map[string]interface{}{
			"provider":        e.Provider,
			"total_tokens":    e.TotalTokens,
			"block_count":     e.BlockCount,
			"compacted":       e.Compacted,
			"truncated":       e.Truncated,
			"excluded_blocks": e.ExcludedBlocks,
			"tokens_by_kind":  e.TokensByKind,
		},
	}

	// Prefer OpenTelemetry trace context if present on ctx.
	if traceID, spanID, ok := OTelTraceSpanIDs(ctx); ok {
		obs.TraceID = traceID
		obs.ParentID = spanID
		obs.Attributes["otel.span_id"] = spanID
	}

	return h.provider.RecordSpan(ctx, obs)
}

func (h *providerHandler) handleInterrupt(ctx context.Context, e *events.InterruptEvent) error {
	inputSummary := map[string]interface{}{
		"interrupt_type": e.InterruptType,
		"agent_name":     e.AgentName,
	}
	inputJSON, err := json.Marshal(inputSummary)
	if err != nil {
		inputJSON = []byte(`{"error":"marshal"}`)
	}

	outputSummary := map[string]interface{}{
		"question":      e.Question,
		"checkpoint_id": e.CheckpointID,
	}
	outputJSON, err := json.Marshal(outputSummary)
	if err != nil {
		outputJSON = []byte(`{"error":"marshal"}`)
	}

	obs := SpanObservation{
		ID:        uuid.New().String(),
		TraceID:   e.SessionID().String(),
		TenantID:  e.TenantID(),
		SessionID: e.SessionID(),
		Timestamp: e.Timestamp(),
		Name:      "interrupt",
		Type:      "session",
		Input:     string(inputJSON),
		Output:    string(outputJSON),
		Duration:  0,
		Status:    "success",
		Attributes: map[string]interface{}{
			"interrupt_type": e.InterruptType,
			"agent_name":     e.AgentName,
			"checkpoint_id":  e.CheckpointID,
		},
	}
	if traceID, spanID, ok := OTelTraceSpanIDs(ctx); ok {
		obs.TraceID = traceID
		obs.ParentID = spanID
		obs.Attributes["otel.span_id"] = spanID
	}
	return h.provider.RecordSpan(ctx, obs)
}

func (h *providerHandler) handleSessionTitleUpdated(ctx context.Context, e *events.SessionTitleUpdatedEvent) error {
	if updater, ok := h.provider.(TraceNameUpdater); ok {
		return updater.UpdateTraceName(ctx, e.SessionID().String(), e.Title)
	}
	return nil
}

func (h *providerHandler) handleToolStart(_ context.Context, _ *events.ToolStartEvent) error {
	// No-op: tool.complete already captures the full span with input, output, duration, and status.
	// Previously this created a separate span with a different UUID, resulting in duplicate noise.
	return nil
}

func (h *providerHandler) handleLLMRequest(_ context.Context, _ *events.LLMRequestEvent) error {
	// No-op: llm.response creates the proper GenerationObservation with all data.
	// Request data is captured by llmRequestHandler for correlation.
	// Previously this created a redundant "pending" span alongside the generation.
	return nil
}

func (h *providerHandler) handleGenericEvent(ctx context.Context, event hooks.Event) error {
	obs := EventObservation{
		ID:         uuid.New().String(),
		TraceID:    event.SessionID().String(),
		TenantID:   event.TenantID(),
		SessionID:  event.SessionID(),
		Timestamp:  event.Timestamp(),
		Name:       event.Type(),
		Type:       "custom",
		Level:      "info",
		Attributes: make(map[string]interface{}),
	}

	// Prefer OpenTelemetry trace context if present on ctx.
	if traceID, spanID, ok := OTelTraceSpanIDs(ctx); ok {
		obs.TraceID = traceID
		obs.Attributes["otel.span_id"] = spanID
	}

	return h.provider.RecordEvent(ctx, obs)
}
