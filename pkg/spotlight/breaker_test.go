package spotlight

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEngineBreaker_OpensAfterRepeatedFailures(t *testing.T) {
	t.Parallel()
	cfg := DefaultEngineBreakerConfig("test")
	cfg.MinRequests = 4
	cfg.FailureRatio = 0.5
	cfg.Interval = 5 * time.Second
	cfg.Timeout = 5 * time.Second
	b := NewEngineBreaker(cfg)

	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_ = b.Execute(ctx, func() error { return errors.New("boom") })
	}
	require.Equal(t, BreakerStateOpen, b.State())

	// Execute now short-circuits with ErrBreakerOpen.
	err := b.Execute(ctx, func() error { return nil })
	require.ErrorIs(t, err, ErrBreakerOpen)
}

func TestEngineBreaker_ResetManuallyReturnsToClosed(t *testing.T) {
	t.Parallel()
	cfg := DefaultEngineBreakerConfig("test")
	cfg.MinRequests = 2
	cfg.FailureRatio = 0.5
	cfg.Timeout = 1 * time.Hour
	b := NewEngineBreaker(cfg)

	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_ = b.Execute(ctx, func() error { return errors.New("boom") })
	}
	require.Equal(t, BreakerStateOpen, b.State())

	b.ResetManually()
	require.Equal(t, BreakerStateClosed, b.State())

	// And a passing call now flows through.
	require.NoError(t, b.Execute(ctx, func() error { return nil }))
}
