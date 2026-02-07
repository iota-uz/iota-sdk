package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider for testing
type mockProvider struct {
	mu          sync.Mutex
	generations []GenerationObservation
	spans       []SpanObservation
	events      []EventObservation
}

func (m *mockProvider) RecordGeneration(ctx context.Context, obs GenerationObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generations = append(m.generations, obs)
	return nil
}

func (m *mockProvider) RecordSpan(ctx context.Context, obs SpanObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spans = append(m.spans, obs)
	return nil
}

func (m *mockProvider) RecordEvent(ctx context.Context, obs EventObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, obs)
	return nil
}

func (m *mockProvider) RecordTrace(ctx context.Context, obs TraceObservation) error {
	// No-op for testing
	return nil
}

func (m *mockProvider) getGenerations() []GenerationObservation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]GenerationObservation{}, m.generations...)
}

func (m *mockProvider) getSpans() []SpanObservation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]SpanObservation{}, m.spans...)
}

func TestEventBridge_RequestResponseCorrelation(t *testing.T) {
	t.Parallel()

	// Setup
	bus := hooks.NewEventBus()
	provider := &mockProvider{}
	bridge := NewEventBridge(bus, []Provider{provider})
	defer func() { _ = bridge.Shutdown(context.Background()) }()

	sessionID := uuid.New()
	tenantID := uuid.New()

	// Emit LLM request
	requestEvent := events.NewLLMRequestEvent(
		sessionID, tenantID,
		"claude-3-5-sonnet-20241022", "anthropic",
		3,                 // messages
		5,                 // tools
		1000,              // estimatedTokens
		"test user input", // userInput
	)
	require.NoError(t, bus.Publish(context.Background(), requestEvent))

	// Wait a bit for async handling
	time.Sleep(50 * time.Millisecond)

	// Emit LLM response (2 seconds later)
	time.Sleep(100 * time.Millisecond)
	responseEvent := events.NewLLMResponseEvent(
		sessionID, tenantID,
		"claude-3-5-sonnet-20241022", "anthropic",
		950, 120, 1070, // tokens
		1234, // latencyMs
		"stop",
		2,                    // toolCalls
		"test response text", // responseText
	)
	require.NoError(t, bus.Publish(context.Background(), responseEvent))

	// Wait for async handling
	time.Sleep(100 * time.Millisecond)

	// Verify
	generations := provider.getGenerations()
	require.Len(t, generations, 1)

	obs := generations[0]
	assert.Equal(t, 3, obs.PromptMessages, "PromptMessages should be correlated")
	assert.Equal(t, 5, obs.Tools, "Tools should be correlated")
	assert.Equal(t, 950, obs.PromptTokens)
	assert.Equal(t, 120, obs.CompletionTokens)

	// Verify pendingGenerations still has the entry (multi-provider support)
	// It will be cleaned up by orphan cleanup after 5 minutes
	bridge.mu.RLock()
	defer bridge.mu.RUnlock()
	assert.Len(t, bridge.pendingGenerations, 1, "Pending generation should remain for multi-provider support")
}

func TestEventBridge_MissingRequest(t *testing.T) {
	t.Parallel()

	// Setup
	bus := hooks.NewEventBus()
	provider := &mockProvider{}
	bridge := NewEventBridge(bus, []Provider{provider})
	defer func() { _ = bridge.Shutdown(context.Background()) }()

	sessionID := uuid.New()
	tenantID := uuid.New()

	// Emit LLM response WITHOUT prior request
	responseEvent := events.NewLLMResponseEvent(
		sessionID, tenantID,
		"claude-3-5-sonnet-20241022", "anthropic",
		950, 120, 1070,
		1234,
		"stop",
		2,
		"test response text",
	)
	require.NoError(t, bus.Publish(context.Background(), responseEvent))

	// Wait for async handling
	time.Sleep(100 * time.Millisecond)

	// Verify graceful degradation
	generations := provider.getGenerations()
	require.Len(t, generations, 1)

	obs := generations[0]
	assert.Equal(t, 0, obs.PromptMessages, "PromptMessages should be 0 (graceful degradation)")
	assert.Equal(t, 0, obs.Tools, "Tools should be 0 (graceful degradation)")
	assert.Equal(t, 950, obs.PromptTokens, "Other fields should still work")
}

