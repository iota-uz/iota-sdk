package serve

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type executionMetrics struct {
	firstKPI    metric.Float64Histogram
	firstChild  metric.Float64Histogram
	prefetchHit metric.Float64Histogram
	cancelled   metric.Int64Counter
	redundant   metric.Int64Counter
	saturation  metric.Float64Histogram
	queueWait   metric.Float64Histogram
	execution   metric.Float64Histogram
}

var (
	executionMetricsOnce  sync.Once
	executionMetricsValue executionMetrics
)

func lensExecutionMetrics() executionMetrics {
	executionMetricsOnce.Do(func() {
		meter := otel.Meter("github.com/iota-uz/iota-sdk/lens/serve")
		executionMetricsValue.firstKPI, _ = meter.Float64Histogram("lens.time_to_first_useful_kpi", metric.WithUnit("ms"))
		executionMetricsValue.firstChild, _ = meter.Float64Histogram("lens.time_to_first_child_frame", metric.WithUnit("ms"))
		executionMetricsValue.prefetchHit, _ = meter.Float64Histogram("lens.prefetch_hit", metric.WithUnit("1"))
		executionMetricsValue.cancelled, _ = meter.Int64Counter("lens.cancelled_speculative_work")
		executionMetricsValue.redundant, _ = meter.Int64Counter("lens.redundant_work")
		executionMetricsValue.saturation, _ = meter.Float64Histogram("lens.scheduler_saturation", metric.WithUnit("1"))
		executionMetricsValue.queueWait, _ = meter.Float64Histogram("lens.queue_wait", metric.WithUnit("ms"))
		executionMetricsValue.execution, _ = meter.Float64Histogram("lens.execution_duration", metric.WithUnit("ms"))
	})
	return executionMetricsValue
}

func recordMetric(ctx context.Context, value Metric) {
	metrics := lensExecutionMetrics()
	attributes := make([]attribute.KeyValue, 0, len(value.Labels))
	for key, label := range value.Labels {
		attributes = append(attributes, attribute.String(key, label))
	}
	options := metric.WithAttributes(attributes...)
	switch value.Name {
	case MetricTimeToFirstUsefulKPI:
		metrics.firstKPI.Record(ctx, value.Value, options)
	case MetricTimeToFirstChild:
		metrics.firstChild.Record(ctx, value.Value, options)
	case MetricPrefetchHit:
		metrics.prefetchHit.Record(ctx, value.Value, options)
	case MetricCancelledSpeculation:
		metrics.cancelled.Add(ctx, int64(value.Value), options)
	case MetricRedundantWork:
		metrics.redundant.Add(ctx, int64(value.Value), options)
	case MetricSchedulerSaturation:
		metrics.saturation.Record(ctx, value.Value, options)
	case MetricQueueWait:
		metrics.queueWait.Record(ctx, value.Value, options)
	case MetricExecutionDuration:
		metrics.execution.Record(ctx, value.Value, options)
	}
}
