package composition_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/stretchr/testify/require"
)

// Characterisation of the OwnedResource contract. Falsely green if Close
// returned early while still closing the resource: every case asserts the
// close counter at the moment Close returns, not only eventually.
func TestOwnedResource(t *testing.T) {
	t.Parallel()

	t.Run("closes immediately without holders", func(t *testing.T) {
		t.Parallel()
		var closed atomic.Int32
		r := composition.NewOwnedResource(func() error { closed.Add(1); return nil })
		require.NoError(t, r.Close(context.Background()))
		require.Equal(t, int32(1), closed.Load())
		require.NoError(t, r.Close(context.Background()), "close is idempotent")
		require.Equal(t, int32(1), closed.Load())
		_, err := r.Acquire()
		require.ErrorIs(t, err, composition.ErrResourceClosing)
	})

	t.Run("waits for the last holder within the deadline", func(t *testing.T) {
		t.Parallel()
		var closed atomic.Int32
		r := composition.NewOwnedResource(func() error { closed.Add(1); return nil })
		release, err := r.Acquire()
		require.NoError(t, err)
		go func() {
			time.Sleep(20 * time.Millisecond)
			release()
			release() // idempotent: must not underflow or double close
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, r.Close(ctx))
		require.Equal(t, int32(1), closed.Load())
	})

	t.Run("deadline defers close to the last release", func(t *testing.T) {
		t.Parallel()
		var closed atomic.Int32
		r := composition.NewOwnedResource(func() error { closed.Add(1); return errors.New("close failed") })
		release, err := r.Acquire()
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		err = r.Close(ctx)
		require.ErrorIs(t, err, composition.ErrResourceInUse)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, int32(0), closed.Load(), "resource must stay open while held")
		_, err = r.Acquire()
		require.ErrorIs(t, err, composition.ErrResourceClosing, "no new holders once closing")

		release()
		<-r.Done()
		require.Equal(t, int32(1), closed.Load())
		require.EqualError(t, r.Close(context.Background()), "close failed", "later Close reports the deferred result")
	})

	t.Run("nil resource is a no-op", func(t *testing.T) {
		t.Parallel()
		var r *composition.OwnedResource
		release, err := r.Acquire()
		require.NoError(t, err)
		release()
		require.NoError(t, r.Close(context.Background()))
		<-r.Done()
	})
}
