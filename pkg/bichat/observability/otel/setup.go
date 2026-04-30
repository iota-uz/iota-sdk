package otel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// noopShutdown is a shutdown function that does nothing and returns nil.
// Used when observability is disabled.
func noopShutdown(context.Context) error { return nil }

// HasGlobalTracerProvider reports whether a non-default global TracerProvider
// has been registered (i.e. someone called otel.SetTracerProvider with a real
// provider). Returns false when the global is still the zero-value no-op
// TracerProvider that OTel installs by default — in that case spans created
// by Provider would be silently dropped.
//
// Use this from a host module's bootstrap to detect "endpoint configured but
// nobody initialized the pipeline" misconfigurations and warn loudly.
func HasGlobalTracerProvider() bool {
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		return true
	}
	return false
}

// InitTracerProvider sets up a global OpenTelemetry TracerProvider with an
// OTLP/HTTP exporter pointed at cfg.Endpoint, batched span processing, and
// resource attributes describing the bichat service.
//
// Resource attributes set:
//   - service.name = "bichat"
//   - service.version = cfg.Version (when non-empty)
//   - deployment.environment.name = cfg.Environment (when non-empty)
//
// The returned shutdown function drains pending spans and should be invoked
// from the component shutdown path (typically with a bounded context).
//
// When cfg.Enabled is false or cfg.Endpoint is empty, this function returns a
// no-op shutdown and does NOT mutate the global tracer provider.
func InitTracerProvider(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		return noopShutdown, nil
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("otel.InitTracerProvider: invalid config: %w", err)
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(cfg.Endpoint),
		otlptracehttp.WithHeaders(cfg.Headers),
	)
	if err != nil {
		return nil, fmt.Errorf("otel.InitTracerProvider: build exporter: %w", err)
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

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, resAttrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("otel.InitTracerProvider: build resource: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
