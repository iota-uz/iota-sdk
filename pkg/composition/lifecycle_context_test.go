package composition

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLifecycleContextOutlivesStartContext(t *testing.T) {
	lifecycle, stop := context.WithCancel(context.Background())
	startCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	ctx := WithLifecycleContext(startCtx, lifecycle)
	cancel()

	got := LifecycleContext(ctx)
	require.NoError(t, got.Err())
	stop()
	require.ErrorIs(t, got.Err(), context.Canceled)
}

func TestLifecycleContextFallsBackToDetachedContext(t *testing.T) {
	startCtx, cancel := context.WithCancel(context.Background())
	got := LifecycleContext(startCtx)
	cancel()
	require.NoError(t, got.Err())
}
