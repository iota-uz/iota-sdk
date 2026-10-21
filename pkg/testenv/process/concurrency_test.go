package process

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/testenv"
	"github.com/stretchr/testify/require"
)

type gatedResources struct {
	memoryResources
	id, phase        string
	entered, release chan struct{}
}

func (r *gatedResources) wait(id, phase string) {
	if id == r.id && phase == r.phase {
		close(r.entered)
		<-r.release
	}
}
func (r *gatedResources) Prepare(ctx context.Context, id string) ([]string, error) {
	r.wait(id, "prepare")
	return r.memoryResources.Prepare(ctx, id)
}
func (r *gatedResources) Dispose(ctx context.Context, id string) error {
	r.wait(id, "dispose")
	return r.memoryResources.Dispose(ctx, id)
}

// Falsely green if the second environment is only described, without a live readiness response while the first is blocked.
func TestIndependentEnvironmentStartsWhileAnotherLifecycleIsBlocked(t *testing.T) {
	for _, phase := range []string{"prepare", "dispose"} {
		t.Run(phase, func(t *testing.T) {
			id := uuid.NewString()
			r := &gatedResources{memoryResources: memoryResources{owned: map[string]bool{}}, id: id, phase: phase, entered: make(chan struct{}), release: make(chan struct{})}
			var release sync.Once
			defer release.Do(func() { close(r.release) })
			a, err := New(Config{Command: []string{os.Args[0], "-test.run=^TestServerChild$"}, Environment: []string{"TESTENV_PROCESS_CHILD=1"}, Artifacts: t.TempDir(), Revision: "test", Resources: r, ReadyPath: "/ready"})
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			first := testenv.Descriptor{EnvironmentID: id}
			done := make(chan error, 1)
			if phase == "dispose" {
				first, err = a.Start(ctx, testenv.Spec{}, id)
				require.NoError(t, err)
				require.NoError(t, a.Ready(ctx, first))
				go func() { done <- a.Stop(ctx, first) }()
			} else {
				go func() { _, startErr := a.Start(ctx, testenv.Spec{}, id); done <- startErr }()
			}
			select {
			case <-r.entered:
			case <-ctx.Done():
				t.Fatal("first lifecycle never reached its gate")
			}
			secondID := uuid.NewString()
			secondDone := make(chan error, 1)
			go func() {
				d, startErr := a.Start(ctx, testenv.Spec{}, secondID)
				if startErr == nil {
					startErr = a.Ready(ctx, d)
				}
				secondDone <- startErr
			}()
			select {
			case err = <-secondDone:
			case <-time.After(time.Second):
				err = context.DeadlineExceeded
			}
			release.Do(func() { close(r.release) })
			firstErr := <-done
			cleanupCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			require.NoError(t, a.Stop(cleanupCtx, first))
			require.NoError(t, a.Stop(cleanupCtx, testenv.Descriptor{EnvironmentID: secondID}))
			require.NoError(t, firstErr)
			require.NoError(t, err, "an independent environment was serialized behind %s", phase)
		})
	}
}
