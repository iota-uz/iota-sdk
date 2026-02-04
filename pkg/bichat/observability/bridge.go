package observability

import (
	"context"
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

	// Correlation state
	mu                 sync.RWMutex
	pendingGenerations map[string]*simplePendingGeneration
	cleanupStop        chan struct{}
	cleanupDone        chan struct{}
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
func NewEventBridge(eventBus hooks.EventBus, providers []Provider) *EventBridge {
	if eventBus == nil {
		eventBus = hooks.NewEventBus()
	}

	bridge := &EventBridge{
		eventBus:           eventBus,
		providers:          providers,
		handlers:           make([]hooks.EventHandler, 0, len(providers)),
		pendingGenerations: make(map[string]*simplePendingGeneration),
		cleanupStop:        make(chan struct{}),
		cleanupDone:        make(chan struct{}),
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

	// Subscribe to LLMRequestEvent for correlation
	eventBus.Subscribe(&llmRequestHandler{bridge: bridge}, string(hooks.EventLLMRequest))

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
	case *events.LLMResponseEvent:
		return h.handleLLMResponse(ctx, e)
	case *events.ToolCompleteEvent:
		return h.handleToolComplete(ctx, e)
	case *events.ToolErrorEvent:
		return h.handleToolError(ctx, e)
	case *events.ContextCompileEvent:
		return h.handleContextCompile(ctx, e)
	default:
		// Convert generic events to EventObservation
		return h.handleGenericEvent(ctx, event)
	}
}

func (h *providerHandler) handleLLMResponse(ctx context.Context, e *events.LLMResponseEvent) error {
	// Find matching pending generation (correlation)
	var matchedGen *simplePendingGeneration

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
	h.bridge.mu.RUnlock()

	// NOTE: We don't delete the correlation data here to support multi-provider.
	// Multiple providers can use the same correlation data.
	// Orphan cleanup will remove stale entries after 5 minutes.

	// Populate observation with correlated data (graceful degradation)
	promptMessages := 0
	tools := 0
	if matchedGen != nil {
		promptMessages = matchedGen.messages
		tools = matchedGen.tools
	}

	obs := GenerationObservation{
		ID:               uuid.New().String(),
		TraceID:          e.SessionID().String(),
		TenantID:         e.TenantID(),
		SessionID:        e.SessionID(),
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
		Attributes:       make(map[string]interface{}),
	}

	return h.provider.RecordGeneration(ctx, obs)
}

func (h *providerHandler) handleToolComplete(ctx context.Context, e *events.ToolCompleteEvent) error {
	obs := SpanObservation{
		ID:        uuid.New().String(),
		TraceID:   e.SessionID().String(),
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

	return h.provider.RecordSpan(ctx, obs)
}

func (h *providerHandler) handleToolError(ctx context.Context, e *events.ToolErrorEvent) error {
	obs := SpanObservation{
		ID:        uuid.New().String(),
		TraceID:   e.SessionID().String(),
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

	return h.provider.RecordSpan(ctx, obs)
}

func (h *providerHandler) handleContextCompile(ctx context.Context, e *events.ContextCompileEvent) error {
	obs := SpanObservation{
		ID:        uuid.New().String(),
		TraceID:   e.SessionID().String(),
		TenantID:  e.TenantID(),
		SessionID: e.SessionID(),
		Timestamp: e.Timestamp(),
		Name:      "context.compile",
		Type:      "context",
		Input:     "",
		Output:    "",
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

	return h.provider.RecordSpan(ctx, obs)
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
		Message:    "",
		Level:      "info",
		Attributes: make(map[string]interface{}),
	}

	return h.provider.RecordEvent(ctx, obs)
}
