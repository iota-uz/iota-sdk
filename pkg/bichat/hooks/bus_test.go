package hooks_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
)

func TestEventBus_PublishSubscribe(t *testing.T) {
	t.Parallel()

	bus := hooks.NewEventBus()
	ctx := context.Background()

	// Track received events
	var mu sync.Mutex
	receivedEvents := []hooks.Event{}

	handler := hooks.EventHandlerFunc(func(ctx context.Context, event hooks.Event) error {
		mu.Lock()
		defer mu.Unlock()
		receivedEvents = append(receivedEvents, event)
		return nil
	})

	// Subscribe to LLM request events
	unsubscribe := bus.Subscribe(handler, "llm.request")
	defer unsubscribe()

	// Publish LLM request event
	sessionID := uuid.New()
	tenantID := uuid.New()
	event := events.NewLLMRequestEvent(sessionID, tenantID, "claude-3-5-sonnet", "anthropic", 10, 5, 1000)

	err := bus.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify event was received
	mu.Lock()
	defer mu.Unlock()
	if len(receivedEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(receivedEvents))
	}

	receivedEvent := receivedEvents[0].(*events.LLMRequestEvent)
	if receivedEvent.Model != "claude-3-5-sonnet" {
		t.Errorf("Expected model 'claude-3-5-sonnet', got '%s'", receivedEvent.Model)
	}
}

func TestEventBus_SubscribeAll(t *testing.T) {
	t.Parallel()

	bus := hooks.NewEventBus()
	ctx := context.Background()

	// Track received events
	var mu sync.Mutex
	receivedCount := 0

	handler := hooks.EventHandlerFunc(func(ctx context.Context, event hooks.Event) error {
		mu.Lock()
		defer mu.Unlock()
		receivedCount++
		return nil
	})

	// Subscribe to all events
	unsubscribe := bus.SubscribeAll(handler)
	defer unsubscribe()

	// Publish multiple event types
	sessionID := uuid.New()
	tenantID := uuid.New()

	if err := bus.Publish(ctx, events.NewLLMRequestEvent(sessionID, tenantID, "model", "provider", 1, 1, 100)); err != nil {
		t.Fatalf("Failed to publish LLM request event: %v", err)
	}
	if err := bus.Publish(ctx, events.NewToolStartEvent(sessionID, tenantID, "tool", "args", "id1")); err != nil {
		t.Fatalf("Failed to publish tool start event: %v", err)
	}
	if err := bus.Publish(ctx, events.NewSessionCreateEvent(sessionID, tenantID, 123, "title")); err != nil {
		t.Fatalf("Failed to publish session create event: %v", err)
	}

	// Verify all events were received
	mu.Lock()
	defer mu.Unlock()
	if receivedCount != 3 {
		t.Fatalf("Expected 3 events, got %d", receivedCount)
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	t.Parallel()

	bus := hooks.NewEventBus()
	ctx := context.Background()

	// Track received events
	var mu sync.Mutex
	receivedCount := 0

	handler := hooks.EventHandlerFunc(func(ctx context.Context, event hooks.Event) error {
		mu.Lock()
		defer mu.Unlock()
		receivedCount++
		return nil
	})

	// Subscribe and immediately unsubscribe
	unsubscribe := bus.Subscribe(handler, "llm.request")
	unsubscribe()

	// Publish event
	sessionID := uuid.New()
	tenantID := uuid.New()
	if err := bus.Publish(ctx, events.NewLLMRequestEvent(sessionID, tenantID, "model", "provider", 1, 1, 100)); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Verify event was NOT received
	mu.Lock()
	defer mu.Unlock()
	if receivedCount != 0 {
		t.Fatalf("Expected 0 events after unsubscribe, got %d", receivedCount)
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	t.Parallel()

	bus := hooks.NewEventBus()
	ctx := context.Background()

	// Track received events for each subscriber
	var mu1, mu2 sync.Mutex
	count1, count2 := 0, 0

	handler1 := hooks.EventHandlerFunc(func(ctx context.Context, event hooks.Event) error {
		mu1.Lock()
		defer mu1.Unlock()
		count1++
		return nil
	})

	handler2 := hooks.EventHandlerFunc(func(ctx context.Context, event hooks.Event) error {
		mu2.Lock()
		defer mu2.Unlock()
		count2++
		return nil
	})

	// Subscribe both handlers to LLM events
	unsubscribe1 := bus.Subscribe(handler1, "llm.request", "llm.response")
	unsubscribe2 := bus.Subscribe(handler2, "llm.request", "llm.response")
	defer unsubscribe1()
	defer unsubscribe2()

	// Publish event
	sessionID := uuid.New()
	tenantID := uuid.New()
	if err := bus.Publish(ctx, events.NewLLMRequestEvent(sessionID, tenantID, "model", "provider", 1, 1, 100)); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Verify both subscribers received the event
	mu1.Lock()
	mu2.Lock()
	defer mu1.Unlock()
	defer mu2.Unlock()

	if count1 != 1 {
		t.Errorf("Handler1: Expected 1 event, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("Handler2: Expected 1 event, got %d", count2)
	}
}

func TestEventBus_TypeFiltering(t *testing.T) {
	t.Parallel()

	bus := hooks.NewEventBus()
	ctx := context.Background()

	// Track received events
	var mu sync.Mutex
	receivedTypes := []string{}

	handler := hooks.EventHandlerFunc(func(ctx context.Context, event hooks.Event) error {
		mu.Lock()
		defer mu.Unlock()
		receivedTypes = append(receivedTypes, event.Type())
		return nil
	})

	// Subscribe only to tool events
	unsubscribe := bus.Subscribe(handler, "tool.start", "tool.complete")
	defer unsubscribe()

	// Publish various event types
	sessionID := uuid.New()
	tenantID := uuid.New()

	if err := bus.Publish(ctx, events.NewLLMRequestEvent(sessionID, tenantID, "model", "provider", 1, 1, 100)); err != nil {
		t.Fatalf("Failed to publish LLM request event: %v", err)
	}
	if err := bus.Publish(ctx, events.NewToolStartEvent(sessionID, tenantID, "tool", "args", "id1")); err != nil {
		t.Fatalf("Failed to publish tool start event: %v", err)
	}
	if err := bus.Publish(ctx, events.NewToolCompleteEvent(sessionID, tenantID, "tool", "args", "id1", "result", 100)); err != nil {
		t.Fatalf("Failed to publish tool complete event: %v", err)
	}
	if err := bus.Publish(ctx, events.NewSessionCreateEvent(sessionID, tenantID, 123, "title")); err != nil {
		t.Fatalf("Failed to publish session create event: %v", err)
	}

	// Verify only tool events were received
	mu.Lock()
	defer mu.Unlock()

	if len(receivedTypes) != 2 {
		t.Fatalf("Expected 2 tool events, got %d events", len(receivedTypes))
	}

	if receivedTypes[0] != "tool.start" {
		t.Errorf("Expected 'tool.start', got '%s'", receivedTypes[0])
	}
	if receivedTypes[1] != "tool.complete" {
		t.Errorf("Expected 'tool.complete', got '%s'", receivedTypes[1])
	}
}
