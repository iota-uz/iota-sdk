package langfuse

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/henomis/langfuse-go/model"
)

func TestMockClient_Generation(t *testing.T) {
	t.Parallel()

	t.Run("successful generation call", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()
		gen := &model.Generation{
			TraceID: "trace-123",
			Name:    "test-generation",
		}

		result, err := mock.Generation(gen, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if result.ID != "gen-mock-id" {
			t.Errorf("expected ID to be set, got %s", result.ID)
		}

		calls := mock.GetGenerationCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}

		if calls[0].Generation.TraceID != "trace-123" {
			t.Errorf("expected traceID trace-123, got %s", calls[0].Generation.TraceID)
		}
	})

	t.Run("generation call with error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("generation failed")
		mock := NewMockClient().WithGenerationError(expectedErr)

		gen := &model.Generation{TraceID: "trace-123"}
		result, err := mock.Generation(gen, nil)

		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}

		if result != nil {
			t.Errorf("expected nil result on error, got %+v", result)
		}
	})

	t.Run("generation call with custom response", func(t *testing.T) {
		t.Parallel()

		customResp := &model.Generation{
			ID:      "custom-id",
			TraceID: "trace-123",
			Name:    "custom",
		}
		mock := NewMockClient().WithGenerationResponse(customResp)

		gen := &model.Generation{TraceID: "trace-123"}
		result, err := mock.Generation(gen, nil)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if result.ID != "custom-id" {
			t.Errorf("expected custom ID, got %s", result.ID)
		}
	})
}

func TestMockClient_Span(t *testing.T) {
	t.Parallel()

	t.Run("successful span call", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()
		span := &model.Span{
			TraceID: "trace-123",
			Name:    "test-span",
		}

		result, err := mock.Span(span, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if result.ID != "span-mock-id" {
			t.Errorf("expected ID to be set, got %s", result.ID)
		}

		calls := mock.GetSpanCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
	})

	t.Run("span call with parent ID", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()
		span := &model.Span{TraceID: "trace-123"}
		parentID := "parent-123"

		_, err := mock.Span(span, &parentID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		calls := mock.GetSpanCalls()
		if calls[0].ParentID == nil || *calls[0].ParentID != parentID {
			t.Errorf("expected parent ID %s, got %v", parentID, calls[0].ParentID)
		}
	})
}

func TestMockClient_Event(t *testing.T) {
	t.Parallel()

	t.Run("successful event call", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()
		event := &model.Event{
			TraceID: "trace-123",
			Name:    "test-event",
			Level:   model.ObservationLevelDefault,
		}

		result, err := mock.Event(event, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if result.ID != "event-mock-id" {
			t.Errorf("expected ID to be set, got %s", result.ID)
		}
	})
}

func TestMockClient_Trace(t *testing.T) {
	t.Parallel()

	t.Run("successful trace call", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()
		trace := &model.Trace{
			ID:   "trace-123",
			Name: "test-trace",
		}

		result, err := mock.Trace(trace)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if result.ID != "trace-123" {
			t.Errorf("expected ID trace-123, got %s", result.ID)
		}

		calls := mock.GetTraceCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
	})

	t.Run("trace call with error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("trace failed")
		mock := NewMockClient().WithTraceError(expectedErr)

		trace := &model.Trace{ID: "trace-123"}
		result, err := mock.Trace(trace)

		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}

		if result != nil {
			t.Errorf("expected nil result on error, got %+v", result)
		}
	})
}

func TestMockClient_Flush(t *testing.T) {
	t.Parallel()

	t.Run("flush call tracking", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()
		ctx := context.Background()

		mock.Flush(ctx)

		calls := mock.GetFlushCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 flush call, got %d", len(calls))
		}
	})
}

func TestMockClient_Reset(t *testing.T) {
	t.Parallel()

	t.Run("reset clears all state", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()

		// Make some calls
		_, _ = mock.Generation(&model.Generation{}, nil)
		_, _ = mock.Span(&model.Span{}, nil)
		_, _ = mock.Event(&model.Event{}, nil)
		_, _ = mock.Trace(&model.Trace{})
		mock.Flush(context.Background())

		// Set some errors
		mock.WithGenerationError(errors.New("test"))
		mock.WithSpanError(errors.New("test"))

		// Reset
		mock.Reset()

		// Verify all cleared
		if mock.GenerationCallCount() != 0 {
			t.Errorf("expected 0 generation calls after reset, got %d", mock.GenerationCallCount())
		}
		if mock.SpanCallCount() != 0 {
			t.Errorf("expected 0 span calls after reset, got %d", mock.SpanCallCount())
		}
		if mock.EventCallCount() != 0 {
			t.Errorf("expected 0 event calls after reset, got %d", mock.EventCallCount())
		}
		if mock.TraceCallCount() != 0 {
			t.Errorf("expected 0 trace calls after reset, got %d", mock.TraceCallCount())
		}
		if mock.FlushCallCount() != 0 {
			t.Errorf("expected 0 flush calls after reset, got %d", mock.FlushCallCount())
		}

		// Verify errors cleared
		if mock.GenerationError != nil {
			t.Errorf("expected generation error to be cleared")
		}
		if mock.SpanError != nil {
			t.Errorf("expected span error to be cleared")
		}
	})
}

