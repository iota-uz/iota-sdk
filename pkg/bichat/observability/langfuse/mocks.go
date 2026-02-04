package langfuse

import (
	"context"
	"sync"

	"github.com/henomis/langfuse-go"
	"github.com/henomis/langfuse-go/model"
)

// LangfuseClient defines the interface for Langfuse client operations.
// This interface mirrors the methods used by LangfuseProvider from the henomis/langfuse-go SDK.
type LangfuseClient interface {
	// Generation creates a new generation observation
	Generation(gen *model.Generation, parentID *string) (*model.Generation, error)

	// GenerationEnd marks a generation as complete
	GenerationEnd(gen *model.Generation) (*model.Generation, error)

	// Span creates a new span observation
	Span(span *model.Span, parentID *string) (*model.Span, error)

	// SpanEnd marks a span as complete
	SpanEnd(span *model.Span) (*model.Span, error)

	// Event creates a new event observation
	Event(event *model.Event, parentID *string) (*model.Event, error)

	// Trace creates a new trace
	Trace(trace *model.Trace) (*model.Trace, error)

	// Flush forces all pending observations to be sent
	Flush(ctx context.Context)
}

// Ensure *langfuse.Langfuse implements LangfuseClient
var _ LangfuseClient = (*langfuse.Langfuse)(nil)

// MockLangfuseClient is a mock implementation of LangfuseClient for testing.
// It provides thread-safe call tracking, error injection, and response customization.
type MockLangfuseClient struct {
	mu sync.RWMutex

	// Call tracking - stores all invocations for assertion
	GenerationCalls    []GenerationCall
	GenerationEndCalls []GenerationEndCall
	SpanCalls          []SpanCall
	SpanEndCalls       []SpanEndCall
	EventCalls         []EventCall
	TraceCalls         []TraceCall
	FlushCalls         []FlushCall

	// Error injection - return errors for specific operations
	GenerationError    error
	GenerationEndError error
	SpanError          error
	SpanEndError       error
	EventError         error
	TraceError         error

	// Response customization - override default responses
	GenerationResponse    *model.Generation
	GenerationEndResponse *model.Generation
	SpanResponse          *model.Span
	SpanEndResponse       *model.Span
	EventResponse         *model.Event
	TraceResponse         *model.Trace
}

// Call structs to track method invocations with full context

// GenerationCall records a call to Generation method
type GenerationCall struct {
	Generation *model.Generation
	ParentID   *string
}

// GenerationEndCall records a call to GenerationEnd method
type GenerationEndCall struct {
	Generation *model.Generation
}

// SpanCall records a call to Span method
type SpanCall struct {
	Span     *model.Span
	ParentID *string
}

// SpanEndCall records a call to SpanEnd method
type SpanEndCall struct {
	Span *model.Span
}

// EventCall records a call to Event method
type EventCall struct {
	Event    *model.Event
	ParentID *string
}

// TraceCall records a call to Trace method
type TraceCall struct {
	Trace *model.Trace
}

// FlushCall records a call to Flush method
type FlushCall struct {
	Ctx context.Context
}

// NewMockClient creates a new mock client with default behavior.
// All methods return successful responses unless configured otherwise.
func NewMockClient() *MockLangfuseClient {
	return &MockLangfuseClient{
		GenerationCalls:    make([]GenerationCall, 0),
		GenerationEndCalls: make([]GenerationEndCall, 0),
		SpanCalls:          make([]SpanCall, 0),
		SpanEndCalls:       make([]SpanEndCall, 0),
		EventCalls:         make([]EventCall, 0),
		TraceCalls:         make([]TraceCall, 0),
		FlushCalls:         make([]FlushCall, 0),
	}
}

// LangfuseClient interface implementation

// Generation creates a generation observation.
// Records call details and returns configured response or error.
func (m *MockLangfuseClient) Generation(gen *model.Generation, parentID *string) (*model.Generation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GenerationCalls = append(m.GenerationCalls, GenerationCall{
		Generation: gen,
		ParentID:   parentID,
	})

	if m.GenerationError != nil {
		return nil, m.GenerationError
	}

	if m.GenerationResponse != nil {
		return m.GenerationResponse, nil
	}

	// Default: return copy with ID if missing
	result := *gen
	if result.ID == "" {
		result.ID = "gen-mock-id"
	}
	return &result, nil
}