func TestEventBridge_OrphanCleanup(t *testing.T) {
	t.Parallel()

	// Setup
	bus := hooks.NewEventBus()
	provider := &mockProvider{}
	bridge := NewEventBridge(bus, []Provider{provider})
	defer func() { _ = bridge.Shutdown(context.Background()) }()

	sessionID := uuid.New()

	// Manually insert old pending generation (6 minutes ago)
	bridge.mu.Lock()
	correlationKey := fmt.Sprintf("%s-%d", sessionID.String(), time.Now().Add(-6*time.Minute).Unix())
	bridge.pendingGenerations[correlationKey] = &simplePendingGeneration{
		sessionID: sessionID,
		timestamp: time.Now().Add(-6 * time.Minute),
		messages:  3,
		tools:     5,
		userInput: "old user input",
	}
	bridge.mu.Unlock()

	// Trigger cleanup manually
	bridge.performCleanup()

	// Verify orphan removed
	bridge.mu.RLock()
	defer bridge.mu.RUnlock()
	assert.Empty(t, bridge.pendingGenerations, "Orphans should be cleaned up")
}

func TestEventBridge_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	// Setup
	bus := hooks.NewEventBus()
	provider := &mockProvider{}
	bridge := NewEventBridge(bus, []Provider{provider})
	defer func() { _ = bridge.Shutdown(context.Background()) }()

	tenantID := uuid.New()
	var wg sync.WaitGroup
	errCh := make(chan error, 20)

	// Launch 10 goroutines emitting requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessionID := uuid.New()
			requestEvent := events.NewLLMRequestEvent(
				sessionID, tenantID,
				"claude-3-5-sonnet-20241022", "anthropic",
				3, 5, 1000,
				"test user input",
			)
			errCh <- bus.Publish(context.Background(), requestEvent)
		}()
	}

	// Launch 10 goroutines emitting responses
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessionID := uuid.New()
			responseEvent := events.NewLLMResponseEvent(
				sessionID, tenantID,
				"claude-3-5-sonnet-20241022", "anthropic",
				950, 120, 1070, 1234, "stop", 2,
				"test response text",
			)
			errCh <- bus.Publish(context.Background(), responseEvent)
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	time.Sleep(200 * time.Millisecond)

	// Verify no panics (test passes if no race conditions)
	generations := provider.getGenerations()
	assert.GreaterOrEqual(t, len(generations), 10, "Should have at least 10 generations")
}

func TestEventBridge_MultiProvider(t *testing.T) {
	t.Parallel()

	// Setup with 3 providers
	bus := hooks.NewEventBus()
	provider1 := &mockProvider{}
	provider2 := &mockProvider{}
	provider3 := &mockProvider{}
	bridge := NewEventBridge(bus, []Provider{provider1, provider2, provider3})
	defer func() { _ = bridge.Shutdown(context.Background()) }()

	sessionID := uuid.New()
	tenantID := uuid.New()

	// Emit request and response
	requestEvent := events.NewLLMRequestEvent(
		sessionID, tenantID,
		"claude-3-5-sonnet-20241022", "anthropic",
		3, 5, 1000,
		"test user input",
	)
	require.NoError(t, bus.Publish(context.Background(), requestEvent))
	time.Sleep(50 * time.Millisecond)

	responseEvent := events.NewLLMResponseEvent(
		sessionID, tenantID,
		"claude-3-5-sonnet-20241022", "anthropic",
		950, 120, 1070, 1234, "stop", 2,
		"test response text",
	)
	require.NoError(t, bus.Publish(context.Background(), responseEvent))

	// Wait longer for all async handlers (3 providers) to process
	time.Sleep(300 * time.Millisecond)

	// Verify ALL providers got same data
	for idx, provider := range []*mockProvider{provider1, provider2, provider3} {
		generations := provider.getGenerations()
		require.Len(t, generations, 1, "Provider %d should have 1 generation", idx+1)
		assert.Equal(t, 3, generations[0].PromptMessages, "Provider %d PromptMessages", idx+1)
		assert.Equal(t, 5, generations[0].Tools, "Provider %d Tools", idx+1)
	}
}

func TestEventBridge_ContextCompileSpanHasInputOutput(t *testing.T) {
	t.Parallel()

	bus := hooks.NewEventBus()
	provider := &mockProvider{}
	bridge := NewEventBridge(bus, []Provider{provider})
	defer func() { _ = bridge.Shutdown(context.Background()) }()

	sessionID := uuid.New()
	tenantID := uuid.New()
	tokensByKind := map[string]int{"history": 13, "pinned": 980, "turn": 13}

	event := events.NewContextCompileEvent(
		sessionID, tenantID,
		"anthropic",
		1006,
		tokensByKind,
		3,
		false, false,
		0,
	)
	require.NoError(t, bus.Publish(context.Background(), event))

	time.Sleep(100 * time.Millisecond)

	spans := provider.getSpans()
	require.Len(t, spans, 1)
	obs := spans[0]
	assert.Equal(t, "context.compile", obs.Name)
	assert.Equal(t, "context", obs.Type)

	require.NotEmpty(t, obs.Input, "Input should be set")
	require.NotEmpty(t, obs.Output, "Output should be set")

	var inputMap map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(obs.Input), &inputMap), "Input should be valid JSON")
	assert.Equal(t, "anthropic", inputMap["provider"])
	assert.EqualValues(t, 3, inputMap["block_count"])

	var outputMap map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(obs.Output), &outputMap), "Output should be valid JSON")
	assert.EqualValues(t, 1006, outputMap["total_tokens"])
	tbk, ok := outputMap["tokens_by_kind"].(map[string]interface{})
	require.True(t, ok, "tokens_by_kind should be an object")
	assert.EqualValues(t, 13, tbk["history"])
	assert.EqualValues(t, 980, tbk["pinned"])
	assert.EqualValues(t, 13, tbk["turn"])
	assert.Equal(t, false, outputMap["truncated"])
	assert.Equal(t, false, outputMap["compacted"])
	assert.EqualValues(t, 0, outputMap["excluded_blocks"])
}

