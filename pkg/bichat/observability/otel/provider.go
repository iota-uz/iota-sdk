package otel

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Provider implements observability.Provider by emitting OpenTelemetry spans
// that follow the GenAI semantic conventions. Spans are exported via the
// configured global TracerProvider (typically set up by InitTracerProvider).
//
// Cost attributes are intentionally NOT emitted: the trace backend computes
// cost server-side from gen_ai.request.model + token counts, removing the
// entire bug class around mismatched cost columns.
type Provider struct {
	tracer trace.Tracer
	cfg    Config
	log    *logrus.Logger
}

// Option configures a Provider during construction.
type Option func(*Provider)

// WithTracer overrides the tracer used by the provider. When unset, the
// provider acquires "bichat" from the global OTel TracerProvider. This is
// primarily useful for tests with an in-memory exporter.
func WithTracer(t trace.Tracer) Option {
	return func(p *Provider) { p.tracer = t }
}

// WithLogger overrides the logger used by the provider.
func WithLogger(l *logrus.Logger) Option {
	return func(p *Provider) { p.log = l }
}

// NewProvider creates an OTel-based observability provider.
//
// If WithTracer is not supplied, the provider lazily resolves
// otel.Tracer("bichat") on each call, so callers can call NewProvider before
// InitTracerProvider as long as setup completes before any Record* calls.
func NewProvider(cfg Config, opts ...Option) *Provider {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	p := &Provider{
		cfg: cfg,
		log: log,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// resolveTracer returns the configured tracer or the global "bichat" tracer.
func (p *Provider) resolveTracer() trace.Tracer {
	if p.tracer != nil {
		return p.tracer
	}
	return otel.Tracer("bichat")
}

// deriveTraceID hashes an arbitrary string into a 16-byte OTel trace ID.
// Empty input yields the zero trace ID, which OTel treats as "no trace".
func deriveTraceID(s string) trace.TraceID {
	var tid trace.TraceID
	if s == "" {
		return tid
	}
	h := sha256.Sum256([]byte(s))
	copy(tid[:], h[:16])
	return tid
}

// deriveSpanID hashes an arbitrary string into an 8-byte OTel span ID.
// Empty input yields the zero span ID.
func deriveSpanID(s string) trace.SpanID {
	var sid trace.SpanID
	if s == "" {
		return sid
	}
	h := sha256.Sum256([]byte(s))
	copy(sid[:], h[:8])
	return sid
}

// parentContext builds a context whose embedded SpanContext carries the
// derived trace ID and (only when parentID is non-empty) a real parent span ID.
// This is what makes child spans land under the correct trace once exported.
//
// When parentID is empty, the SpanContext omits SpanID entirely so OTel treats
// the new span as a fresh root — synthesizing a fake parent span ID would
// produce orphan/broken-parent badges in backends that validate parent
// linkage, since that synthesized ID is never actually exported.
func parentContext(ctx context.Context, traceID, parentID string) context.Context {
	tid := deriveTraceID(traceID)
	if !tid.IsValid() {
		return ctx
	}
	cfg := trace.SpanContextConfig{
		TraceID:    tid,
		Remote:     true,
		TraceFlags: trace.FlagsSampled,
	}
	if parentID != "" {
		sid := deriveSpanID(parentID)
		if sid.IsValid() {
			cfg.SpanID = sid
		}
	}
	// If parentID was empty or its derived span ID was invalid (zero), leave
	// cfg.SpanID as the zero value so OTel treats this as a root span.
	sc := trace.NewSpanContext(cfg)
	return trace.ContextWithSpanContext(ctx, sc)
}

// safeEnd ends a span at obs.Timestamp + duration. End time of zero is fine
// for OTel — it falls back to the current time — but providing the recorded
// duration produces more accurate trace timelines.
func safeEnd(span trace.Span, start time.Time, dur time.Duration) {
	if start.IsZero() {
		span.End()
		return
	}
	if dur <= 0 {
		span.End()
		return
	}
	span.End(trace.WithTimestamp(start.Add(dur)))
}

// RecordGeneration emits a CLIENT-kind span with GenAI semantic-convention
// attributes. The span name is the model identifier so it renders nicely in
// trace UIs.
func (p *Provider) RecordGeneration(ctx context.Context, obs observability.GenerationObservation) error {
	if !p.cfg.Enabled {
		return nil
	}
	defer p.recover("RecordGeneration", obs.ID)

	pctx := parentContext(ctx, obs.TraceID, obs.ParentID)
	startOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindClient),
	}
	if !obs.Timestamp.IsZero() {
		startOpts = append(startOpts, trace.WithTimestamp(obs.Timestamp))
	}

	name := obs.Model
	if name == "" {
		name = "llm.generation"
	}

	_, span := p.resolveTracer().Start(pctx, name, startOpts...)
	span.SetAttributes(generationToAttributes(obs)...)
	safeEnd(span, obs.Timestamp, obs.Duration)
	return nil
}

