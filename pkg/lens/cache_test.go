package lens

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// anyQueryDS returns the same result for any query and tracks call counts.
type anyQueryDS struct {
	calls  atomic.Int64
	result *QueryResult
	err    error
	closed atomic.Bool
}

func (d *anyQueryDS) Execute(_ context.Context, _ string) (*QueryResult, error) {
	d.calls.Add(1)
	return d.result, d.err
}

func (d *anyQueryDS) Close() error {
	d.closed.Store(true)
	return nil
}

// ---------------------------------------------------------------------------
// TestWithCache_Hit
// ---------------------------------------------------------------------------

func TestWithCache_Hit(t *testing.T) {
	t.Parallel()

	inner := &anyQueryDS{
		result: &QueryResult{
			Columns: []QueryColumn{{Name: "value", Type: "number"}},
			Rows:    []map[string]any{{"value": float64(42)}},
		},
	}

	cached := WithCache(inner, 5*time.Minute)
	defer func() { _ = cached.Close() }()

	ctx := context.Background()
	const query = "SELECT 42 AS value"

	// First call — should hit the underlying DataSource.
	r1, err := cached.Execute(ctx, query)
	if err != nil {
		t.Fatalf("first Execute: unexpected error: %v", err)
	}
	if r1 == nil {
		t.Fatal("first Execute: expected non-nil result")
	}

	// Second call with the same query — should serve from cache.
	r2, err := cached.Execute(ctx, query)
	if err != nil {
		t.Fatalf("second Execute: unexpected error: %v", err)
	}
	if r2 == nil {
		t.Fatal("second Execute: expected non-nil result")
	}

	// Verify the underlying DataSource was only called once.
	if got := inner.calls.Load(); got != 1 {
		t.Errorf("expected inner DataSource to be called 1 time, got %d", got)
	}

	// Both results should point to the same underlying data.
	if r1 != r2 {
		t.Error("expected cached result to be the same pointer as the original")
	}
}

// ---------------------------------------------------------------------------
// TestWithCache_Miss
// ---------------------------------------------------------------------------

