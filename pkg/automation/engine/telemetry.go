package engine

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// metrics are the engine's instruments.
type metrics struct {
	started  metric.Int64Counter
	finished metric.Int64Counter
	step     metric.Float64Histogram
	lag      metric.Float64Histogram

	calls      metric.Int64Counter
	callDur    metric.Float64Histogram
	inbound    metric.Int64Counter
	pollErrors metric.Int64Counter
}

func newMetrics(m metric.Meter) (metrics, error) {
	var out metrics
	var err error
	if out.started, err = m.Int64Counter("automation_runs_started_total", metric.WithDescription("Runs of automation graphs started")); err != nil {
		return out, err
	}
	if out.finished, err = m.Int64Counter("automation_runs_finished_total", metric.WithDescription("Runs of automation graphs finished, by status")); err != nil {
		return out, err
	}
	if out.step, err = m.Float64Histogram("automation_step_duration_seconds", metric.WithDescription("Duration of one step of a run, by behavior"), metric.WithUnit("s")); err != nil {
		return out, err
	}
	if out.lag, err = m.Float64Histogram("automation_event_lag_seconds", metric.WithDescription("Time from an event's publication to its fan-out"), metric.WithUnit("s")); err != nil {
		return out, err
	}
	if out.calls, err = m.Int64Counter("automation_calls_total", metric.WithDescription("Calls of call nodes, by connector, node type and result (ok, failed, outcome_unknown, retried)")); err != nil {
		return out, err
	}
	if out.callDur, err = m.Float64Histogram("automation_call_duration_seconds", metric.WithDescription("Duration of one attempt of a call, by connector"), metric.WithUnit("s")); err != nil {
		return out, err
	}
	if out.inbound, err = m.Int64Counter("automation_inbound_events_total", metric.WithDescription("Inbound events of connectors, by connector and result (published, duplicate, rejected)")); err != nil {
		return out, err
	}
	if out.pollErrors, err = m.Int64Counter("automation_poll_errors_total", metric.WithDescription("Failed polls of inbound connectors, by connector")); err != nil {
		return out, err
	}
	return out, nil
}

func (m metrics) call(ctx context.Context, connector, node, result string) {
	m.calls.Add(ctx, 1, metric.WithAttributes(attribute.String("connector", connector), attribute.String("node", node), attribute.String("result", result)))
}

func (m metrics) callDuration(ctx context.Context, connector string, d time.Duration) {
	m.callDur.Record(ctx, d.Seconds(), metric.WithAttributes(attribute.String("connector", connector)))
}

func (m metrics) inboundEvent(ctx context.Context, connector, result string) {
	m.inbound.Add(ctx, 1, metric.WithAttributes(attribute.String("connector", connector), attribute.String("result", result)))
}

func (m metrics) pollError(ctx context.Context, connector string) {
	m.pollErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("connector", connector)))
}

func (m metrics) runStarted(ctx context.Context, trigger string) {
	m.started.Add(ctx, 1, metric.WithAttributes(attribute.String("trigger", trigger)))
}

func (m metrics) runFinished(ctx context.Context, status RunStatus) {
	m.finished.Add(ctx, 1, metric.WithAttributes(attribute.String("status", string(status))))
}

func (m metrics) stepDone(ctx context.Context, behavior string, d time.Duration) {
	m.step.Record(ctx, d.Seconds(), metric.WithAttributes(attribute.String("behavior", behavior)))
}

func (m metrics) eventLag(ctx context.Context, occurred time.Time) {
	if !occurred.IsZero() {
		m.lag.Record(ctx, time.Since(occurred).Seconds())
	}
}
