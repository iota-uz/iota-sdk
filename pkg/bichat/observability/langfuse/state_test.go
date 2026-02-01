package langfuse

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestState_ConcurrentAccess verifies thread-safety under concurrent access.
func TestState_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	sessionCount := 100
	var wg sync.WaitGroup

	// Generate concurrent requests
	for i := 0; i < sessionCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			obs := buildCompleteObservation()
			obs.SessionID = uuid.New()
			obs.ID = fmt.Sprintf("gen-%d", id)

			err := provider.RecordGeneration(ctx, obs)
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// All generations should have been recorded
	// Note: Due to race conditions with trace creation, we may have more trace calls than generations
	genCalls := mock.GetGenerationCalls()
	assert.Len(t, genCalls, sessionCount)

	// Verify no panics and all IDs are unique
	ids := make(map[string]bool)
	for _, call := range genCalls {
		id := call.Generation.ID
		assert.False(t, ids[id], "duplicate generation ID: %s", id)
		ids[id] = true
	}
}

// TestState_SessionIsolation verifies that sessions don't interfere with each other.
func TestState_SessionIsolation(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()

	// Session 1
	session1 := uuid.New()
	obs1 := buildCompleteObservation()
	obs1.SessionID = session1
	obs1.ID = "gen-1"

	err := provider.RecordGeneration(ctx, obs1)
	require.NoError(t, err)

	// Session 2
	session2 := uuid.New()
	obs2 := buildCompleteObservation()
	obs2.SessionID = session2
	obs2.ID = "gen-2"

	err = provider.RecordGeneration(ctx, obs2)
	require.NoError(t, err)

	// Each session should have separate state
	traceID1 := provider.state.getTraceID(session1.String())
	traceID2 := provider.state.getTraceID(session2.String())

	assert.NotEmpty(t, traceID1)
	assert.NotEmpty(t, traceID2)
	assert.NotEqual(t, traceID1, traceID2)

	// Generation IDs should be tracked separately
	genID1 := provider.state.getGenerationID("gen-1")
	genID2 := provider.state.getGenerationID("gen-2")

	assert.Equal(t, "gen-1", genID1)
	assert.Equal(t, "gen-2", genID2)
}

// TestState_StateCleanup verifies that state can be cleared.
func TestState_StateCleanup(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()

	// Record some observations
	obs := buildCompleteObservation()
	err := provider.RecordGeneration(ctx, obs)
	require.NoError(t, err)

	spanObs := buildSpanObservation()
	err = provider.RecordSpan(ctx, spanObs)
	require.NoError(t, err)

	// Verify state exists
	traceID := provider.state.getTraceID(obs.SessionID.String())
	assert.NotEmpty(t, traceID)

	genID := provider.state.getGenerationID(obs.ID)
	assert.NotEmpty(t, genID)

	spanID := provider.state.getSpanID(spanObs.ID)
	assert.NotEmpty(t, spanID)

	// Clear state
	provider.state.clear()

	// State should be empty
	traceID = provider.state.getTraceID(obs.SessionID.String())
	assert.Empty(t, traceID)

	genID = provider.state.getGenerationID(obs.ID)
	assert.Empty(t, genID)

	spanID = provider.state.getSpanID(spanObs.ID)
	assert.Empty(t, spanID)
}

// TestState_MultipleGenerations verifies handling of multiple generations in same session.
func TestState_MultipleGenerations(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	sessionID := uuid.New()

	// Record 3 generations in same session
	for i := 0; i < 3; i++ {
		obs := buildCompleteObservation()
		obs.SessionID = sessionID
		obs.ID = fmt.Sprintf("gen-%d", i)

		err := provider.RecordGeneration(ctx, obs)
		require.NoError(t, err)
	}

	// All generations should be recorded
	genCalls := mock.GetGenerationCalls()
	assert.Len(t, genCalls, 3)

	// All should share same trace ID
	for _, call := range genCalls {
		assert.Equal(t, sessionID.String(), call.Generation.TraceID)
	}

	// Trace should only be created once
	traceCalls := mock.GetTraceCalls()
	assert.Len(t, traceCalls, 1)
}