func TestEventBridge_InterruptSpanHasInputOutput(t *testing.T) {
	t.Parallel()
	bus := hooks.NewEventBus()
	provider := &mockProvider{}
	bridge := NewEventBridge(bus, []Provider{provider})
	defer func() { _ = bridge.Shutdown(context.Background()) }()
	sessionID := uuid.New()
	tenantID := uuid.New()
	event := events.NewInterruptEvent(sessionID, tenantID, "ask_user_question", "default", "Which date range?", "cp-123")
	require.NoError(t, bus.Publish(context.Background(), event))
	time.Sleep(100 * time.Millisecond)
	spans := provider.getSpans()
	require.Len(t, spans, 1)
	obs := spans[0]
	assert.Equal(t, "interrupt", obs.Name)
	require.NotEmpty(t, obs.Input)
	require.NotEmpty(t, obs.Output)
	var inputMap map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(obs.Input), &inputMap))
	assert.Equal(t, "ask_user_question", inputMap["interrupt_type"])
	assert.Equal(t, "default", inputMap["agent_name"])
	var outputMap map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(obs.Output), &outputMap))
	assert.Equal(t, "Which date range?", outputMap["question"])
	assert.Equal(t, "cp-123", outputMap["checkpoint_id"])
}

func TestEventBridge_ToolStartSpanHasInputOutput(t *testing.T) {
	t.Parallel()
	bus := hooks.NewEventBus()
	provider := &mockProvider{}
	bridge := NewEventBridge(bus, []Provider{provider})
	defer func() { _ = bridge.Shutdown(context.Background()) }()
	sessionID := uuid.New()
	tenantID := uuid.New()
	event := events.NewToolStartEvent(sessionID, tenantID, "sql_execute", `{"query":"SELECT 1"}`, "call-1")
	require.NoError(t, bus.Publish(context.Background(), event))
	time.Sleep(100 * time.Millisecond)
	spans := provider.getSpans()
	require.Len(t, spans, 1)
	obs := spans[0]
	assert.Equal(t, "tool.start", obs.Name)
	require.NotEmpty(t, obs.Input)
	require.NotEmpty(t, obs.Output)
	assert.Equal(t, `{"query":"SELECT 1"}`, obs.Input)
	var outputMap map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(obs.Output), &outputMap))
	assert.Equal(t, "started", outputMap["status"])
	assert.Equal(t, "sql_execute", outputMap["tool"])
	assert.Equal(t, "call-1", outputMap["call_id"])
}

func TestEventBridge_LLMRequestSpanHasInputOutput(t *testing.T) {
	t.Parallel()
	bus := hooks.NewEventBus()
	provider := &mockProvider{}
	bridge := NewEventBridge(bus, []Provider{provider})
	defer func() { _ = bridge.Shutdown(context.Background()) }()
	sessionID := uuid.New()
	tenantID := uuid.New()
	event := events.NewLLMRequestEvent(sessionID, tenantID, "claude-3-5-sonnet", "anthropic", 5, 3, 1000, "Show me sales")
	require.NoError(t, bus.Publish(context.Background(), event))
	time.Sleep(100 * time.Millisecond)
	spans := provider.getSpans()
	require.Len(t, spans, 1)
	obs := spans[0]
	assert.Equal(t, "llm.request", obs.Name)
	require.NotEmpty(t, obs.Input)
	require.NotEmpty(t, obs.Output)
	var inputMap map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(obs.Input), &inputMap))
	assert.Equal(t, "claude-3-5-sonnet", inputMap["model"])
	assert.Equal(t, "anthropic", inputMap["provider"])
	assert.EqualValues(t, 5, inputMap["messages"])
	assert.EqualValues(t, 3, inputMap["tools"])
	assert.Equal(t, "Show me sales", inputMap["user_input"])
	var outputMap map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(obs.Output), &outputMap))
	assert.Equal(t, "pending", outputMap["status"])
}
