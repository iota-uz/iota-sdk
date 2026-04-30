package otel

import (
	"context"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Provider implements observability.Provider by emitting OpenTelemetry spans
// that follow the GenAI semantic conventions. The provider is intended to be
// constructed against a SCOPED TracerProvider (created via InitTracerProvider
// and passed in via WithTracer), not the OTel global — this keeps bichat's
// pipeline isolated from any other instrumented library in the same process.
//
// Cost attributes are intentionally NOT emitted: the trace backend computes
// cost server-side from gen_ai.request.model + token counts, removing the
// entire bug class around mismatched cost columns.
type Provider struct {
	tracer trace.Tracer
	cfg    Config
	log    *logrus.Logger
	// spans maps bichat span/trace IDs (the strings on GenerationObservation/
	// SpanObservation/EventObservation) to the real OTel SpanContext that the
	// SDK assigned to the underlying span. Subsequent observations whose
	// ParentID points at a bichat ID look up the captured SpanContext here so
	// the new span correctly inherits TraceID + parent linkage. Without this
	// map the only options would be (a) install a custom IDGenerator on the
	// TracerProvider or (b) propagate via ctx — neither fits the bridge,
	// which builds bichat-internal IDs and passes them as plain strings.
	//
	// TODO: bound size / evict on session end. Currently grows for the
	// process lifetime; fine for short-lived test runs and the typical
	// dev-server workload, but a long-running production node will leak
	// ~100 bytes per recorded span.
	spans sync.Map // map[string]trace.SpanContext
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
// Returns observability.Provider rather than the concrete *Provider so the
// package boundary stays decoupled from this backend (see CLAUDE.md:
// "Prefer interfaces over concrete structs at boundaries"). Callers that
// need access to the additional TraceNameUpdater / TraceTagUpdater
// interfaces can type-assert against those.
//
// If WithTracer is not supplied, the provider lazily resolves
// otel.Tracer("bichat") on each call. NOTE: with the migration to a SCOPED
// TracerProvider (see InitTracerProvider), the global is typically the OTel
// noop and spans would be silently dropped — host bootstraps should ALWAYS
// pass WithTracer(tp.Tracer("bichat")) using the TracerProvider returned by
// InitTracerProvider.
func NewProvider(cfg Config, opts ...Option) observability.Provider {
	return newProvider(cfg, opts...)
}

// newProvider is the package-internal constructor used by tests that need to
// access the concrete *Provider type (e.g. for direct field inspection or
// type-asserted helpers).
func newProvider(cfg Config, opts ...Option) *Provider {
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

// rememberSpan stores the OTel SpanContext of a freshly-started span keyed by
// its bichat ID, so subsequent observations whose ParentID matches can find
// the real parent SpanContext and inherit TraceID + parent linkage.
func (p *Provider) rememberSpan(bichatID string, sc trace.SpanContext) {
	if bichatID == "" || !sc.IsValid() {
		return
	}
	p.spans.Store(bichatID, sc)
}

// lookupSpan returns the captured SpanContext for a bichat ID, or zero+false.
func (p *Provider) lookupSpan(bichatID string) (trace.SpanContext, bool) {
	if bichatID == "" {
		return trace.SpanContext{}, false
	}
	v, ok := p.spans.Load(bichatID)
	if !ok {
		return trace.SpanContext{}, false
	}
	sc, ok := v.(trace.SpanContext)
	return sc, ok
}

// parentContext returns a context that embeds the parent's REAL OTel
// SpanContext (TraceID + SpanID) when we've already recorded that bichat ID.
// When parentID is empty or unknown, the original ctx is returned and the new
// span becomes a root — which is correct for the first observation of a run.
//
// Earlier iterations of this provider hashed bichat IDs into synthetic OTel
// IDs to thread a deterministic TraceID through. That produced orphaned
// parent links: the synthetic SpanIDs never matched any span actually
// exported by the SDK, and OTel's tracer.Start treats a SpanContext with a
// zero/invalid SpanID as "no parent" anyway, falling back to a fresh
// TraceID. The state map is the only correct way to bridge bichat's
// string-ID world into OTel's binary-ID world without a custom IDGenerator.
func (p *Provider) parentContext(ctx context.Context, parentID string) context.Context {
	if parentID == "" {
		return ctx
	}
	sc, ok := p.lookupSpan(parentID)
	if !ok {
		return ctx
	}
	return trace.ContextWithSpanContext(ctx, sc)
}

// safeEndOptions returns the End() options that pin a span to its recorded
// timestamp+duration. When duration is zero/negative we still end at the
// recorded start timestamp so historical observations don't mutate into
// long-running spans (which would skew latency dashboards). A zero start
// timestamp returns nil so the call site falls back to OTel's "now" default.
//
// Returning options instead of calling span.End() directly lets each call
// site invoke span.End(...) inline — keeps the spancheck linter happy and
// makes the lifecycle obvious to readers.
func safeEndOptions(start time.Time, dur time.Duration) []trace.SpanEndOption {
	if start.IsZero() {
		return nil
	}
	end := start
	if dur > 0 {
		end = start.Add(dur)
	}
	return []trace.SpanEndOption{trace.WithTimestamp(end)}
}

// RecordGeneration emits a CLIENT-kind span with GenAI semantic-convention
// attributes. The span name is the model identifier so it renders nicely in
// trace UIs.
func (p *Provider) RecordGeneration(ctx context.Context, obs observability.GenerationObservation) error {
	if !p.cfg.Enabled {
		return nil
	}
	defer p.recover("RecordGeneration", obs.ID)

	pctx := p.parentContext(ctx, obs.ParentID)
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
	p.rememberSpan(obs.ID, span.SpanContext())
	span.SetAttributes(generationToAttributes(obs)...)
	span.End(safeEndOptions(obs.Timestamp, obs.Duration)...)
	return nil
}

// RecordSpan emits an INTERNAL-kind span with eai.span.* attributes.
func (p *Provider) RecordSpan(ctx context.Context, obs observability.SpanObservation) error {
	if !p.cfg.Enabled {
		return nil
	}
	defer p.recover("RecordSpan", obs.ID)

	pctx := p.parentContext(ctx, obs.ParentID)
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
	p.rememberSpan(obs.ID, span.SpanContext())
	span.SetAttributes(spanToAttributes(obs)...)
	// Mirror the bichat status into the OTel standard. Backends like Jaeger
	// and Datadog use SetStatus(codes.Error, ...) to drive error-rate
	// metrics; a custom eai.span.status attribute alone is invisible there.
	if obs.Status == "error" {
		span.SetStatus(codes.Error, obs.Output)
	}
	span.End(safeEndOptions(obs.Timestamp, obs.Duration)...)
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

	pctx := p.parentContext(ctx, obs.TraceID)
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
	p.rememberSpan(obs.ID, span.SpanContext())
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

	pctx := p.parentContext(ctx, obs.ID)
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
	p.rememberSpan(obs.ID, span.SpanContext())
	// Resource attrs (service.version, deployment.environment.name, eai.tag.*)
	// are now set on the TracerProvider's Resource and propagate to every
	// span automatically — duplicating them on each trace summary is dead
	// noise for any consumer that respects Resource attrs (every standard
	// OTel sink does). Tags and metadata that are genuinely per-trace stay.
	span.SetAttributes(traceToAttributes(obs)...)
	span.End(safeEndOptions(obs.Timestamp, obs.Duration)...)
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

	pctx := p.parentContext(ctx, traceID)
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

	pctx := p.parentContext(ctx, traceID)
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
