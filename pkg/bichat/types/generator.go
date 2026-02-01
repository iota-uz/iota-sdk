package types

import (
	"context"
	"errors"
	"sync"
)

// ErrGeneratorDone is returned when the generator has no more values.
var ErrGeneratorDone = errors.New("generator done")

// Generator is a context-aware iterator interface for streaming operations.
type Generator[T any] interface {
	// Next returns the next value, respecting context cancellation.
	// Returns an error if the context is cancelled or the generator encounters an error.
	Next(ctx context.Context) (T, error)

	// TryNext attempts to get the next value without blocking.
	// Returns (value, true, nil) if a value is available.
	// Returns (zero, false, nil) if no value is currently available.
	// Returns (zero, false, error) if the generator is closed or encountered an error.
	TryNext() (T, bool, error)

	// Close stops the generator and releases resources.
	Close()

	// Done returns a channel that is closed when the generator is finished.
	Done() <-chan struct{}
}

// generatorConfig holds configuration options for a generator.
type generatorConfig struct {
	bufferSize int
}

// GeneratorOption is a functional option for configuring a Generator.
type GeneratorOption func(*generatorConfig)

// WithBufferSize sets the buffer size for the generator's internal channel.
func WithBufferSize(size int) GeneratorOption {
	return func(c *generatorConfig) {
		c.bufferSize = size
	}
}

// generator is the internal implementation of Generator.
type generator[T any] struct {
	ch     chan T
	done   chan struct{}
	err    error
	errMu  sync.RWMutex
	cancel context.CancelFunc
}

// NewGenerator creates a new Generator that produces values using the provided producer function.
// The producer function should call yield(value) for each value to emit.
// If yield returns false, the producer should stop producing values.
func NewGenerator[T any](ctx context.Context, producer func(ctx context.Context, yield func(T) bool) error, opts ...GeneratorOption) Generator[T] {
	config := &generatorConfig{
		bufferSize: 0, // Unbuffered by default
	}
	for _, opt := range opts {
		opt(config)
	}

	genCtx, cancel := context.WithCancel(ctx)
	g := &generator[T]{
		ch:     make(chan T, config.bufferSize),
		done:   make(chan struct{}),
		cancel: cancel,
	}

	go func() {
		defer close(g.ch)
		defer close(g.done)
		defer cancel()

		yield := func(value T) bool {
			select {
			case <-genCtx.Done():
				return false
			case g.ch <- value:
				return true
			}
		}

		if err := producer(genCtx, yield); err != nil {
			g.errMu.Lock()
			g.err = err
			g.errMu.Unlock()
		}
	}()

	return g
}

// Next implements Generator.Next.
func (g *generator[T]) Next(ctx context.Context) (T, error) {
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case value, ok := <-g.ch:
		if !ok {
			var zero T
			g.errMu.RLock()
			err := g.err
			g.errMu.RUnlock()
			if err != nil {
				return zero, err
			}
			return zero, ErrGeneratorDone
		}
		return value, nil
	}
}

// TryNext implements Generator.TryNext.
func (g *generator[T]) TryNext() (T, bool, error) {
	select {
	case value, ok := <-g.ch:
		if !ok {
			var zero T
			g.errMu.RLock()
			err := g.err
			g.errMu.RUnlock()
			return zero, false, err
		}
		return value, true, nil
	default:
		var zero T
		return zero, false, nil
	}
}

// Close implements Generator.Close.
func (g *generator[T]) Close() {
	g.cancel()
}

// Done implements Generator.Done.
func (g *generator[T]) Done() <-chan struct{} {
	return g.done
}

// Collect consumes all values from the generator and returns them as a slice.
// This function blocks until the generator is exhausted or the context is cancelled.
// Returns nil error when the generator completes normally (ErrGeneratorDone is not propagated).
func Collect[T any](ctx context.Context, gen Generator[T]) ([]T, error) {
	var result []T
	for {
		value, err := gen.Next(ctx)
		if err != nil {
			if errors.Is(err, ErrGeneratorDone) {
				// Generator finished normally
				return result, nil
			}
			// Actual error occurred
			return result, err
		}
		result = append(result, value)
	}
}
