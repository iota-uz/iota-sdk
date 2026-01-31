package agents

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// InterruptEvent represents a human-in-the-loop (HITL) interrupt event.
// These events pause agent execution and require external intervention
// (typically user input) before the agent can continue.
//
// Example flow:
//  1. Agent calls ask_user_question tool
//  2. Executor yields InterruptEvent with Type="ask_user_question"
//  3. UI displays question to user
//  4. User provides answer
//  5. Executor resumes with answer injected as tool result
type InterruptEvent struct {
	// Type identifies the interrupt type (e.g., "ask_user_question").
	// This determines which handler should process the interrupt.
	Type string

	// Data contains interrupt-specific payload (JSON-encoded).
	// For ask_user_question: {"question": "What is your favorite color?"}
	// For custom interrupts: application-specific data
	Data json.RawMessage

	// AgentName is the name of the agent that triggered this interrupt.
	// Used for observability and routing in multi-agent scenarios.
	AgentName string

	// SessionID is the chat session this interrupt belongs to.
	// Used to correlate interrupts with their originating conversation.
	SessionID uuid.UUID
}

// InterruptHandler processes a specific type of interrupt event.
// Handlers are registered in the InterruptHandlerRegistry by type.
//
// Example handler for ask_user_question:
//
//	type UserQuestionHandler struct {
//	    ui UserInterface
//	}
//
//	func (h *UserQuestionHandler) Handle(ctx context.Context, event InterruptEvent) (bool, error) {
//	    var data struct {
//	        Question string `json:"question"`
//	    }
//	    if err := json.Unmarshal(event.Data, &data); err != nil {
//	        return false, fmt.Errorf("invalid question data: %w", err)
//	    }
//
//	    answer, err := h.ui.AskUser(ctx, data.Question)
//	    if err != nil {
//	        return false, err
//	    }
//
//	    // Store answer in context or somewhere executor can retrieve it
//	    return true, nil // handled successfully
//	}
type InterruptHandler interface {
	// Handle processes the interrupt event.
	// Returns (true, nil) if handled successfully.
	// Returns (false, nil) if this handler cannot handle this event type.
	// Returns (false, error) if an error occurred during handling.
	Handle(ctx context.Context, event InterruptEvent) (bool, error)
}

// InterruptHandlerRegistry manages registered interrupt handlers.
// It provides thread-safe registration and lookup of handlers by type.
//
// Example usage:
//
//	registry := NewInterruptHandlerRegistry()
//	registry.Register("ask_user_question", &AskUserQuestionHandler{ui: myUI})
//
//	// Later, when interrupt occurs:
//	handler, ok := registry.Get("ask_user_question")
//	if ok {
//	    handled, err := handler.Handle(ctx, event)
//	}
type InterruptHandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]InterruptHandler
}

// NewInterruptHandlerRegistry creates a new registry with the built-in
// AskUserQuestionHandler pre-registered.
//
// To customize the ask_user_question handler, call Register again with
// your custom implementation.
func NewInterruptHandlerRegistry() *InterruptHandlerRegistry {
	registry := &InterruptHandlerRegistry{
		handlers: make(map[string]InterruptHandler),
	}

	// Register built-in handler for ask_user_question
	registry.Register("ask_user_question", &AskUserQuestionHandler{})

	return registry
}

// Register adds or replaces a handler for the given interrupt type.
// This is thread-safe and can be called concurrently.
//
// If a handler already exists for this type, it will be replaced.
func (r *InterruptHandlerRegistry) Register(eventType string, handler InterruptHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[eventType] = handler
}

// Get retrieves a handler for the given interrupt type.
// Returns (handler, true) if found, (nil, false) if not registered.
//
// This is thread-safe and can be called concurrently with Register.
func (r *InterruptHandlerRegistry) Get(eventType string) (InterruptHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[eventType]
	return handler, ok
}

// All returns a copy of all registered handlers.
// The returned map is a snapshot and safe to iterate without locking.
//
// This is useful for observability and debugging.
func (r *InterruptHandlerRegistry) All() map[string]InterruptHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid exposing internal map
	result := make(map[string]InterruptHandler, len(r.handlers))
	for k, v := range r.handlers {
		result[k] = v
	}
	return result
}

// AskUserQuestionHandler is the built-in handler for ask_user_question interrupts.
// This is a no-op handler that always returns (true, nil).
//
// In practice, you should replace this with a custom handler that integrates
// with your UI layer to actually prompt the user for input.
//
// Example custom handler:
//
//	type CustomQuestionHandler struct {
//	    service UserInteractionService
//	}
//
//	func (h *CustomQuestionHandler) Handle(ctx context.Context, event InterruptEvent) (bool, error) {
//	    var data struct {
//	        Question string `json:"question"`
//	    }
//	    if err := json.Unmarshal(event.Data, &data); err != nil {
//	        return false, fmt.Errorf("parse question: %w", err)
//	    }
//
//	    // Send question to UI via WebSocket/SSE/etc
//	    answer, err := h.service.AskUserAndWait(ctx, event.SessionID, data.Question)
//	    if err != nil {
//	        return false, err
//	    }
//
//	    // Store answer so executor can inject it as tool result
//	    StoreAnswer(ctx, event.SessionID, answer)
//	    return true, nil
//	}
//
// To use custom handler:
//
//	registry.Register("ask_user_question", &CustomQuestionHandler{service: svc})
type AskUserQuestionHandler struct{}

const op serrors.Op = "AskUserQuestionHandler.Handle"

// Handle processes ask_user_question interrupts.
// This default implementation is a no-op placeholder.
//
// Override this by registering a custom handler in the registry:
//
//	registry.Register("ask_user_question", myCustomHandler)
func (h *AskUserQuestionHandler) Handle(ctx context.Context, event InterruptEvent) (bool, error) {
	// Default implementation: just log that we received the interrupt
	// In production, replace this with actual UI interaction logic

	var data struct {
		Question string `json:"question"`
	}
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return false, serrors.E(op, err)
	}

	// TODO: Implement actual user interaction
	// For now, this is a no-op that returns success
	// The executor will handle the interrupt event separately

	return true, nil
}
