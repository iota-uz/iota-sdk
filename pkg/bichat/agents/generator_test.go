package agents_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

func TestGenerator_Basic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a generator that yields numbers 1-5
	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
		for i := 1; i <= 5; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
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
		value, err := gen.Next(ctx)
		if err != nil {
			if errors.Is(err, types.ErrGeneratorDone) {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
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

	ctx := context.Background()
	producerStarted := make(chan struct{})
	producerStopped := make(chan struct{})

	// Create a generator that yields infinitely but respects cancellation
	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
		close(producerStarted)
		defer close(producerStopped)

		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
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
		_, err := gen.Next(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
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

	// Subsequent calls to Next() should return error after close
	_, err := gen.Next(ctx)
	if err == nil {
		t.Error("Expected error after close")
	}
}

func TestGenerator_Error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("producer error")

	// Create a generator that yields a few values then returns error
	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
		yield(1)
		yield(2)
		return expectedErr
	})
	defer gen.Close()

	// Consume first two values
	for i := 1; i <= 2; i++ {
		value, err := gen.Next(ctx)
		if err != nil {
			t.Fatalf("Unexpected error on value %d: %v", i, err)
		}
		if value != i {
			t.Errorf("Expected value %d, got %d", i, value)
		}
	}

	// Next call should return the error
	_, err := gen.Next(ctx)
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got: %v", expectedErr, err)
	}
}

func TestGenerator_Close(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
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

	// After close, Next() should return error
	_, err := gen.Next(ctx)
	if err == nil {
		t.Error("Expected error after close")
	}
}

func TestGenerator_Collect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a generator that yields numbers 1-10
	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
		for i := 1; i <= 10; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if !yield(i) {
				return nil
			}
		}
		return nil
	})

	// Collect all values
	values, err := types.Collect(ctx, gen)
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

	ctx := context.Background()
	expectedErr := errors.New("collection error")

	// Create a generator that yields a few values then errors
	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
		yield(1)
		yield(2)
		yield(3)
		return expectedErr
	})

	// Collect should return partial results and the error
	values, err := types.Collect(ctx, gen)
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

	ctx := context.Background()
	producerYieldCount := 0

	// Create a generator
	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(string) bool) error {
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
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
		_, err := gen.Next(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
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

	ctx := context.Background()

	// Create a generator that yields nothing
	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
		return nil
	})
	defer gen.Close()

	// First call to Next() should return ErrGeneratorDone
	_, err := gen.Next(ctx)
	if !errors.Is(err, types.ErrGeneratorDone) {
		t.Errorf("Expected ErrGeneratorDone for empty generator, got: %v", err)
	}
}

func TestGenerator_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	producerStopped := make(chan struct{})

	// Create a generator that respects context cancellation
	gen := types.NewGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
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
		_, err := gen.Next(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
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