// GenerationEnd marks a generation as complete.
// Records call details and returns configured response or error.
func (m *MockLangfuseClient) GenerationEnd(gen *model.Generation) (*model.Generation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GenerationEndCalls = append(m.GenerationEndCalls, GenerationEndCall{
		Generation: gen,
	})

	if m.GenerationEndError != nil {
		return nil, m.GenerationEndError
	}

	if m.GenerationEndResponse != nil {
		return m.GenerationEndResponse, nil
	}

	// Default: return the generation unchanged
	return gen, nil
}

// Span creates a span observation.
// Records call details and returns configured response or error.
func (m *MockLangfuseClient) Span(span *model.Span, parentID *string) (*model.Span, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SpanCalls = append(m.SpanCalls, SpanCall{
		Span:     span,
		ParentID: parentID,
	})

	if m.SpanError != nil {
		return nil, m.SpanError
	}

	if m.SpanResponse != nil {
		return m.SpanResponse, nil
	}

	// Default: return copy with ID if missing
	result := *span
	if result.ID == "" {
		result.ID = "span-mock-id"
	}
	return &result, nil
}

// SpanEnd marks a span as complete.
// Records call details and returns configured response or error.
func (m *MockLangfuseClient) SpanEnd(span *model.Span) (*model.Span, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SpanEndCalls = append(m.SpanEndCalls, SpanEndCall{
		Span: span,
	})

	if m.SpanEndError != nil {
		return nil, m.SpanEndError
	}

	if m.SpanEndResponse != nil {
		return m.SpanEndResponse, nil
	}

	// Default: return the span unchanged
	return span, nil
}

// Event creates an event observation.
// Records call details and returns configured response or error.
func (m *MockLangfuseClient) Event(event *model.Event, parentID *string) (*model.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.EventCalls = append(m.EventCalls, EventCall{
		Event:    event,
		ParentID: parentID,
	})

	if m.EventError != nil {
		return nil, m.EventError
	}

	if m.EventResponse != nil {
		return m.EventResponse, nil
	}

	// Default: return copy with ID if missing
	result := *event
	if result.ID == "" {
		result.ID = "event-mock-id"
	}
	return &result, nil
}

// Trace creates a trace.
// Records call details and returns configured response or error.
func (m *MockLangfuseClient) Trace(trace *model.Trace) (*model.Trace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TraceCalls = append(m.TraceCalls, TraceCall{
		Trace: trace,
	})

	if m.TraceError != nil {
		return nil, m.TraceError
	}

	if m.TraceResponse != nil {
		return m.TraceResponse, nil
	}

	// Default: return copy with ID if missing
	result := *trace
	if result.ID == "" {
		result.ID = "trace-mock-id"
	}
	return &result, nil
}

// Flush forces pending observations to be sent.
// Records call details (non-blocking, no error return).
func (m *MockLangfuseClient) Flush(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.FlushCalls = append(m.FlushCalls, FlushCall{
		Ctx: ctx,
	})
}

// Helper methods for test assertions

// GetGenerationCalls returns a copy of all Generation calls.
// Thread-safe for assertions in tests.
func (m *MockLangfuseClient) GetGenerationCalls() []GenerationCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]GenerationCall{}, m.GenerationCalls...)
}

// GetGenerationEndCalls returns a copy of all GenerationEnd calls.
// Thread-safe for assertions in tests.
func (m *MockLangfuseClient) GetGenerationEndCalls() []GenerationEndCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]GenerationEndCall{}, m.GenerationEndCalls...)
}

// GetSpanCalls returns a copy of all Span calls.
// Thread-safe for assertions in tests.
func (m *MockLangfuseClient) GetSpanCalls() []SpanCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]SpanCall{}, m.SpanCalls...)
}

// GetSpanEndCalls returns a copy of all SpanEnd calls.
// Thread-safe for assertions in tests.
func (m *MockLangfuseClient) GetSpanEndCalls() []SpanEndCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]SpanEndCall{}, m.SpanEndCalls...)
}

// GetEventCalls returns a copy of all Event calls.
// Thread-safe for assertions in tests.
func (m *MockLangfuseClient) GetEventCalls() []EventCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]EventCall{}, m.EventCalls...)
}