func TestMockClient_CallCounts(t *testing.T) {
	t.Parallel()

	t.Run("call count methods", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()

		// Make multiple calls
		_, _ = mock.Generation(&model.Generation{}, nil)
		_, _ = mock.Generation(&model.Generation{}, nil)
		_, _ = mock.Span(&model.Span{}, nil)
		_, _ = mock.Event(&model.Event{}, nil)
		_, _ = mock.Event(&model.Event{}, nil)
		_, _ = mock.Event(&model.Event{}, nil)
		_, _ = mock.Trace(&model.Trace{})

		if mock.GenerationCallCount() != 2 {
			t.Errorf("expected 2 generation calls, got %d", mock.GenerationCallCount())
		}
		if mock.SpanCallCount() != 1 {
			t.Errorf("expected 1 span call, got %d", mock.SpanCallCount())
		}
		if mock.EventCallCount() != 3 {
			t.Errorf("expected 3 event calls, got %d", mock.EventCallCount())
		}
		if mock.TraceCallCount() != 1 {
			t.Errorf("expected 1 trace call, got %d", mock.TraceCallCount())
		}
	})
}

func TestMockClient_BuilderPattern(t *testing.T) {
	t.Parallel()

	t.Run("chained builder methods", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient().
			WithGenerationError(errors.New("gen error")).
			WithSpanError(errors.New("span error")).
			WithEventError(errors.New("event error"))

		// Verify all errors set
		_, err := mock.Generation(&model.Generation{}, nil)
		if err == nil || err.Error() != "gen error" {
			t.Errorf("expected gen error, got %v", err)
		}

		_, err = mock.Span(&model.Span{}, nil)
		if err == nil || err.Error() != "span error" {
			t.Errorf("expected span error, got %v", err)
		}

		_, err = mock.Event(&model.Event{}, nil)
		if err == nil || err.Error() != "event error" {
			t.Errorf("expected event error, got %v", err)
		}
	})
}

func TestMockClient_ThreadSafety(t *testing.T) {
	t.Parallel()

	t.Run("concurrent calls are safe", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()
		done := make(chan bool)

		// Start multiple goroutines
		for i := 0; i < 10; i++ {
			go func() {
				_, _ = mock.Generation(&model.Generation{}, nil)
				_, _ = mock.Span(&model.Span{}, nil)
				_, _ = mock.Event(&model.Event{}, nil)
				done <- true
			}()
		}

		// Wait for all to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify counts
		if mock.GenerationCallCount() != 10 {
			t.Errorf("expected 10 generation calls, got %d", mock.GenerationCallCount())
		}
		if mock.SpanCallCount() != 10 {
			t.Errorf("expected 10 span calls, got %d", mock.SpanCallCount())
		}
		if mock.EventCallCount() != 10 {
			t.Errorf("expected 10 event calls, got %d", mock.EventCallCount())
		}
	})
}

func TestMockClient_EndMethods(t *testing.T) {
	t.Parallel()

	t.Run("generation end tracking", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()
		now := time.Now()
		gen := &model.Generation{
			ID:      "gen-123",
			EndTime: &now,
		}

		result, err := mock.GenerationEnd(gen)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if result.ID != "gen-123" {
			t.Errorf("expected ID gen-123, got %s", result.ID)
		}

		calls := mock.GetGenerationEndCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
	})

	t.Run("span end tracking", func(t *testing.T) {
		t.Parallel()

		mock := NewMockClient()
		now := time.Now()
		span := &model.Span{
			ID:      "span-123",
			EndTime: &now,
		}

		result, err := mock.SpanEnd(span)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if result.ID != "span-123" {
			t.Errorf("expected ID span-123, got %s", result.ID)
		}

		calls := mock.GetSpanEndCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
	})
}
