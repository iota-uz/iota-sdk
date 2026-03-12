// Package periodics provides a reusable periodic task manager built on robfig/cron v3.
// It supports cron-based scheduling, retry with exponential backoff, timeout,
// skip-if-running, metrics collection, and health reporting.
package periodics

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
)

// PeriodicTask represents a task that runs periodically
type PeriodicTask interface {
	// Name returns the unique name of the task
	Name() string

	// Schedule returns the cron schedule expression (e.g., "*/5 * * * *")
	Schedule() string

	// Execute runs the task with the given context
	Execute(ctx context.Context) error

	// Config returns the task configuration
	Config() TaskConfig

	// RunOnStart returns true if the task should run immediately on startup
	RunOnStart() bool
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

	// GetEntries returns all registered cron entries
	GetEntries() []cron.Entry

	// GetMetrics returns performance metrics for all tasks
	GetMetrics() map[string]*TaskMetrics

	// LogHealthReport logs a health report for all tasks
	LogHealthReport()
}

// TaskConfig holds configuration for a periodic task
type TaskConfig struct {
	// MaxRetries is the maximum number of retry attempts on failure
	MaxRetries int

	// RetryDelay is the initial delay between retries (will be exponentially increased)
	RetryDelay time.Duration

	// Timeout is the maximum time a task can run before being cancelled
	Timeout time.Duration

	// EnableSkipIfRunning skips execution if previous instance is still running
	EnableSkipIfRunning bool
}

// DefaultTaskConfig returns a default configuration for periodic tasks
func DefaultTaskConfig() TaskConfig {
	return TaskConfig{
		MaxRetries:          3,
		RetryDelay:          time.Second,
		Timeout:             5 * time.Minute,
		EnableSkipIfRunning: true,
	}
}