// TestState_StateAfterFlush verifies state persists after flush.
func TestState_StateAfterFlush(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()

	// Record observation
	obs := buildCompleteObservation()
	err := provider.RecordGeneration(ctx, obs)
	require.NoError(t, err)

	// Verify state exists
	traceID := provider.state.getTraceID(obs.SessionID.String())
	genID := provider.state.getGenerationID(obs.ID)
	assert.NotEmpty(t, traceID)
	assert.NotEmpty(t, genID)

	// Flush
	err = provider.Flush(ctx)
	require.NoError(t, err)

	// State should persist after flush
	traceIDAfter := provider.state.getTraceID(obs.SessionID.String())
	genIDAfter := provider.state.getGenerationID(obs.ID)

	assert.Equal(t, traceID, traceIDAfter)
	assert.Equal(t, genID, genIDAfter)

	// Flush should have been called
	assert.Equal(t, 1, mock.FlushCallCount())
}

// TestState_StateAfterShutdown verifies state is cleared after shutdown.
func TestState_StateAfterShutdown(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()

	// Record some observations
	obs := buildCompleteObservation()
	err := provider.RecordGeneration(ctx, obs)
	require.NoError(t, err)

	// Verify state exists
	traceID := provider.state.getTraceID(obs.SessionID.String())
	genID := provider.state.getGenerationID(obs.ID)
	assert.NotEmpty(t, traceID)
	assert.NotEmpty(t, genID)

	// Shutdown
	err = provider.Shutdown(ctx)
	require.NoError(t, err)

	// State should be cleared
	traceIDAfter := provider.state.getTraceID(obs.SessionID.String())
	genIDAfter := provider.state.getGenerationID(obs.ID)

	assert.Empty(t, traceIDAfter)
	assert.Empty(t, genIDAfter)

	// Flush should have been called during shutdown
	assert.Equal(t, 1, mock.FlushCallCount())
}

// TestState_SetAndGetTraceID verifies trace ID storage and retrieval.
func TestState_SetAndGetTraceID(t *testing.T) {
	t.Parallel()

	s := newState()

	// Initially empty
	traceID := s.getTraceID("session-123")
	assert.Empty(t, traceID)

	// Set trace ID
	s.setTraceID("session-123", "trace-abc")

	// Retrieve trace ID
	traceID = s.getTraceID("session-123")
	assert.Equal(t, "trace-abc", traceID)

	// Different session should have different ID
	traceID2 := s.getTraceID("session-456")
	assert.Empty(t, traceID2)
}

// TestState_SetAndGetGenerationID verifies generation ID storage and retrieval.
func TestState_SetAndGetGenerationID(t *testing.T) {
	t.Parallel()

	s := newState()

	// Initially empty
	genID := s.getGenerationID("gen-123")
	assert.Empty(t, genID)

	// Set generation ID
	s.setGenerationID("gen-123", "langfuse-gen-abc")

	// Retrieve generation ID
	genID = s.getGenerationID("gen-123")
	assert.Equal(t, "langfuse-gen-abc", genID)

	// Different generation should have different ID
	genID2 := s.getGenerationID("gen-456")
	assert.Empty(t, genID2)
}

// TestState_SetAndGetSpanID verifies span ID storage and retrieval.
func TestState_SetAndGetSpanID(t *testing.T) {
	t.Parallel()

	s := newState()

	// Initially empty
	spanID := s.getSpanID("span-123")
	assert.Empty(t, spanID)

	// Set span ID
	s.setSpanID("span-123", "langfuse-span-abc")

	// Retrieve span ID
	spanID = s.getSpanID("span-123")
	assert.Equal(t, "langfuse-span-abc", spanID)

	// Different span should have different ID
	spanID2 := s.getSpanID("span-456")
	assert.Empty(t, spanID2)
}

// TestState_ConcurrentReadWrite verifies thread-safety of state operations.
func TestState_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	s := newState()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%d", id)
			traceID := fmt.Sprintf("trace-%d", id)
			s.setTraceID(sessionID, traceID)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%d", id)
			_ = s.getTraceID(sessionID)
		}(i)
	}

	wg.Wait()

	// All writes should have succeeded
	for i := 0; i < 50; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		traceID := s.getTraceID(sessionID)
		assert.Equal(t, fmt.Sprintf("trace-%d", i), traceID)
	}
}

