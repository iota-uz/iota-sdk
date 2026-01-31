package agents_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

func TestGenerator_Basic(t *testing.T) {
	t.Parallel()

	// Create a generator that yields numbers 1-5
	gen := agents.NewGenerator(func(yield func(int) bool) error {
		for i := 1; i <= 5; i++ {
			if !yield(i) {
				return nil
			}
		}
		return nil
	})
	defer gen.Close()

	// Consume all values
	expectedValues := []int{1, 2, 3, 4, 5}
	receivedValues := []int{}

	for {
		value, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			break
		}
		receivedValues = append(receivedValues, value)
	}

	// Verify all values were received
	if len(receivedValues) != len(expectedValues) {
		t.Fatalf("Expected %d values, got %d", len(expectedValues), len(receivedValues))
	}

	for i, expected := range expectedValues {
		if receivedValues[i] != expected {
			t.Errorf("Value at index %d: expected %d, got %d", i, expected, receivedValues[i])
		}
	}
}

func TestGenerator_Cancellation(t *testing.T) {
	t.Parallel()

	producerStarted := make(chan struct{})
	producerStopped := make(chan struct{})

	// Create a generator that yields infinitely but respects cancellation
	gen := agents.NewGenerator(func(yield func(int) bool) error {
		close(producerStarted)
		defer close(producerStopped)

		for i := 0; ; i++ {
			if !yield(i) {
				return nil // Consumer stopped
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	// Wait for producer to start
	<-producerStarted

	// Consume a few values
	for i := 0; i < 3; i++ {
		_, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			t.Fatal("Expected more values")
		}
	}

	// Close the generator (signals cancellation)
	gen.Close()

	// Wait for producer to stop (with timeout)
	select {
	case <-producerStopped:
		// Producer stopped as expected
	case <-time.After(1 * time.Second):
		t.Fatal("Producer did not stop after Close()")
	}

	// Subsequent calls to Next() should return closed error
	_, err, hasMore := gen.Next()
	if !errors.Is(err, agents.ErrGeneratorClosed) {
		t.Errorf("Expected ErrGeneratorClosed, got: %v", err)
	}
	if hasMore {
		t.Error("Expected hasMore=false after close")
	}
}

func TestGenerator_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("producer error")

	// Create a generator that yields a few values then returns error
	gen := agents.NewGenerator(func(yield func(int) bool) error {
		yield(1)
		yield(2)
		return expectedErr
	})
	defer gen.Close()

	// Consume first two values
	for i := 1; i <= 2; i++ {
		value, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error on value %d: %v", i, err)
		}
		if !hasMore {
			t.Fatal("Expected more values")
		}
		if value != i {
			t.Errorf("Expected value %d, got %d", i, value)
		}
	}

	// Next call should return the error
	_, err, hasMore := gen.Next()
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got: %v", expectedErr, err)
	}
	if hasMore {
		t.Error("Expected hasMore=false when error occurs")
	}
}

func TestGenerator_Close(t *testing.T) {
	t.Parallel()

	gen := agents.NewGenerator(func(yield func(int) bool) error {
		for i := 0; i < 100; i++ {
			if !yield(i) {
				return nil
			}
		}
		return nil
	})

	// Close can be called multiple times safely
	gen.Close()
	gen.Close()
	gen.Close()

	// After close, Next() should return closed error
	_, err, hasMore := gen.Next()
	if !errors.Is(err, agents.ErrGeneratorClosed) {
		t.Errorf("Expected ErrGeneratorClosed, got: %v", err)
	}
	if hasMore {
		t.Error("Expected hasMore=false after close")
	}
}

func TestGenerator_Collect(t *testing.T) {
	t.Parallel()

	// Create a generator that yields numbers 1-10
	gen := agents.NewGenerator(func(yield func(int) bool) error {
		for i := 1; i <= 10; i++ {
			if !yield(i) {
				return nil
			}
		}
		return nil
	})

	// Collect all values
	values, err := agents.Collect(gen)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify collected values
	if len(values) != 10 {
		t.Fatalf("Expected 10 values, got %d", len(values))
	}

	for i, v := range values {
		if v != i+1 {
			t.Errorf("Value at index %d: expected %d, got %d", i, i+1, v)
		}
	}
}

func TestGenerator_CollectWithError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("collection error")

	// Create a generator that yields a few values then errors
	gen := agents.NewGenerator(func(yield func(int) bool) error {
		yield(1)
		yield(2)
		yield(3)
		return expectedErr
	})

	// Collect should return partial results and the error
	values, err := agents.Collect(gen)
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got: %v", expectedErr, err)
	}

	// Should have collected values before the error
	if len(values) != 3 {
		t.Fatalf("Expected 3 values before error, got %d", len(values))
	}
}

func TestGenerator_EarlyStop(t *testing.T) {
	t.Parallel()

	producerYieldCount := 0

	// Create a generator
	gen := agents.NewGenerator(func(yield func(string) bool) error {
		for i := 0; i < 10; i++ {
			producerYieldCount++
			if !yield("value") {
				return nil // Consumer stopped early
			}
		}
		return nil
	})
	defer gen.Close()

	// Consume only 3 values
	for i := 0; i < 3; i++ {
		_, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			t.Fatal("Expected more values")
		}
	}

	// Close early
	gen.Close()

	// Producer should have yielded at most 4 values (3 consumed + 1 buffered)
	// Give it a moment to complete
	time.Sleep(50 * time.Millisecond)

	if producerYieldCount > 5 {
		t.Errorf("Expected producer to stop early, but it yielded %d values", producerYieldCount)
	}
}

func TestGenerator_EmptyGenerator(t *testing.T) {
	t.Parallel()

	// Create a generator that yields nothing
	gen := agents.NewGenerator(func(yield func(int) bool) error {
		return nil
	})
	defer gen.Close()

	// First call to Next() should return no values
	_, err, hasMore := gen.Next()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if hasMore {
		t.Error("Expected hasMore=false for empty generator")
	}
}

func TestGenerator_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	producerStopped := make(chan struct{})

	// Create a generator that respects context cancellation
	gen := agents.NewGenerator(func(yield func(int) bool) error {
		defer close(producerStopped)

		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if !yield(i) {
					return nil
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	defer gen.Close()

	// Consume a few values
	for i := 0; i < 3; i++ {
		_, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			t.Fatal("Expected more values")
		}
	}

	// Cancel context
	cancel()

	// Wait for producer to stop
	select {
	case <-producerStopped:
		// Producer stopped as expected
	case <-time.After(1 * time.Second):
		t.Fatal("Producer did not stop after context cancellation")
	}
}
