package periodics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// manager implements the Manager interface
type manager struct {
	cron          *cron.Cron
	logger        *logrus.Logger
	metrics       *MetricsCollector
	pool          *pgxpool.Pool
	tenantID      uuid.UUID
	mu            sync.RWMutex
	tasks         map[string]PeriodicTask
	taskEntryIDs  map[string]cron.EntryID
	disabledTasks []RegisteredTask
	runsMu        sync.Mutex
	runs          map[string]*managedRun
	activeRuns    map[string][]*managedRun
	completedRuns []string
	stopping      bool
}

// NewManager creates a new periodic task manager
func NewManager(logger *logrus.Logger, pool *pgxpool.Pool, tenantID uuid.UUID) Manager {
	return &manager{
		cron:         nil, // Will be initialized in Start()
		logger:       logger,
		metrics:      NewMetricsCollector(logger),
		pool:         pool,
		tenantID:     tenantID,
		tasks:        make(map[string]PeriodicTask),
		taskEntryIDs: make(map[string]cron.EntryID),
		runs:         make(map[string]*managedRun),
		activeRuns:   make(map[string][]*managedRun),
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
	m.runsMu.Lock()
	m.stopping = false
	m.runsMu.Unlock()

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

	// Startup and cron use the same owned invocation path.
	for _, task := range m.tasks {
		if task.RunOnStart() {
			executor := m.buildWrappedExecutor(task)
			go func(taskName string, exec func()) {
				defer func() {
					if r := recover(); r != nil {
						m.logger.WithFields(logrus.Fields{
							"task":  taskName,
							"panic": r,
						}).Error("Startup task panicked")
					}
				}()
				m.logger.WithField("task", taskName).Info("Running periodic task on startup")
				exec()
			}(task.Name(), executor)
		}
	}

	return nil
}

// Stop gracefully stops all running tasks
func (m *manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	var stopCtx context.Context
	if m.cron != nil {
		stopCtx = m.cron.Stop()
	}
	m.cron = nil
	m.runsMu.Lock()
	m.stopping = true
	var pending []<-chan struct{}
	for _, runs := range m.activeRuns {
		for _, run := range runs {
			run.cancel()
			pending = append(pending, run.done)
		}
	}
	m.runsMu.Unlock()
	m.mu.Unlock()
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if stopCtx != nil {
		select {
		case <-stopCtx.Done():
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// IsRunning returns true if the manager is currently running
func (m *manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cron != nil
}

// GetEntries returns all registered scheduled entries
func (m *manager) GetEntries() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.cron == nil {
		return []Entry{}
	}

	cronEntries := m.cron.Entries()
	entries := make([]Entry, len(cronEntries))
	for i, e := range cronEntries {
		entries[i] = Entry{
			ID:   int(e.ID),
			Next: e.Next,
			Prev: e.Prev,
		}
	}
	return entries
}

// buildWrappedExecutor waits for the owned invocation, so cron shutdown joins
// actual execution even when the task ignores its deadline.
func (m *manager) buildWrappedExecutor(task PeriodicTask) func() {
	return func() {
		cfg := mergeWithDefaults(task.Config())
		receipt, err := m.startRun(task.Name(), !*cfg.EnableSkipIfRunning)
		if err != nil {
			m.logger.WithError(err).WithField("task", task.Name()).Debug("Periodic invocation skipped")
			return
		}
		_, _ = m.WaitRun(context.Background(), receipt.ID)
	}
}

// addTaskToCron adds a single task to the cron scheduler
func (m *manager) addTaskToCron(task PeriodicTask) error {
	executor := m.buildWrappedExecutor(task)

	// Add to cron
	entryID, err := m.cron.AddJob(task.Schedule(), cron.FuncJob(executor))
	if err != nil {
		return fmt.Errorf("failed to schedule task: %w", err)
	}

	m.taskEntryIDs[task.Name()] = entryID

	m.logger.WithFields(logrus.Fields{
		"task":     task.Name(),
		"schedule": task.Schedule(),
		"entry_id": entryID,
	}).Info("Periodic task scheduled")

	return nil
}

// GetTaskScheduleInfo returns scheduling information for all tasks, keyed by task name.
func (m *manager) GetTaskScheduleInfo() map[string]TaskScheduleInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]TaskScheduleInfo)
	if m.cron == nil {
		return result
	}

	entries := m.cron.Entries()
	entryMap := make(map[cron.EntryID]cron.Entry)
	for _, e := range entries {
		entryMap[e.ID] = e
	}

	for taskName, entryID := range m.taskEntryIDs {
		if e, ok := entryMap[entryID]; ok {
			result[taskName] = TaskScheduleInfo{
				Next: e.Next,
				Prev: e.Prev,
			}
		}
	}
	return result
}

// SubscribeMetrics returns a channel that receives events when task metrics change,
// and an unsubscribe function to stop receiving events and close the channel.
func (m *manager) SubscribeMetrics() (<-chan TaskMetricEvent, func()) {
	return m.metrics.Subscribe()
}

// GetMetrics returns performance metrics for all tasks
func (m *manager) GetMetrics() map[string]*TaskMetrics {
	return m.metrics.GetMetrics()
}

// LogHealthReport logs a health report for all tasks
func (m *manager) LogHealthReport() {
	m.metrics.LogHealthReport()
}

// AddDisabledTaskInfo registers metadata for a disabled task so it appears in monitoring.
// It silently ignores duplicates and conflicts with already-enabled tasks.
func (m *manager) AddDisabledTaskInfo(name, schedule string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for conflict with enabled tasks
	if _, exists := m.tasks[name]; exists {
		m.logger.WithField("task", name).Warn("AddDisabledTaskInfo called for an already-enabled task, ignoring")
		return
	}

	// Check for duplicate in disabled tasks
	for _, dt := range m.disabledTasks {
		if dt.Name == name {
			m.logger.WithField("task", name).Warn("AddDisabledTaskInfo called with duplicate name, ignoring")
			return
		}
	}

	m.disabledTasks = append(m.disabledTasks, RegisteredTask{
		Name:     name,
		Schedule: schedule,
		Enabled:  false,
	})
}

// RunTask executes a registered task immediately (out-of-schedule).
func (m *manager) RunTask(name string) error {
	_, err := m.RunTaskWithReceipt(name)
	return err
}

// GetRegisteredTasks returns information about all registered tasks (both enabled and disabled).
func (m *manager) GetRegisteredTasks() []RegisteredTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]RegisteredTask, 0, len(m.tasks)+len(m.disabledTasks))
	for _, task := range m.tasks {
		tasks = append(tasks, RegisteredTask{
			Name:       task.Name(),
			Schedule:   task.Schedule(),
			RunOnStart: task.RunOnStart(),
			Enabled:    true,
		})
	}
	tasks = append(tasks, m.disabledTasks...)
	return tasks
}