// RecordSpan emits an INTERNAL-kind span with eai.span.* attributes.
func (p *Provider) RecordSpan(ctx context.Context, obs observability.SpanObservation) error {
	if !p.cfg.Enabled {
		return nil
	}
	defer p.recover("RecordSpan", obs.ID)

	pctx := parentContext(ctx, obs.TraceID, obs.ParentID)
	startOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindInternal),
	}
	if !obs.Timestamp.IsZero() {
		startOpts = append(startOpts, trace.WithTimestamp(obs.Timestamp))
	}

	name := obs.Name
	if name == "" {
		name = "span"
	}

	_, span := p.resolveTracer().Start(pctx, name, startOpts...)
	span.SetAttributes(spanToAttributes(obs)...)
	safeEnd(span, obs.Timestamp, obs.Duration)
	return nil
}

// RecordEvent emits a zero-duration INTERNAL-kind span with eai.event.*
// attributes. Bichat events are point-in-time signals, modeled here as spans
// that begin and end at obs.Timestamp.
func (p *Provider) RecordEvent(ctx context.Context, obs observability.EventObservation) error {
	if !p.cfg.Enabled {
		return nil
	}
	defer p.recover("RecordEvent", obs.ID)

	pctx := parentContext(ctx, obs.TraceID, "")
	startOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindInternal),
	}
	if !obs.Timestamp.IsZero() {
		startOpts = append(startOpts, trace.WithTimestamp(obs.Timestamp))
	}

	name := obs.Name
	if name == "" {
		name = "event"
	}

	_, span := p.resolveTracer().Start(pctx, name, startOpts...)
	span.SetAttributes(eventToAttributes(obs)...)
	if !obs.Timestamp.IsZero() {
		span.End(trace.WithTimestamp(obs.Timestamp))
	} else {
		span.End()
	}
	return nil
}

// RecordTrace emits a span representing the trace summary. Backends that
// understand langfuse.trace.* keys (e.g. the Langfuse OTel collector) will
// fold this into trace-level metadata.
func (p *Provider) RecordTrace(ctx context.Context, obs observability.TraceObservation) error {
	if !p.cfg.Enabled {
		return nil
	}
	defer p.recover("RecordTrace", obs.ID)

	pctx := parentContext(ctx, obs.ID, "")
	startOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindInternal),
	}
	if !obs.Timestamp.IsZero() {
		startOpts = append(startOpts, trace.WithTimestamp(obs.Timestamp))
	}

	name := obs.Name
	if name == "" {
		name = "bichat.trace"
	}

	_, span := p.resolveTracer().Start(pctx, name, startOpts...)
	attrs := traceToAttributes(obs)
	if len(p.cfg.Tags) > 0 {
		attrs = append(attrs, attribute.StringSlice(attrLangfuseTraceTags, p.cfg.Tags))
	}
	if p.cfg.Environment != "" {
		attrs = append(attrs, attribute.String("deployment.environment.name", p.cfg.Environment))
	}
	if p.cfg.Version != "" {
		attrs = append(attrs, attribute.String("service.version", p.cfg.Version))
	}
	span.SetAttributes(attrs...)
	safeEnd(span, obs.Timestamp, obs.Duration)
	return nil
}

// UpdateTraceName emits a marker span carrying langfuse.trace.name. Note that
// Langfuse's OTel ingestion has more limited trace-update semantics than its
// direct API; if propagation proves unreliable in production, switching to
// direct API for these updates is a follow-up.
func (p *Provider) UpdateTraceName(ctx context.Context, traceID, name string) error {
	if !p.cfg.Enabled {
		return nil
	}
	defer p.recover("UpdateTraceName", traceID)

	pctx := parentContext(ctx, traceID, "")
	_, span := p.resolveTracer().Start(pctx, "bichat.trace.update",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	span.SetAttributes(
		attribute.String(attrLangfuseUpdateKind, "name"),
		attribute.String(attrLangfuseTraceName, name),
	)
	span.End()
	return nil
}

// UpdateTraceTags emits a marker span carrying langfuse.trace.tags. See the
// note on UpdateTraceName regarding Langfuse OTel update semantics.
func (p *Provider) UpdateTraceTags(ctx context.Context, traceID string, tags []string) error {
	if !p.cfg.Enabled {
		return nil
	}
	defer p.recover("UpdateTraceTags", traceID)

	pctx := parentContext(ctx, traceID, "")
	_, span := p.resolveTracer().Start(pctx, "bichat.trace.update",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	span.SetAttributes(
		attribute.String(attrLangfuseUpdateKind, "tags"),
		attribute.StringSlice(attrLangfuseTraceTags, tags),
	)
	span.End()
	return nil
}

// recover swallows panics from span creation/export. Observability must never
// break the app.
func (p *Provider) recover(op, id string) {
	if r := recover(); r != nil {
		p.log.Errorf("otel: %s panic for id=%s: %v", op, id, r)
	}
}
