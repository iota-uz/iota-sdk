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
}

// TaskConfig holds configuration for a periodic task.
// Zero-value fields will be filled with defaults from DefaultTaskConfig().
// Use pointers for bool fields that need explicit false vs unset distinction.
type TaskConfig struct {
	// MaxRetries is the maximum number of retry attempts on failure (default: 3)
	MaxRetries int

	// RetryDelay is the initial delay between retries, exponentially increased (default: 1s)
	RetryDelay time.Duration

	// Timeout is the maximum time a task can run before being cancelled (default: 5m)
	Timeout time.Duration

	// EnableSkipIfRunning skips execution if previous instance is still running.
	// Use a pointer to distinguish between unset (nil → default true) and explicit false.
	EnableSkipIfRunning *bool
}

// DefaultTaskConfig returns a default configuration for periodic tasks
func DefaultTaskConfig() TaskConfig {
	skipIfRunning := true
	return TaskConfig{
		MaxRetries:          3,
		RetryDelay:          time.Second,
		Timeout:             5 * time.Minute,
		EnableSkipIfRunning: &skipIfRunning,
	}
}

// mergeWithDefaults fills zero-value fields in cfg with values from DefaultTaskConfig()
func mergeWithDefaults(cfg TaskConfig) TaskConfig {
	defaults := DefaultTaskConfig()
	if cfg.MaxRetries == 0 {
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
