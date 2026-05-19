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

func TestEngineBreaker_ResetManuallyPreservesConfiguredCallback(t *testing.T) {
	t.Parallel()
	var transitions []string
	cfg := DefaultEngineBreakerConfig("test-callback")
	cfg.MinRequests = 2
	cfg.FailureRatio = 0.5
	cfg.Timeout = 1 * time.Hour
	cfg.OnStateChange = func(name string, from, to BreakerState) {
		transitions = append(transitions, name+":"+from.String()+"->"+to.String())
	}
	b := NewEngineBreaker(cfg)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_ = b.Execute(ctx, func() error { return errors.New("boom") })
	}
	require.NotEmpty(t, transitions, "OnStateChange must fire before reset")

	transitions = nil
	b.ResetManually()

	for i := 0; i < 4; i++ {
		_ = b.Execute(ctx, func() error { return errors.New("boom again") })
	}
	require.NotEmpty(t, transitions,
		"OnStateChange must remain wired after ResetManually (regression: previously hardcoded settings dropped the callback)")
	require.Contains(t, transitions[0], "test-callback:",
		"reset must keep the configured breaker Name, not switch to spotlight.meili.reset")
}

func TestEngineBreaker_ExecuteHonorsCancelledContext(t *testing.T) {
	t.Parallel()
	b := NewEngineBreaker(DefaultEngineBreakerConfig("test-ctx"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := b.Execute(ctx, func() error {
		called = true
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, called, "guarded fn must not run when caller ctx is cancelled")
}

func TestEngineBreaker_RaceFreeStateAfterReset(t *testing.T) {
	t.Parallel()
	cfg := DefaultEngineBreakerConfig("test-race")
	cfg.MinRequests = 2
	cfg.FailureRatio = 0.5
	cfg.Timeout = 1 * time.Hour
	b := NewEngineBreaker(cfg)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			_ = b.Execute(ctx, func() error { return errors.New("x") })
			_ = b.State()
		}
	}()
	for i := 0; i < 20; i++ {
		b.ResetManually()
	}
	<-done
	// The point of this test is `go test -race` cleanliness — concurrent
	// Execute, State, and ResetManually must not race on b.cb. The final
	// breaker state depends on the interleaving so we don't assert it.
	_ = b.State()
}
