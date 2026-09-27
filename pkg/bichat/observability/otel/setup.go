package otel

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
)

// noopShutdown is a shutdown function that does nothing and returns nil.
// Used when observability is disabled.
func noopShutdown(context.Context) error { return nil }

// InitTracerProvider builds a SCOPED OpenTelemetry TracerProvider for bichat.
// It is intentionally NOT installed as the global TracerProvider — doing so
// would hijack any other OTel-instrumented library in the same process
// (otelhttp middleware, sql instrumentation, etc.) and ship their spans to
// the same OTLP endpoint, polluting bichat's trace dashboard.
//
// Callers should pass the returned TracerProvider's tracer to NewProvider via
// WithTracer:
//
//	tp, shutdown, err := otelprovider.InitTracerProvider(ctx, cfg)
//	if err != nil { ... }
//	defer shutdown(...)
//	prov := otelprovider.NewProvider(cfg, otelprovider.WithTracer(tp.Tracer("bichat")))
//
// Resource attributes set:
//   - service.name = "bichat"
//   - service.version = cfg.Version (when non-empty)
//   - deployment.environment.name = cfg.Environment (when non-empty)
//   - eai.tag.<i> = each value in cfg.Tags
//
// The returned shutdown function drains pending spans and should be invoked
// from the component shutdown path (typically with a bounded context).
//
// When cfg.Enabled is false or cfg.Endpoint is empty, this function returns a
// nil TracerProvider and a no-op shutdown.
func InitTracerProvider(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		return nil, noopShutdown, nil
	}

	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("otel.InitTracerProvider: invalid config: %w", err)
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(cfg.Endpoint),
		otlptracehttp.WithHeaders(cfg.Headers),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("otel.InitTracerProvider: build exporter: %w", err)
	}

	resAttrs := []attribute.KeyValue{
		semconv.ServiceName("bichat"),
	}
	if cfg.Version != "" {
		resAttrs = append(resAttrs, semconv.ServiceVersion(cfg.Version))
	}
	if cfg.Environment != "" {
		resAttrs = append(resAttrs, semconv.DeploymentEnvironmentName(cfg.Environment))
	}
	for i, tag := range cfg.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		resAttrs = append(resAttrs, attribute.String(fmt.Sprintf("eai.tag.%d", i), tag))
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, resAttrs...),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("otel.InitTracerProvider: build resource: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
	)
	// NOTE: deliberately not calling otel.SetTracerProvider(tp). Bichat owns
	// this TP. Host applications that want a global tracer can configure one
	// independently.
	return tp, tp.Shutdown, nil
}
