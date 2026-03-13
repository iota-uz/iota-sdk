// Package periodics provides a reusable periodic task manager built on robfig/cron v3.
// It supports cron-based scheduling, retry with exponential backoff, timeout,
// skip-if-running, metrics collection, and health reporting.
package periodics

import (
	"context"
	"time"
)

// PeriodicTask represents a task that runs periodically
type PeriodicTask interface {
	// Name returns the unique name of the task
	Name() string

	// Schedule returns the cron schedule expression (e.g., "*/5 * * * *")
	Schedule() string

	// Execute runs the task with the given context
	Execute(ctx context.Context) error

	// Config returns the task configuration.
	// Zero-value fields are merged with DefaultTaskConfig() by the manager,
	// so tasks only need to override fields they care about.
	Config() TaskConfig

	// RunOnStart returns true if the task should run immediately on startup
	RunOnStart() bool
}

// Entry represents a scheduled task entry, abstracting away the cron library details
type Entry struct {
	// ID is the unique identifier for this entry
	ID int
	// Next is the next time the task will run
	Next time.Time
	// Prev is the last time the task ran
	Prev time.Time
}

// RegisteredTask describes a task that has been added to a Manager.
type RegisteredTask struct {
	Name       string
	Schedule   string
	RunOnStart bool
	Enabled    bool
}

// Manager manages periodic tasks using cron
type Manager interface {
	// AddTask registers a periodic task
	AddTask(task PeriodicTask) error

	// Start begins executing all registered tasks
	Start() error

	// Stop gracefully stops all running tasks
	Stop(ctx context.Context) error

	// IsRunning returns true if the manager is currently running
	IsRunning() bool

	// GetEntries returns all registered scheduled entries
	GetEntries() []Entry

	// GetMetrics returns performance metrics for all tasks
	GetMetrics() map[string]*TaskMetrics

	// LogHealthReport logs a health report for all tasks
	LogHealthReport()

	// GetRegisteredTasks returns information about all registered tasks (both enabled and disabled).
	GetRegisteredTasks() []RegisteredTask

	// AddDisabledTaskInfo registers metadata for a disabled task so it appears in monitoring
	// without being scheduled for execution.
	AddDisabledTaskInfo(name, schedule string)

	// GetTaskScheduleInfo returns scheduling information for all tasks, keyed by task name.
	// Added to support correct next/prev run correlation without relying on map iteration order.
	GetTaskScheduleInfo() map[string]TaskScheduleInfo

	// SubscribeMetrics returns a channel that receives events when task metrics change,
	// and an unsubscribe function to stop receiving events and close the channel.
	SubscribeMetrics() (<-chan TaskMetricEvent, func())
}

// TaskScheduleInfo provides scheduling information for a specific task.
type TaskScheduleInfo struct {
	Next time.Time
	Prev time.Time
}

// TaskMetricEvent is emitted when a task's metrics change.
type TaskMetricEvent struct {
	TaskName  string
	EventType string // "start", "success", "failure"
	Metrics   *TaskMetrics
}

// TaskConfig holds configuration for a periodic task.
// Zero-value fields will be filled with defaults from DefaultTaskConfig().
// Use pointers for bool fields that need explicit false vs unset distinction.
type TaskConfig struct {
	// MaxRetries is the maximum number of retry attempts on failure (default: 3).
	// Use a pointer to distinguish unset (nil → default 3) from explicit 0 (no retries).
	MaxRetries *int

	// RetryDelay is the initial delay between retries, exponentially increased (default: 1s)
	RetryDelay time.Duration

	// Timeout is the maximum time a task can run before being cancelled (default: 5m)
	Timeout time.Duration

	// EnableSkipIfRunning skips execution if previous instance is still running.
	// Use a pointer to distinguish unset (nil → default true) from explicit false.
	EnableSkipIfRunning *bool
}

// IntPtr returns a pointer to the given int value, useful for setting TaskConfig.MaxRetries
func IntPtr(v int) *int { return &v }

// BoolPtr returns a pointer to the given bool value, useful for setting TaskConfig.EnableSkipIfRunning
func BoolPtr(v bool) *bool { return &v }

// DefaultTaskConfig returns a default configuration for periodic tasks
func DefaultTaskConfig() TaskConfig {
	return TaskConfig{
		MaxRetries:          IntPtr(3),
		RetryDelay:          time.Second,
		Timeout:             5 * time.Minute,
		EnableSkipIfRunning: BoolPtr(true),
	}
}

// mergeWithDefaults fills nil/zero-value fields in cfg with values from DefaultTaskConfig()
func mergeWithDefaults(cfg TaskConfig) TaskConfig {
	defaults := DefaultTaskConfig()
	if cfg.MaxRetries == nil {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = defaults.RetryDelay
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaults.Timeout
	}
	if cfg.EnableSkipIfRunning == nil {
		cfg.EnableSkipIfRunning = defaults.EnableSkipIfRunning
	}
	return cfg
}