// TestState_ConcurrentClear verifies thread-safety of clear operation.
func TestState_ConcurrentClear(t *testing.T) {
	t.Parallel()

	s := newState()

	// Pre-populate state
	for i := 0; i < 10; i++ {
		s.setTraceID(fmt.Sprintf("session-%d", i), fmt.Sprintf("trace-%d", i))
		s.setGenerationID(fmt.Sprintf("gen-%d", i), fmt.Sprintf("langfuse-gen-%d", i))
		s.setSpanID(fmt.Sprintf("span-%d", i), fmt.Sprintf("langfuse-span-%d", i))
	}

	var wg sync.WaitGroup

	// Concurrent clear operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.clear()
		}()
	}

	// Concurrent reads during clear
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = s.getTraceID(fmt.Sprintf("session-%d", id))
		}(i)
	}

	wg.Wait()

	// After all clears, state should be empty
	for i := 0; i < 10; i++ {
		traceID := s.getTraceID(fmt.Sprintf("session-%d", i))
		assert.Empty(t, traceID)

		genID := s.getGenerationID(fmt.Sprintf("gen-%d", i))
		assert.Empty(t, genID)

		spanID := s.getSpanID(fmt.Sprintf("span-%d", i))
		assert.Empty(t, spanID)
	}
}

// TestState_OverwriteExisting verifies that setting same ID twice overwrites.
func TestState_OverwriteExisting(t *testing.T) {
	t.Parallel()

	s := newState()

	// Set initial value
	s.setTraceID("session-1", "trace-old")
	traceID := s.getTraceID("session-1")
	assert.Equal(t, "trace-old", traceID)

	// Overwrite
	s.setTraceID("session-1", "trace-new")
	traceID = s.getTraceID("session-1")
	assert.Equal(t, "trace-new", traceID)
}

// TestState_LargeNumberOfSessions verifies state handles many sessions efficiently.
func TestState_LargeNumberOfSessions(t *testing.T) {
	t.Parallel()

	s := newState()
	sessionCount := 10000

	start := time.Now()

	// Store many sessions
	for i := 0; i < sessionCount; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		traceID := fmt.Sprintf("trace-%d", i)
		s.setTraceID(sessionID, traceID)
	}

	elapsed := time.Since(start)

	// Should complete in reasonable time (< 1 second)
	assert.Less(t, elapsed, 1*time.Second, "storing %d sessions took too long: %v", sessionCount, elapsed)

	// Verify all sessions stored
	for i := 0; i < sessionCount; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		traceID := s.getTraceID(sessionID)
		assert.Equal(t, fmt.Sprintf("trace-%d", i), traceID)
	}
}

// TestState_EmptyStringKeys verifies handling of empty string keys.
func TestState_EmptyStringKeys(t *testing.T) {
	t.Parallel()

	s := newState()

	// Set empty key
	s.setTraceID("", "trace-empty")
	traceID := s.getTraceID("")
	assert.Equal(t, "trace-empty", traceID)

	// Should not interfere with non-empty keys
	s.setTraceID("session-1", "trace-1")
	traceID1 := s.getTraceID("session-1")
	assert.Equal(t, "trace-1", traceID1)

	traceIDEmpty := s.getTraceID("")
	assert.Equal(t, "trace-empty", traceIDEmpty)
}

// TestState_MixedOperations verifies state handles mixed concurrent operations.
func TestState_MixedOperations(t *testing.T) {
	t.Parallel()

	s := newState()
	var wg sync.WaitGroup

	operations := 100

	// Mix of all operations
	for i := 0; i < operations; i++ {
		wg.Add(4)

		// Trace operations
		go func(id int) {
			defer wg.Done()
			s.setTraceID(fmt.Sprintf("session-%d", id), fmt.Sprintf("trace-%d", id))
		}(i)

		go func(id int) {
			defer wg.Done()
			_ = s.getTraceID(fmt.Sprintf("session-%d", id))
		}(i)

		// Generation operations
		go func(id int) {
			defer wg.Done()
			s.setGenerationID(fmt.Sprintf("gen-%d", id), fmt.Sprintf("langfuse-gen-%d", id))
		}(i)

		// Span operations
		go func(id int) {
			defer wg.Done()
			s.setSpanID(fmt.Sprintf("span-%d", id), fmt.Sprintf("langfuse-span-%d", id))
		}(i)
	}

	wg.Wait()

	// Verify data integrity
	for i := 0; i < operations; i++ {
		traceID := s.getTraceID(fmt.Sprintf("session-%d", i))
		assert.Equal(t, fmt.Sprintf("trace-%d", i), traceID)

		genID := s.getGenerationID(fmt.Sprintf("gen-%d", i))
		assert.Equal(t, fmt.Sprintf("langfuse-gen-%d", i), genID)

		spanID := s.getSpanID(fmt.Sprintf("span-%d", i))
		assert.Equal(t, fmt.Sprintf("langfuse-span-%d", i), spanID)
	}
}
