package periodics

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskMetrics holds metrics for periodic task execution
type TaskMetrics struct {
	TaskName         string        `json:"task_name"`
	LastRun          time.Time     `json:"last_run"`
	LastSuccess      time.Time     `json:"last_success"`
	LastError        time.Time     `json:"last_error"`
	LastErrorMessage string        `json:"last_error_message,omitempty"`
	TotalRuns        int64         `json:"total_runs"`
	SuccessfulRuns   int64         `json:"successful_runs"`
	FailedRuns       int64         `json:"failed_runs"`
	AverageRuntime   time.Duration `json:"average_runtime"`
	LastRuntime      time.Duration `json:"last_runtime"`
	IsRunning        bool          `json:"is_running"`
}

// MetricsCollector collects and manages metrics for periodic tasks
type MetricsCollector struct {
	metrics map[string]*TaskMetrics
	mu      sync.RWMutex
	logger  *logrus.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger *logrus.Logger) *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]*TaskMetrics),
		logger:  logger,
	}
}

// RecordTaskStart records when a task starts execution
func (mc *MetricsCollector) RecordTaskStart(taskName string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.metrics[taskName] == nil {
		mc.metrics[taskName] = &TaskMetrics{
			TaskName: taskName,
		}
	}

	metrics := mc.metrics[taskName]
	metrics.IsRunning = true
	metrics.LastRun = time.Now()
	metrics.TotalRuns++
}

// RecordTaskSuccess records when a task completes successfully
func (mc *MetricsCollector) RecordTaskSuccess(taskName string, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics := mc.metrics[taskName]
	if metrics == nil {
		return
	}

	metrics.IsRunning = false
	metrics.LastSuccess = time.Now()
	metrics.SuccessfulRuns++
	metrics.LastRuntime = duration

	// Update average runtime
	if metrics.SuccessfulRuns == 1 {
		metrics.AverageRuntime = duration
	} else {
		// Simple moving average
		metrics.AverageRuntime = (metrics.AverageRuntime*time.Duration(metrics.SuccessfulRuns-1) + duration) / time.Duration(metrics.SuccessfulRuns)
	}

	mc.logger.WithFields(logrus.Fields{
		"task":          taskName,
		"duration":      duration,
		"success_count": metrics.SuccessfulRuns,
		"total_runs":    metrics.TotalRuns,
		"success_rate":  float64(metrics.SuccessfulRuns) / float64(metrics.TotalRuns) * 100,
	}).Info("Periodic task completed successfully")
}

// RecordTaskFailure records when a task fails
func (mc *MetricsCollector) RecordTaskFailure(taskName string, duration time.Duration, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics := mc.metrics[taskName]
	if metrics == nil {
		return
	}

	metrics.IsRunning = false
	metrics.LastError = time.Now()
	metrics.LastErrorMessage = err.Error()
	metrics.FailedRuns++
	metrics.LastRuntime = duration

	successRate := float64(metrics.SuccessfulRuns) / float64(metrics.TotalRuns) * 100

	mc.logger.WithFields(logrus.Fields{
		"task":         taskName,
		"duration":     duration,
		"error":        err.Error(),
		"failed_count": metrics.FailedRuns,
		"total_runs":   metrics.TotalRuns,
		"success_rate": successRate,
	}).Error("Periodic task failed")

	// Alert if success rate drops below threshold
	if metrics.TotalRuns >= 10 && successRate < 80 {
		mc.logger.WithFields(logrus.Fields{
			"task":         taskName,
			"success_rate": successRate,
		}).Warn("Periodic task success rate is below acceptable threshold (80%)")
	}
}

// GetMetrics returns current metrics for all tasks
func (mc *MetricsCollector) GetMetrics() map[string]*TaskMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*TaskMetrics)
	for name, metrics := range mc.metrics {
		// Create a copy to avoid race conditions
		metricsCopy := *metrics
		result[name] = &metricsCopy
	}
	return result
}

// GetTaskMetrics returns metrics for a specific task
func (mc *MetricsCollector) GetTaskMetrics(taskName string) *TaskMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if metrics := mc.metrics[taskName]; metrics != nil {
		metricsCopy := *metrics
		return &metricsCopy
	}
	return nil
}

// LogHealthReport logs a health report for all tasks
func (mc *MetricsCollector) LogHealthReport() {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	for taskName, metrics := range mc.metrics {
		var status string
		successRate := float64(0)

		if metrics.TotalRuns > 0 {
			successRate = float64(metrics.SuccessfulRuns) / float64(metrics.TotalRuns) * 100
		}

		switch {
		case metrics.IsRunning:
			status = "RUNNING"
		case successRate >= 95:
			status = "HEALTHY"
		case successRate >= 80:
			status = "WARNING"
		default:
			status = "CRITICAL"
		}

		timeSinceLastRun := time.Since(metrics.LastRun)
		timeSinceLastSuccess := time.Since(metrics.LastSuccess)

		mc.logger.WithFields(logrus.Fields{
			"task":                    taskName,
			"status":                  status,
			"total_runs":              metrics.TotalRuns,
			"successful_runs":         metrics.SuccessfulRuns,
			"failed_runs":             metrics.FailedRuns,
			"success_rate":            successRate,
			"average_runtime":         metrics.AverageRuntime,
			"last_runtime":            metrics.LastRuntime,
			"time_since_last_run":     timeSinceLastRun,
			"time_since_last_success": timeSinceLastSuccess,
			"last_error":              metrics.LastErrorMessage,
		}).Info("Periodic task health report")
	}
}
