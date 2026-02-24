package hooks

import "log"

// MetricsRecorder provides metric collection for observability.
// Implementations can use Prometheus, StatsD, OpenTelemetry, etc.
type MetricsRecorder interface {
	// IncrementCounter increments a counter metric.
	//
	// Parameters:
	//   - name: Metric name (e.g., "bichat.requests.total")
	//   - value: Amount to increment by (typically 1)
	//   - labels: Key-value pairs for metric dimensions
	//
	// Example:
	//
	//	metrics.IncrementCounter("bichat.llm.requests", 1, map[string]string{
	//	    "model": "gpt-5.2",
	//	    "status": "success",
	//	})
	IncrementCounter(name string, value int64, labels map[string]string)

	// RecordGauge sets a gauge metric to a specific value.
	// Gauges can go up or down (e.g., queue depth, active connections).
	//
	// Parameters:
	//   - name: Metric name (e.g., "bichat.queue.depth")
	//   - value: Current value of the gauge
	//   - labels: Key-value pairs for metric dimensions
	//
	// Example:
	//
	//	metrics.RecordGauge("bichat.queue.depth", 42, map[string]string{
	//	    "queue": "async_events",
	//	})
	RecordGauge(name string, value float64, labels map[string]string)

	// RecordHistogram records a value in a histogram (for distributions).
	// Useful for timing data, response sizes, etc.
	//
	// Parameters:
	//   - name: Metric name (e.g., "bichat.llm.latency")
	//   - value: Observed value (e.g., duration in seconds)
	//   - labels: Key-value pairs for metric dimensions
	//
	// Example:
	//
	//	metrics.RecordHistogram("bichat.llm.latency", 1.23, map[string]string{
	//	    "model": "gpt-5.2",
	//	})
	RecordHistogram(name string, value float64, labels map[string]string)
}

// NoOpMetricsRecorder is a metrics recorder that does nothing.
// Useful as a default when metrics collection is disabled.
type NoOpMetricsRecorder struct{}

// NewNoOpMetricsRecorder creates a no-op metrics recorder.
func NewNoOpMetricsRecorder() MetricsRecorder {
	return &NoOpMetricsRecorder{}
}

// IncrementCounter does nothing.
func (m *NoOpMetricsRecorder) IncrementCounter(name string, value int64, labels map[string]string) {}

// RecordGauge does nothing.
func (m *NoOpMetricsRecorder) RecordGauge(name string, value float64, labels map[string]string) {}

// RecordHistogram does nothing.
func (m *NoOpMetricsRecorder) RecordHistogram(name string, value float64, labels map[string]string) {}

// StdMetricsRecorder logs metrics to stdout for debugging.
// Not suitable for production - use Prometheus/OpenTelemetry instead.
type StdMetricsRecorder struct{}

// NewStdMetricsRecorder creates a metrics recorder that prints to stdout.
func NewStdMetricsRecorder() MetricsRecorder {
	return &StdMetricsRecorder{}
}

// IncrementCounter records counter increment.
func (m *StdMetricsRecorder) IncrementCounter(name string, value int64, labels map[string]string) {
	log.Print("[METRIC] counter ", name, " + ", value, formatLabels(labels))
}

// RecordGauge records gauge value.
func (m *StdMetricsRecorder) RecordGauge(name string, value float64, labels map[string]string) {
	log.Print("[METRIC] gauge ", name, " = ", value, formatLabels(labels))
}

// RecordHistogram records histogram observation.
func (m *StdMetricsRecorder) RecordHistogram(name string, value float64, labels map[string]string) {
	log.Print("[METRIC] histogram ", name, " observed ", value, formatLabels(labels))
}

// formatLabels formats label map as string.
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	result := "{"
	first := true
	for k, v := range labels {
		if !first {
			result += ", "
		}
		result += k + "=" + v
		first = false
	}
	result += "}"
	return result
}