// GetTraceCalls returns a copy of all Trace calls.
// Thread-safe for assertions in tests.
func (m *MockLangfuseClient) GetTraceCalls() []TraceCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]TraceCall{}, m.TraceCalls...)
}

// GetFlushCalls returns a copy of all Flush calls.
// Thread-safe for assertions in tests.
func (m *MockLangfuseClient) GetFlushCalls() []FlushCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]FlushCall{}, m.FlushCalls...)
}

// Reset clears all call tracking and error states.
// Useful for reusing mock between test cases.
func (m *MockLangfuseClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GenerationCalls = make([]GenerationCall, 0)
	m.GenerationEndCalls = make([]GenerationEndCall, 0)
	m.SpanCalls = make([]SpanCall, 0)
	m.SpanEndCalls = make([]SpanEndCall, 0)
	m.EventCalls = make([]EventCall, 0)
	m.TraceCalls = make([]TraceCall, 0)
	m.FlushCalls = make([]FlushCall, 0)

	m.GenerationError = nil
	m.GenerationEndError = nil
	m.SpanError = nil
	m.SpanEndError = nil
	m.EventError = nil
	m.TraceError = nil

	m.GenerationResponse = nil
	m.GenerationEndResponse = nil
	m.SpanResponse = nil
	m.SpanEndResponse = nil
	m.EventResponse = nil
	m.TraceResponse = nil
}

// Builder methods for test setup

// WithGenerationError configures the mock to return an error on Generation calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithGenerationError(err error) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GenerationError = err
	return m
}

// WithGenerationEndError configures the mock to return an error on GenerationEnd calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithGenerationEndError(err error) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GenerationEndError = err
	return m
}

// WithSpanError configures the mock to return an error on Span calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithSpanError(err error) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SpanError = err
	return m
}

// WithSpanEndError configures the mock to return an error on SpanEnd calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithSpanEndError(err error) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SpanEndError = err
	return m
}

// WithEventError configures the mock to return an error on Event calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithEventError(err error) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EventError = err
	return m
}

// WithTraceError configures the mock to return an error on Trace calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithTraceError(err error) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TraceError = err
	return m
}

// WithGenerationResponse configures the mock to return a custom response for Generation calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithGenerationResponse(resp *model.Generation) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GenerationResponse = resp
	return m
}

// WithGenerationEndResponse configures the mock to return a custom response for GenerationEnd calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithGenerationEndResponse(resp *model.Generation) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GenerationEndResponse = resp
	return m
}

// WithSpanResponse configures the mock to return a custom response for Span calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithSpanResponse(resp *model.Span) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SpanResponse = resp
	return m
}

// WithSpanEndResponse configures the mock to return a custom response for SpanEnd calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithSpanEndResponse(resp *model.Span) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SpanEndResponse = resp
	return m
}

// WithEventResponse configures the mock to return a custom response for Event calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithEventResponse(resp *model.Event) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EventResponse = resp
	return m
}

// WithTraceResponse configures the mock to return a custom response for Trace calls.
// Returns self for method chaining.
func (m *MockLangfuseClient) WithTraceResponse(resp *model.Trace) *MockLangfuseClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TraceResponse = resp
	return m
}

// CallCount methods for convenient assertions

// GenerationCallCount returns the total number of Generation calls.
func (m *MockLangfuseClient) GenerationCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.GenerationCalls)
}

// GenerationEndCallCount returns the total number of GenerationEnd calls.
func (m *MockLangfuseClient) GenerationEndCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.GenerationEndCalls)
}

// SpanCallCount returns the total number of Span calls.
func (m *MockLangfuseClient) SpanCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.SpanCalls)
}

// SpanEndCallCount returns the total number of SpanEnd calls.
func (m *MockLangfuseClient) SpanEndCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.SpanEndCalls)
}

// EventCallCount returns the total number of Event calls.
func (m *MockLangfuseClient) EventCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.EventCalls)
}

// TraceCallCount returns the total number of Trace calls.
func (m *MockLangfuseClient) TraceCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.TraceCalls)
}

// FlushCallCount returns the total number of Flush calls.
func (m *MockLangfuseClient) FlushCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.FlushCalls)
}
