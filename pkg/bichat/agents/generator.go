package agents

import "sync"

// Generator provides step-by-step streaming inspired by Python generators.
// T is the value type yielded on each iteration.
//
// Generators enable lazy evaluation and memory-efficient streaming.
// They are used throughout the agent framework for streaming LLM responses,
// event generation, and async processing.
//
// Usage:
//
//	gen := model.Stream(ctx, req)
//	defer gen.Close()
//	for {
//	    chunk, err, hasMore := gen.Next()
//	    if err != nil { return err }
//	    if !hasMore { break }
//	    process(chunk)
//	}
type Generator[T any] interface {
	// Next returns the next value, any error, and whether more values exist.
	// When hasMore is false, the generator is exhausted.
	Next() (value T, err error, hasMore bool)

	// Close releases any resources held by the generator.
	// It should be called when done consuming values (use defer).
	Close()
}

// generatorResult holds a value or error from the producer.
type generatorResult[T any] struct {
	value T
	err   error
}

// generatorImpl is a channel-based generator implementation.
type generatorImpl[T any] struct {
	ch     chan generatorResult[T]
	done   chan struct{}
	mu     sync.Mutex
	closed bool
}

// NewGenerator creates a generator from a producer function.
// The producer receives a yield function to emit values.
// If yield returns false, the producer should stop producing.
// The producer runs in a separate goroutine.
//
// Example:
//
//	gen := NewGenerator(func(yield func(int) bool) error {
//	    for i := 0; i < 10; i++ {
//	        if !yield(i) { return nil } // Consumer stopped
//	    }
//	    return nil
//	})
func NewGenerator[T any](producer func(yield func(T) bool) error) Generator[T] {
	g := &generatorImpl[T]{
		ch:   make(chan generatorResult[T]),
		done: make(chan struct{}),
	}

	go func() {
		defer close(g.ch)

		yield := func(v T) bool {
			select {
			case <-g.done:
				return false
			case g.ch <- generatorResult[T]{value: v}:
				return true
			}
		}

		if err := producer(yield); err != nil {
			select {
			case <-g.done:
			case g.ch <- generatorResult[T]{err: err}:
			}
		}
	}()

	return g
}

// Next returns the next value, any error, and whether more values exist.
// When hasMore is false, the generator is exhausted.
// Returns ErrGeneratorClosed if the generator has been closed.
func (g *generatorImpl[T]) Next() (T, error, bool) {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		var zero T
		return zero, ErrGeneratorClosed, false
	}
	g.mu.Unlock()

	result, ok := <-g.ch
	if !ok {
		var zero T
		return zero, nil, false
	}

	if result.err != nil {
		var zero T
		return zero, result.err, false
	}

	return result.value, nil, true
}

// Close releases any resources held by the generator.
// It signals the producer to stop and can be called multiple times safely.
func (g *generatorImpl[T]) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.closed {
		g.closed = true
		close(g.done)
	}
}

// Collect exhausts the generator and returns all values.
// It automatically closes the generator when done.
//
// Example:
//
//	values, err := Collect(gen)
func Collect[T any](g Generator[T]) ([]T, error) {
	defer g.Close()

	var results []T
	for {
		value, err, hasMore := g.Next()
		if err != nil {
			return results, err
		}
		if !hasMore {
			break
		}
		results = append(results, value)
	}
	return results, nil
}
