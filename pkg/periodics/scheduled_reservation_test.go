package periodics

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type controlledRunTask struct {
	config  TaskConfig
	execute func(context.Context) error
}

func (t *controlledRunTask) Name() string                      { return "controlled" }
func (t *controlledRunTask) Schedule() string                  { return "@every 1h" }
func (t *controlledRunTask) RunOnStart() bool                  { return false }
func (t *controlledRunTask) Config() TaskConfig                { return t.config }
func (t *controlledRunTask) Execute(ctx context.Context) error { return t.execute(ctx) }

func newRunManager(t *testing.T, task *controlledRunTask) *manager {
	t.Helper()
	m := NewManager(noopLogger(), nil, uuid.Nil).(*manager)
	require.NoError(t, m.AddTask(task))
	return m
}

// Falsely green if timeout alone is treated as completion while Execute still owns the resource.
func TestPeriodicTimeoutDoesNotReleaseScheduledReservation(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	deadline := make(chan struct{}, 2)
	var calls atomic.Int32
	task := &controlledRunTask{config: TaskConfig{Timeout: 20 * time.Millisecond, MaxRetries: IntPtr(1), EnableSkipIfRunning: BoolPtr(true)}, execute: func(ctx context.Context) error {
		calls.Add(1)
		<-ctx.Done()
		deadline <- struct{}{}
		<-release
		return nil
	}}
	m := newRunManager(t, task)
	firstDone := make(chan struct{})
	go func() { m.buildWrappedExecutor(task)(); close(firstDone) }()
	select {
	case <-deadline:
	case <-time.After(time.Second):
		t.Fatal("deadline not delivered")
	}
	secondDone := make(chan struct{})
	go func() { m.buildWrappedExecutor(task)(); close(secondDone) }()
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second invocation did not settle")
	}
	require.Equal(t, int32(1), calls.Load())
	select {
	case <-firstDone:
		t.Fatal("original invocation falsely completed")
	default:
	}
}
