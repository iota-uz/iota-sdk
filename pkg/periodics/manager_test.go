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
	name        string
	timeout     time.Duration
	gotDeadline time.Time
	gotHasDL    bool
	executedAt  time.Time
	completed   chan struct{}
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
	// Capture deadline state inside Execute so the delta we measure is
	// "deadline - executedAt", not "deadline - assertion time". The latter
	// drifts under GC / scheduler stalls and would flake on a busy CI.
	t.executedAt = time.Now()
	t.gotDeadline, t.gotHasDL = ctx.Deadline()
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

	require.True(t, task.gotHasDL, "Execute ctx must carry the configured timeout as a deadline")
	require.False(t, task.executedAt.IsZero(), "expected executedAt to be set inside Execute")

	// Span between ctx creation in buildWrappedExecutor and Execute entry is
	// microseconds in practice; 1s tolerance trivially absorbs scheduler jitter.
	budget := task.gotDeadline.Sub(task.executedAt)
	assert.InDelta(t, timeout.Seconds(), budget.Seconds(), 1,
		"ctx deadline should be ~config.Timeout from the moment Execute runs")
}