func TestWithCache_Miss(t *testing.T) {
	t.Parallel()

	inner := &anyQueryDS{
		result: &QueryResult{
			Rows: []map[string]any{{"value": float64(1)}},
		},
	}

	cached := WithCache(inner, 5*time.Minute)
	defer func() { _ = cached.Close() }()

	ctx := context.Background()

	// Two different queries — both should hit the underlying DataSource.
	_, err := cached.Execute(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("query1: unexpected error: %v", err)
	}

	_, err = cached.Execute(ctx, "SELECT 2")
	if err != nil {
		t.Fatalf("query2: unexpected error: %v", err)
	}

	if got := inner.calls.Load(); got != 2 {
		t.Errorf("expected inner DataSource to be called 2 times, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// TestWithCache_SameQueryDifferentResult (ensure cache key is per query)
// ---------------------------------------------------------------------------

func TestWithCache_CacheKeyPerQuery(t *testing.T) {
	t.Parallel()

	callCount := atomic.Int64{}
	results := []int{10, 20}
	idx := atomic.Int64{}

	ds := &funcDataSource{
		executeFn: func(_ context.Context, query string) (*QueryResult, error) {
			callCount.Add(1)
			i := idx.Add(1) - 1
			val := results[i%int64(len(results))]
			return &QueryResult{
				Rows: []map[string]any{{"value": float64(val)}},
			}, nil
		},
	}

	cached := WithCache(ds, 5*time.Minute)
	defer func() { _ = cached.Close() }()

	ctx := context.Background()
	q1 := "SELECT a"
	q2 := "SELECT b"

	// Execute both queries once — 2 calls expected.
	_, _ = cached.Execute(ctx, q1)
	_, _ = cached.Execute(ctx, q2)

	// Execute both again — should be served from cache (no new calls).
	_, _ = cached.Execute(ctx, q1)
	_, _ = cached.Execute(ctx, q2)

	if got := callCount.Load(); got != 2 {
		t.Errorf("expected 2 underlying calls (one per unique query), got %d", got)
	}
}

// ---------------------------------------------------------------------------
// TestWithCache_Expiry
// ---------------------------------------------------------------------------

func TestWithCache_Expiry(t *testing.T) {
	t.Parallel()

	inner := &anyQueryDS{
		result: &QueryResult{
			Rows: []map[string]any{{"value": float64(99)}},
		},
	}

	// Very short TTL so the entry expires quickly.
	ttl := 20 * time.Millisecond
	cached := WithCache(inner, ttl)
	defer func() { _ = cached.Close() }()

	ctx := context.Background()
	const query = "SELECT 99"

	// First call — cache miss.
	_, err := cached.Execute(ctx, query)
	if err != nil {
		t.Fatalf("first Execute: unexpected error: %v", err)
	}

	// Wait for TTL to expire.
	time.Sleep(ttl * 3)

	// Second call after expiry — should be a cache miss again.
	_, err = cached.Execute(ctx, query)
	if err != nil {
		t.Fatalf("second Execute: unexpected error: %v", err)
	}

	if got := inner.calls.Load(); got != 2 {
		t.Errorf("expected inner DataSource to be called 2 times (expiry re-fetch), got %d", got)
	}
}

// ---------------------------------------------------------------------------
// TestWithCache_Close
// ---------------------------------------------------------------------------

func TestWithCache_Close(t *testing.T) {
	t.Parallel()

	inner := &anyQueryDS{
		result: &QueryResult{},
	}

	cached := WithCache(inner, 5*time.Minute)

	// Populate a cache entry.
	_, err := cached.Execute(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Close should propagate to inner DataSource.
	if err := cached.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}

	if !inner.closed.Load() {
		t.Error("expected inner DataSource Close() to be called")
	}
}

// ---------------------------------------------------------------------------
// TestWithCache_ErrorNotCached
// ---------------------------------------------------------------------------

func TestWithCache_ErrorNotCached(t *testing.T) {
	t.Parallel()

	callCount := atomic.Int64{}
	sentinelErr := errors.New("transient error")
	callNum := atomic.Int64{}

	ds := &funcDataSource{
		executeFn: func(_ context.Context, _ string) (*QueryResult, error) {
			n := callNum.Add(1)
			callCount.Add(1)
			if n == 1 {
				// First call fails.
				return nil, sentinelErr
			}
			// Subsequent calls succeed.
			return &QueryResult{Rows: []map[string]any{{"value": float64(1)}}}, nil
		},
	}

	cached := WithCache(ds, 5*time.Minute)
	defer func() { _ = cached.Close() }()

	ctx := context.Background()
	const query = "SELECT 1"

	// First call — should get an error (not cached on error).
	_, err := cached.Execute(ctx, query)
	if err == nil {
		t.Fatal("expected error on first call")
	}

	// Second call — error should NOT be cached; underlying DS should be called again.
	r, err := cached.Execute(ctx, query)
	if err != nil {
		t.Fatalf("second Execute: unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("second Execute: expected non-nil result")
	}

	if got := callCount.Load(); got != 2 {
		t.Errorf("expected 2 calls to inner DS, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// TestWithCache_HitWithinTTL
// ---------------------------------------------------------------------------

func TestWithCache_HitWithinTTL(t *testing.T) {
	t.Parallel()

	inner := &anyQueryDS{
		result: &QueryResult{
			Rows: []map[string]any{{"value": float64(7)}},
		},
	}

	ttl := 200 * time.Millisecond
	cached := WithCache(inner, ttl)
	defer func() { _ = cached.Close() }()

	ctx := context.Background()
	const query = "SELECT 7"

	// Execute 3 times within TTL — all should be served from cache.
	for i := 0; i < 3; i++ {
		_, err := cached.Execute(ctx, query)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}

	if got := inner.calls.Load(); got != 1 {
		t.Errorf("expected 1 inner call within TTL, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// funcDataSource — a DataSource backed by a function for flexible test setup
// ---------------------------------------------------------------------------

type funcDataSource struct {
	executeFn func(ctx context.Context, query string) (*QueryResult, error)
	closeFn   func() error
}

func (f *funcDataSource) Execute(ctx context.Context, query string) (*QueryResult, error) {
	return f.executeFn(ctx, query)
}

func (f *funcDataSource) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}
