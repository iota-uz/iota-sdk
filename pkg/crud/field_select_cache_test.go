package crud_test

import (
	"context"
	"sync/atomic"
	"testing"

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
