package crud_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/stretchr/testify/require"
)

// TestOptionsLoader_Cache ensures that the dynamic options loader is executed
// exactly once and subsequent invocations return the cached slice.
func TestOptionsLoader_Cache(t *testing.T) {
	var calls int32

	loader := func(ctx context.Context) []crud.SelectOption {
		atomic.AddInt32(&calls, 1)
		return []crud.SelectOption{
			{Value: "1", Label: "One"},
			{Value: "2", Label: "Two"},
		}
	}

	f := crud.NewSelectField("test_field").SetOptionsLoader(loader)

	// Simulate many calls that could happen during rendering of multiple rows.
	for i := 0; i < 100; i++ {
		_ = f.OptionsLoader()(context.Background())
	}

	require.Equal(t, int32(1), calls, "loader should be called exactly once")
}

// TestOptionsLoader_TTL ensures that cached options expire after the TTL and
// loader is executed again.
func TestOptionsLoader_TTL(t *testing.T) {
	var calls int32

	loader := func(ctx context.Context) []crud.SelectOption {
		atomic.AddInt32(&calls, 1)
		return []crud.SelectOption{{Value: "x", Label: "X"}}
	}

	f := crud.NewSelectField("demo").
		SetOptionsLoader(loader).
		SetOptionsCacheTTL(50 * time.Millisecond)

	// first call - loads and caches
	_ = f.OptionsLoader()(context.Background())
	require.Equal(t, int32(1), calls)

	// second call within TTL should use cache
	_ = f.OptionsLoader()(context.Background())
	require.Equal(t, int32(1), calls)

	// wait for ttl to expire
	time.Sleep(70 * time.Millisecond)

	// third call after ttl should trigger loader again
	_ = f.OptionsLoader()(context.Background())
	require.Equal(t, int32(2), calls)
}

// TestOptionsLoader_Invalidate verifies that manual cache invalidation forces
// reloading.
func TestOptionsLoader_Invalidate(t *testing.T) {
	var calls int32

	loader := func(ctx context.Context) []crud.SelectOption {
		atomic.AddInt32(&calls, 1)
		return []crud.SelectOption{{Value: "x", Label: "X"}}
	}

	f := crud.NewSelectField("demo").SetOptionsLoader(loader)

	_ = f.OptionsLoader()(context.Background())
	require.Equal(t, int32(1), calls)

	// invalidate cache and expect loader to run again
	f.InvalidateOptionsCache()

	_ = f.OptionsLoader()(context.Background())
	require.Equal(t, int32(2), calls)
}
