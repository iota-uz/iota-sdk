package testdb

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Falsely green if callers use different clusters, or a cancelled waiter is allowed to execute unlocked.
func TestRequiredMigrationLockSerializesAndRejectsCancelledWaiter(t *testing.T) {
	db := testConfig(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- WithMigrationAdvisoryLockContext(ctx, db, func() error {
			close(entered)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("first caller did not acquire")
	}
	var invoked atomic.Bool
	waiting, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	err := WithMigrationAdvisoryLockContext(waiting, db, func() error { invoked.Store(true); return nil })
	stop()
	require.Error(t, err)
	require.False(t, invoked.Load())
	second := make(chan error, 1)
	go func() {
		second <- WithMigrationAdvisoryLockContext(ctx, db, func() error { invoked.Store(true); return nil })
	}()
	close(release)
	require.NoError(t, <-first)
	require.NoError(t, <-second)
	require.True(t, invoked.Load())
}

// Falsely green if unavailable maintenance connections fall back to running the callback.
func TestRequiredMigrationLockPreCancelledNeverCallsMigration(t *testing.T) {
	db := testConfig(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	invoked := false
	err := WithMigrationAdvisoryLockContext(ctx, db, func() error { invoked = true; return nil })
	require.Error(t, err)
	require.False(t, invoked)
}
