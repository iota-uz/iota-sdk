package periodics

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Falsely green if a different task completion or cancelled waiter releases this invocation.
func TestRunReceiptOwnsCompletionCancellationAndBusy(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	task := &controlledRunTask{config: TaskConfig{Timeout: time.Second, MaxRetries: IntPtr(1)}, execute: func(context.Context) error { close(entered); <-release; return nil }}
	m := newRunManager(t, task)
	receipt, err := m.RunTaskWithReceipt(task.Name())
	require.NoError(t, err)
	<-entered
	_, err = m.RunTaskWithReceipt(task.Name())
	var busy *RunError
	require.ErrorAs(t, err, &busy)
	require.Equal(t, "busy", busy.Code)
	require.Equal(t, receipt.ID, busy.CurrentRunID)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = m.WaitRun(ctx, receipt.ID)
	require.ErrorIs(t, err, context.Canceled)
	running, err := m.GetRun(receipt.ID)
	require.NoError(t, err)
	require.Equal(t, "running", running.Status)
	close(release)
	result, err := m.WaitRun(t.Context(), receipt.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.NotNil(t, result.FinishedAt)
	originalFinished := *result.FinishedAt
	*result.FinishedAt = result.FinishedAt.Add(time.Hour)
	stored, err := m.GetRun(receipt.ID)
	require.NoError(t, err)
	require.Equal(t, originalFinished, *stored.FinishedAt)
	next, err := m.RunTaskWithReceipt("missing")
	require.Empty(t, next)
	require.ErrorAs(t, err, &busy)
	require.Equal(t, "task_not_found", busy.Code)
}

// Falsely green if retries swallow the final error or report success without Execute.
func TestRunReceiptFailuresRetriesAndInvalidConfig(t *testing.T) {
	var calls atomic.Int32
	failure := errors.New("provider unavailable")
	task := &controlledRunTask{config: TaskConfig{Timeout: time.Second, MaxRetries: IntPtr(2), RetryDelay: time.Millisecond}, execute: func(context.Context) error { calls.Add(1); return failure }}
	m := newRunManager(t, task)
	receipt, err := m.RunTaskWithReceipt(task.Name())
	require.NoError(t, err)
	result, err := m.WaitRun(t.Context(), receipt.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.NotEmpty(t, result.Error)
	require.Equal(t, int32(2), calls.Load())
	task.config.MaxRetries = IntPtr(0)
	_, err = m.RunTaskWithReceipt(task.Name())
	var invalid *RunError
	require.ErrorAs(t, err, &invalid)
	require.Equal(t, "invalid_task_config", invalid.Code)
}

// Falsely green if Stop waits only for cron and leaves manually started execution alive.
func TestRunStopCancelsAndJoinsManualInvocation(t *testing.T) {
	entered := make(chan struct{})
	task := &controlledRunTask{execute: func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() }}
	m := newRunManager(t, task)
	receipt, err := m.RunTaskWithReceipt(task.Name())
	require.NoError(t, err)
	<-entered
	require.NoError(t, m.Stop(t.Context()))
	result, err := m.WaitRun(t.Context(), receipt.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", result.Status)
	_, err = m.RunTaskWithReceipt(task.Name())
	var stopped *RunError
	require.ErrorAs(t, err, &stopped)
	require.Equal(t, "manager_stopped", stopped.Code)
}

// Falsely green if two callers can both reserve before the first execution records metrics.
func TestConcurrentManualReceiptsReserveExactlyOne(t *testing.T) {
	release := make(chan struct{})
	var executions atomic.Int32
	task := &controlledRunTask{execute: func(context.Context) error { executions.Add(1); <-release; return nil }}
	m := newRunManager(t, task)
	var callers sync.WaitGroup
	receipts := make(chan RunReceipt, 16)
	errors := make(chan error, 16)
	for i := 0; i < 16; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			receipt, err := m.RunTaskWithReceipt(task.Name())
			if err != nil {
				errors <- err
			} else {
				receipts <- receipt
			}
		}()
	}
	callers.Wait()
	require.Len(t, receipts, 1)
	require.Len(t, errors, 15)
	receipt := <-receipts
	for i := 0; i < 15; i++ {
		var busy *RunError
		require.ErrorAs(t, <-errors, &busy)
		require.Equal(t, receipt.ID, busy.CurrentRunID)
	}
	close(release)
	_, err := m.WaitRun(t.Context(), receipt.ID)
	require.NoError(t, err)
	require.Equal(t, int32(1), executions.Load())
}

// Falsely green if Stop releases an execution that ignores cancellation.
func TestStopDeadlineKeepsInvocationOwnedUntilExecuteExits(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	task := &controlledRunTask{execute: func(context.Context) error { close(entered); <-release; return nil }}
	m := newRunManager(t, task)
	receipt, err := m.RunTaskWithReceipt(task.Name())
	require.NoError(t, err)
	<-entered
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, m.Stop(ctx), context.DeadlineExceeded)
	result, err := m.GetRun(receipt.ID)
	require.NoError(t, err)
	require.Equal(t, "running", result.Status)
	close(release)
	require.NoError(t, m.Stop(t.Context()))
	result, err = m.WaitRun(t.Context(), receipt.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", result.Status)
}

// Falsely green if completed receipts accumulate without a retention bound.
func TestRunReceiptRetentionIsBounded(t *testing.T) {
	task := &controlledRunTask{execute: func(context.Context) error { return nil }}
	m := newRunManager(t, task)
	var first string
	for i := 0; i < retainedCompletedRuns+1; i++ {
		r, err := m.RunTaskWithReceipt(task.Name())
		require.NoError(t, err)
		if i == 0 {
			first = r.ID
		}
		_, err = m.WaitRun(t.Context(), r.ID)
		require.NoError(t, err)
	}
	_, err := m.GetRun(first)
	var missing *RunError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, "run_not_found", missing.Code)
	require.Len(t, m.runs, retainedCompletedRuns)
}

// Falsely green if preserving explicit scheduled overlap also lets manual triggers overlap.
func TestScheduledOverlapOptOutStillRejectsManualRun(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	task := &controlledRunTask{config: TaskConfig{EnableSkipIfRunning: BoolPtr(false)}, execute: func(context.Context) error { entered <- struct{}{}; <-release; return nil }}
	m := newRunManager(t, task)
	first, err := m.startRun(task.Name(), true)
	require.NoError(t, err)
	second, err := m.startRun(task.Name(), true)
	require.NoError(t, err)
	<-entered
	<-entered
	_, err = m.RunTaskWithReceipt(task.Name())
	var busy *RunError
	require.ErrorAs(t, err, &busy)
	require.Equal(t, "busy", busy.Code)
	close(release)
	for _, id := range []string{first.ID, second.ID} {
		result, waitErr := m.WaitRun(t.Context(), id)
		require.NoError(t, waitErr)
		require.Equal(t, "succeeded", result.Status)
	}
	manualRelease := make(chan struct{})
	task.execute = func(context.Context) error { <-manualRelease; return nil }
	manual, err := m.RunTaskWithReceipt(task.Name())
	require.NoError(t, err)
	_, err = m.startRun(task.Name(), true)
	require.ErrorAs(t, err, &busy)
	require.Equal(t, manual.ID, busy.CurrentRunID)
	close(manualRelease)
	_, err = m.WaitRun(t.Context(), manual.ID)
	require.NoError(t, err)
}
