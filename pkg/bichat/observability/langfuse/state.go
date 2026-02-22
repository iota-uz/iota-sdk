package langfuse

import (
	"sync"
)

// state tracks active Langfuse observations and their relationships.
// Langfuse uses a hierarchical model: Trace → Generation/Span/Event.
// We need to map BiChat event IDs to Langfuse observation IDs for proper nesting.
type state struct {
	mu sync.RWMutex

	// traceIDs maps BiChat trace IDs to Langfuse trace IDs.
	// IDs are typically identical, but tracked for consistency/extensibility.
	traceIDs map[string]string

	// generationIDs maps BiChat generation IDs to Langfuse generation IDs.
	// Used for linking tool calls and events to parent generations.
	generationIDs map[string]string

	// spanIDs maps BiChat span IDs to Langfuse span IDs.
	// Used for hierarchical span tracking (nested operations).
	spanIDs map[string]string
}

// newState creates a new state tracker.
func newState() *state {
	return &state{
		traceIDs:      make(map[string]string),
		generationIDs: make(map[string]string),
		spanIDs:       make(map[string]string),
	}
}

// setTraceID stores a BiChat trace ID → Langfuse trace ID mapping.
func (s *state) setTraceID(traceID, langfuseTraceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traceIDs[traceID] = langfuseTraceID
}

// getTraceID retrieves the Langfuse trace ID for a BiChat trace ID.
// Returns empty string if not found.
func (s *state) getTraceID(traceID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.traceIDs[traceID]
}

// setGenerationID stores a BiChat generation ID → Langfuse generation ID mapping.
func (s *state) setGenerationID(genID, langfuseGenID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generationIDs[genID] = langfuseGenID
}

// getGenerationID retrieves the Langfuse generation ID for a BiChat generation ID.
// Returns empty string if not found.
func (s *state) getGenerationID(genID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generationIDs[genID]
}

// setSpanID stores a BiChat span ID → Langfuse span ID mapping.
func (s *state) setSpanID(spanID, langfuseSpanID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spanIDs[spanID] = langfuseSpanID
}

// getSpanID retrieves the Langfuse span ID for a BiChat span ID.
// Returns empty string if not found.
func (s *state) getSpanID(spanID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spanIDs[spanID]
}

// clear removes all state (useful for testing or reset scenarios).
func (s *state) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traceIDs = make(map[string]string)
	s.generationIDs = make(map[string]string)
	s.spanIDs = make(map[string]string)
}
