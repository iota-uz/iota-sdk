package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestInterruptHandlerRegistry_Register(t *testing.T) {
	t.Parallel()

	registry := NewInterruptHandlerRegistry()
	handler := &mockInterruptHandler{}

	registry.Register("test_event", handler)

	retrieved, ok := registry.Get("test_event")
	if !ok {
		t.Fatal("expected handler to be registered")
	}

	if retrieved != handler {
		t.Error("expected retrieved handler to be the same as registered")
	}
}

func TestInterruptHandlerRegistry_Get_NotFound(t *testing.T) {
	t.Parallel()

	registry := NewInterruptHandlerRegistry()

	_, ok := registry.Get("nonexistent")
	if ok {
		t.Error("expected handler to not be found")
	}
}

func TestInterruptHandlerRegistry_Replace(t *testing.T) {
	t.Parallel()

	registry := NewInterruptHandlerRegistry()
	handler1 := &mockInterruptHandler{name: "handler1"}
	handler2 := &mockInterruptHandler{name: "handler2"}

	registry.Register("test_event", handler1)
	registry.Register("test_event", handler2)

	retrieved, ok := registry.Get("test_event")
	if !ok {
		t.Fatal("expected handler to be registered")
	}

	if retrieved != handler2 {
		t.Error("expected second handler to replace first")
	}
}

func TestInterruptHandlerRegistry_All(t *testing.T) {
	t.Parallel()

	registry := NewInterruptHandlerRegistry()
	handler1 := &mockInterruptHandler{name: "handler1"}
	handler2 := &mockInterruptHandler{name: "handler2"}

	registry.Register("event1", handler1)
	registry.Register("event2", handler2)

	all := registry.All()

	// Should have at least our 2 custom handlers plus built-in ask_user_question
	if len(all) < 2 {
		t.Errorf("expected at least 2 handlers, got %d", len(all))
	}

	if all["event1"] != handler1 {
		t.Error("expected handler1 to be in all handlers")
	}

	if all["event2"] != handler2 {
		t.Error("expected handler2 to be in all handlers")
	}

	// Verify built-in handler is registered
	if _, ok := all["ask_user_question"]; !ok {
		t.Error("expected built-in ask_user_question handler to be registered")
	}
}

func TestInterruptHandlerRegistry_BuiltInHandler(t *testing.T) {
	t.Parallel()

	registry := NewInterruptHandlerRegistry()

	// Verify ask_user_question handler is pre-registered
	handler, ok := registry.Get("ask_user_question")
	if !ok {
		t.Fatal("expected built-in ask_user_question handler to be registered")
	}

	if _, ok := handler.(*AskUserQuestionHandler); !ok {
		t.Error("expected handler to be of type *AskUserQuestionHandler")
	}
}

func TestAskUserQuestionHandler_Handle(t *testing.T) {
	t.Parallel()

	handler := &AskUserQuestionHandler{}
	ctx := context.Background()

	questionData := map[string]string{
		"question": "What is your favorite color?",
	}
	data, err := json.Marshal(questionData)
	if err != nil {
		t.Fatalf("failed to marshal question data: %v", err)
	}

	event := InterruptEvent{
		Type:      "ask_user_question",
		Data:      data,
		AgentName: "test_agent",
		SessionID: uuid.New(),
	}

	handled, err := handler.Handle(ctx, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !handled {
		t.Error("expected handler to return handled=true")
	}
}

func TestAskUserQuestionHandler_Handle_InvalidData(t *testing.T) {
	t.Parallel()

	handler := &AskUserQuestionHandler{}
	ctx := context.Background()

	// Invalid JSON
	event := InterruptEvent{
		Type:      "ask_user_question",
		Data:      []byte("{invalid json}"),
		AgentName: "test_agent",
		SessionID: uuid.New(),
	}

	_, err := handler.Handle(ctx, event)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestInterruptEvent_Fields(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	data := json.RawMessage(`{"question": "test"}`)

	event := InterruptEvent{
		Type:      "test_type",
		Data:      data,
		AgentName: "test_agent",
		SessionID: sessionID,
	}

	if event.Type != "test_type" {
		t.Errorf("expected Type='test_type', got %s", event.Type)
	}

	if event.AgentName != "test_agent" {
		t.Errorf("expected AgentName='test_agent', got %s", event.AgentName)
	}

	if event.SessionID != sessionID {
		t.Errorf("expected SessionID=%s, got %s", sessionID, event.SessionID)
	}

	if string(event.Data) != `{"question": "test"}` {
		t.Errorf("expected Data='%s', got %s", `{"question": "test"}`, string(event.Data))
	}
}

// mockInterruptHandler is a test implementation of InterruptHandler
type mockInterruptHandler struct {
	name    string
	handled bool
	err     error
}

func (m *mockInterruptHandler) Handle(ctx context.Context, event InterruptEvent) (bool, error) {
	m.handled = true
	return m.err == nil, m.err
}

func TestInterruptHandlerRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	registry := NewInterruptHandlerRegistry()
	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(n int) {
			handler := &mockInterruptHandler{name: "concurrent"}
			registry.Register("concurrent_event", handler)
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = registry.Get("concurrent_event")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify registry is still functional
	handler := &mockInterruptHandler{name: "final"}
	registry.Register("final_event", handler)

	retrieved, ok := registry.Get("final_event")
	if !ok {
		t.Error("expected handler to be registered after concurrent access")
	}

	if retrieved != handler {
		t.Error("expected retrieved handler to match registered handler")
	}
}

func TestInterruptHandlerRegistry_CustomHandler(t *testing.T) {
	t.Parallel()

	registry := NewInterruptHandlerRegistry()
	customHandler := &mockInterruptHandler{err: errors.New("custom error")}

	// Override built-in handler
	registry.Register("ask_user_question", customHandler)

	handler, ok := registry.Get("ask_user_question")
	if !ok {
		t.Fatal("expected handler to be registered")
	}

	if handler != customHandler {
		t.Error("expected custom handler to override built-in handler")
	}

	// Test custom handler behavior
	ctx := context.Background()
	event := InterruptEvent{
		Type:      "ask_user_question",
		Data:      json.RawMessage(`{}`),
		AgentName: "test",
		SessionID: uuid.New(),
	}

	handled, err := handler.Handle(ctx, event)
	if err == nil {
		t.Error("expected custom handler error to be returned")
	}

	if handled {
		t.Error("expected handled=false when error occurs")
	}
}
