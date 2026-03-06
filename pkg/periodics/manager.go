package periodics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	sdkcomposables "github.com/iota-uz/iota-sdk/pkg/composables"
	sdkconstants "github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// manager implements the Manager interface
type manager struct {
	cron     *cron.Cron
	logger   *logrus.Logger
	metrics  *MetricsCollector
	pool     *pgxpool.Pool
	tenantID uuid.UUID
	mu       sync.RWMutex
	tasks    map[string]PeriodicTask
}

// NewManager creates a new periodic task manager
func NewManager(logger *logrus.Logger, pool *pgxpool.Pool, tenantID uuid.UUID) Manager {
	return &manager{
		cron:     nil, // Will be initialized in Start()
		logger:   logger,
		metrics:  NewMetricsCollector(logger),
		pool:     pool,
		tenantID: tenantID,
		tasks:    make(map[string]PeriodicTask),
	}
}

// AddTask registers a periodic task
func (m *manager) AddTask(task PeriodicTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskName := task.Name()
	if _, exists := m.tasks[taskName]; exists {
		return fmt.Errorf("task with name '%s' already exists", taskName)
	}

	m.tasks[taskName] = task
	m.logger.WithField("task", taskName).Info("Periodic task registered")

	// If cron is already running, add the task immediately
	if m.cron != nil {
		if err := m.addTaskToCron(task); err != nil {
			delete(m.tasks, taskName)
			return fmt.Errorf("failed to add task to cron: %w", err)
		}
	}

	return nil
}

// Start begins executing all registered tasks
func (m *manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cron != nil {
		return fmt.Errorf("manager is already running")
	}

	// Create cron with timezone and logger
	m.cron = cron.New(
		cron.WithLocation(time.UTC),
		cron.WithLogger(cron.VerbosePrintfLogger(m.logger)),
	)

	// Add all registered tasks
	for _, task := range m.tasks {
		if err := m.addTaskToCron(task); err != nil {
			return fmt.Errorf("failed to add task '%s': %w", task.Name(), err)
		}
	}

	// Start the cron scheduler
	m.cron.Start()
	m.logger.WithField("tasks_count", len(m.tasks)).Info("Periodic task manager started")

	// Execute tasks that should run on startup
	for _, task := range m.tasks {
		if task.RunOnStart() {
			go func(t PeriodicTask) {
				// Recover from panics to prevent crashing the entire application
				defer func() {
					if r := recover(); r != nil {
						m.logger.WithFields(logrus.Fields{
							"task":  t.Name(),
							"panic": r,
						}).Error("Panic recovered in periodic task startup")
					}
				}()

				taskName := t.Name()
				m.logger.WithField("task", taskName).Info("Running periodic task on startup")

				ctx := context.Background()
				ctx = sdkcomposables.WithPool(ctx, m.pool)
				ctx = sdkcomposables.WithTenantID(ctx, m.tenantID)
				ctx = context.WithValue(ctx, sdkconstants.LoggerKey, m.logger.WithField("task", taskName))

				if err := t.Execute(ctx); err != nil {
					m.logger.WithFields(logrus.Fields{
						"task":  taskName,
						"error": err,
					}).Error("Periodic task failed on startup")
				} else {
					m.logger.WithField("task", taskName).Info("Periodic task completed successfully on startup")
				}
			}(task)
		}
	}

	return nil
}

// Stop gracefully stops all running tasks
func (m *manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cron == nil {
		return nil // Already stopped
	}

	// Create a context for graceful shutdown
	stopCtx := m.cron.Stop()

	// Wait for all jobs to complete or timeout
	select {
	case <-stopCtx.Done():
		m.logger.Info("Periodic task manager stopped gracefully")
	case <-ctx.Done():
		m.logger.Warn("Periodic task manager stop timed out, some tasks may still be running")
	}

	m.cron = nil
	m.logger.Info("Periodic task manager stopped")
	return nil
}

// IsRunning returns true if the manager is currently running
func (m *manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cron != nil
}

// GetEntries returns all registered cron entries
func (m *manager) GetEntries() []cron.Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.cron == nil {
		return []cron.Entry{}
	}

	return m.cron.Entries()
}

// addTaskToCron adds a single task to the cron scheduler
func (m *manager) addTaskToCron(task PeriodicTask) error {
	config := task.Config()
	taskName := task.Name()

	// Create the job wrapper chain
	wrappers := []cron.JobWrapper{
		// Recovery wrapper (always first)
		cron.Recover(cron.VerbosePrintfLogger(m.logger)),
	}

	// Add skip if running wrapper (if enabled)
	if config.EnableSkipIfRunning {
		wrappers = append(wrappers, cron.SkipIfStillRunning(cron.VerbosePrintfLogger(m.logger)))
	}

	// Add timeout wrapper
	wrappers = append(wrappers, TimeoutWrapper(config.Timeout, m.logger))

	// Add retry wrapper
	wrappers = append(wrappers, RetryWrapper(config.MaxRetries, config.RetryDelay, m.logger))

	// Add metrics wrapper
	wrappers = append(wrappers, MetricsWrapper(taskName, m.metrics))

	// Add logging wrapper (last, so it logs the final result)
	wrappers = append(wrappers, LoggingWrapper(taskName, m.logger))

	// Create the chain
	chain := cron.NewChain(wrappers...)

	// Create the job that will execute the task
	job := cron.FuncJob(func() {
		ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
		defer cancel()

		// Add database pool, tenant ID, and logger to context for task execution
		ctx = sdkcomposables.WithPool(ctx, m.pool)
		ctx = sdkcomposables.WithTenantID(ctx, m.tenantID)
		ctx = context.WithValue(ctx, sdkconstants.LoggerKey, m.logger.WithField("task", taskName))

		if err := task.Execute(ctx); err != nil {
			m.logger.WithFields(logrus.Fields{
				"task":  taskName,
				"error": err,
			}).Error("Periodic task failed")
			// Recovery wrapper will handle any panics
			// No need to panic here - just log the error
		}
	})

	// Wrap the job with the chain
	wrappedJob := chain.Then(job)

	// Add to cron
	entryID, err := m.cron.AddJob(task.Schedule(), wrappedJob)
	if err != nil {
		return fmt.Errorf("failed to schedule task: %w", err)
	}

	m.logger.WithFields(logrus.Fields{
		"task":     taskName,
		"schedule": task.Schedule(),
		"entry_id": entryID,
	}).Info("Periodic task scheduled")

	return nil
}

// GetMetrics returns performance metrics for all tasks
func (m *manager) GetMetrics() map[string]*TaskMetrics {
	return m.metrics.GetMetrics()
}

// LogHealthReport logs a health report for all tasks
func (m *manager) LogHealthReport() {
	m.metrics.LogHealthReport()
}
