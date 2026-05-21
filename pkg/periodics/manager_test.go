package periodics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ctxCapturingTask struct {
	name      string
	timeout   time.Duration
	gotCtx    context.Context
	completed chan struct{}
}

func (t *ctxCapturingTask) Name() string     { return t.name }
func (t *ctxCapturingTask) Schedule() string { return "@every 1h" }
func (t *ctxCapturingTask) RunOnStart() bool { return false }
func (t *ctxCapturingTask) Config() TaskConfig {
	return TaskConfig{
		Timeout:             t.timeout,
		MaxRetries:          IntPtr(1),
		EnableSkipIfRunning: BoolPtr(false),
	}
}

func (t *ctxCapturingTask) Execute(ctx context.Context) error {
	t.gotCtx = ctx
	close(t.completed)
	return nil
}

// TestBuildWrappedExecutor_PropagatesTimeoutDeadline guards against a
// regression where Execute received a bare context.Background() with no
// deadline. The fix injects config.Timeout into ctx so downstream calls
// (DB queries, drains) observe the budget.
func TestBuildWrappedExecutor_PropagatesTimeoutDeadline(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	mgr := NewManager(logger, nil, uuid.Nil).(*manager)

	timeout := 7 * time.Minute
	task := &ctxCapturingTask{
		name:      "ctx-deadline-task",
		timeout:   timeout,
		completed: make(chan struct{}),
	}
	require.NoError(t, mgr.AddTask(task))

	executor := mgr.buildWrappedExecutor(task)
	executor()

	select {
	case <-task.completed:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not execute")
	}

	require.NotNil(t, task.gotCtx, "Execute must receive a non-nil ctx")
	deadline, ok := task.gotCtx.Deadline()
	require.True(t, ok, "Execute ctx must carry the configured timeout as a deadline")

	remaining := time.Until(deadline)
	assert.InDelta(t, timeout.Seconds(), remaining.Seconds(), 5,
		"ctx deadline should be ~config.Timeout from now")
}
