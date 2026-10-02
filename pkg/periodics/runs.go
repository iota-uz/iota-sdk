package periodics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// RunController is an optional correlated invocation API. Manager implementers
// remain compatible; callers can detect support with a type assertion. Manual
// receipts reserve exclusive execution, including against scheduled work.
// Scheduled skip=false permits overlap only among scheduled invocations.
type RunController interface {
	RunTaskWithReceipt(string) (RunReceipt, error)
	GetRun(string) (RunResult, error)
	WaitRun(context.Context, string) (RunResult, error)
}

type RunReceipt struct {
	ID        string    `json:"id"`
	TaskName  string    `json:"taskName"`
	StartedAt time.Time `json:"startedAt"`
}

type RunResult struct {
	RunReceipt
	Status     string     `json:"status"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type RunError struct {
	Code         string `json:"code"`
	CurrentRunID string `json:"currentRunId,omitempty"`
}

func (e *RunError) Error() string { return "periodic invocation: " + e.Code }

const retainedCompletedRuns = 256

type managedRun struct {
	result    RunResult
	cancel    context.CancelFunc
	done      chan struct{}
	exclusive bool
}

func (m *manager) RunTaskWithReceipt(name string) (RunReceipt, error) {
	return m.startRun(name, false)
}

func (m *manager) startRun(name string, allowOverlap bool) (RunReceipt, error) {
	m.mu.RLock()
	task := m.tasks[name]
	disabled := false
	if task == nil {
		for _, info := range m.disabledTasks {
			if info.Name == name {
				disabled = true
				break
			}
		}
	}
	m.mu.RUnlock()
	if task == nil {
		if disabled {
			return RunReceipt{}, &RunError{Code: "task_disabled"}
		}
		return RunReceipt{}, &RunError{Code: "task_not_found"}
	}
	m.runsMu.Lock()
	defer m.runsMu.Unlock()
	if m.stopping {
		return RunReceipt{}, &RunError{Code: "manager_stopped"}
	}
	if running := m.activeRuns[name]; len(running) > 0 && !allowOverlap {
		return RunReceipt{}, &RunError{Code: "busy", CurrentRunID: running[0].result.ID}
	}
	for _, running := range m.activeRuns[name] {
		if running.exclusive {
			return RunReceipt{}, &RunError{Code: "busy", CurrentRunID: running.result.ID}
		}
	}
	cfg := mergeWithDefaults(task.Config())
	if *cfg.MaxRetries < 0 || cfg.Timeout <= 0 || cfg.RetryDelay < 0 {
		return RunReceipt{}, &RunError{Code: "invalid_task_config"}
	}
	if *cfg.MaxRetries == 0 {
		cfg.MaxRetries = IntPtr(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	ctx = composables.WithPool(ctx, m.pool)
	ctx = composables.WithTenantID(ctx, m.tenantID)
	ctx = context.WithValue(ctx, constants.LoggerKey, m.logger.WithField("task", name))
	receipt := RunReceipt{ID: uuid.NewString(), TaskName: name, StartedAt: time.Now().UTC()}
	run := &managedRun{result: RunResult{RunReceipt: receipt, Status: "running"}, cancel: cancel, done: make(chan struct{}), exclusive: !allowOverlap}
	m.runs[receipt.ID] = run
	m.activeRuns[name] = append(m.activeRuns[name], run)
	go m.executeRun(ctx, task, cfg, run)
	return receipt, nil
}

func (m *manager) executeRun(ctx context.Context, task PeriodicTask, cfg TaskConfig, run *managedRun) {
	defer run.cancel()
	var resultErr error
	for attempt := 0; attempt < *cfg.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			resultErr = err
			break
		}
		m.metrics.RecordTaskStart(task.Name())
		started := time.Now()
		resultErr = executeTask(ctx, task)
		if resultErr == nil {
			resultErr = ctx.Err()
		}
		if resultErr == nil {
			m.metrics.RecordTaskSuccess(task.Name(), time.Since(started))
			break
		}
		m.metrics.RecordTaskFailure(task.Name(), time.Since(started), resultErr)
		m.logger.WithError(resultErr).WithField("task", task.Name()).Warn("Periodic invocation attempt failed")
		if attempt+1 == *cfg.MaxRetries {
			break
		}
		delay := cfg.RetryDelay
		for i := 0; i < attempt && delay < time.Hour; i++ {
			delay *= 2
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			resultErr = ctx.Err()
		case <-timer.C:
		}
		timer.Stop()
	}
	finished := time.Now().UTC()
	m.runsMu.Lock()
	defer m.runsMu.Unlock()
	run.result.FinishedAt = &finished
	run.result.Status = "succeeded"
	if resultErr != nil {
		run.result.Status = "failed"
		if errors.Is(resultErr, context.Canceled) || errors.Is(resultErr, context.DeadlineExceeded) {
			run.result.Status = "cancelled"
		}
		run.result.Error = resultErr.Error()
	}
	active := m.activeRuns[task.Name()]
	for i, owned := range active {
		if owned == run {
			active = append(active[:i], active[i+1:]...)
			break
		}
	}
	if len(active) == 0 {
		delete(m.activeRuns, task.Name())
	} else {
		m.activeRuns[task.Name()] = active
	}
	m.completedRuns = append(m.completedRuns, run.result.ID)
	if len(m.completedRuns) > retainedCompletedRuns {
		delete(m.runs, m.completedRuns[0])
		m.completedRuns = m.completedRuns[1:]
	}
	close(run.done)
}

func executeTask(ctx context.Context, task PeriodicTask) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if cause, ok := recovered.(error); ok {
				err = fmt.Errorf("periodic task panic: %w", cause)
			} else {
				err = fmt.Errorf("periodic task panic: %v", recovered)
			}
		}
	}()
	return task.Execute(ctx)
}

func (m *manager) GetRun(id string) (RunResult, error) {
	m.runsMu.Lock()
	defer m.runsMu.Unlock()
	run := m.runs[id]
	if run == nil {
		return RunResult{}, &RunError{Code: "run_not_found"}
	}
	return copyRunResult(run.result), nil
}

// WaitRun cancels only the wait; stopping the manager cancels owned executions.
// Completion means Execute and all retries have actually exited.
func (m *manager) WaitRun(ctx context.Context, id string) (RunResult, error) {
	m.runsMu.Lock()
	run := m.runs[id]
	m.runsMu.Unlock()
	if run == nil {
		return RunResult{}, &RunError{Code: "run_not_found"}
	}
	select {
	case <-ctx.Done():
		return RunResult{}, ctx.Err()
	case <-run.done:
	}
	m.runsMu.Lock()
	defer m.runsMu.Unlock()
	return copyRunResult(run.result), nil
}

func copyRunResult(result RunResult) RunResult {
	if result.FinishedAt != nil {
		finished := *result.FinishedAt
		result.FinishedAt = &finished
	}
	return result
}
