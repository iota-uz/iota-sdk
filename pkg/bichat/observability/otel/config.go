// Package otel provides an observability.Provider implementation that emits
// OpenTelemetry spans following the GenAI semantic conventions.
//
// Spans are exported via OTLP/HTTP to a configured endpoint (e.g. Langfuse's
// /api/public/otel/v1/traces) where backend-side cost computation and trace
// reconstruction take place. This package intentionally does NOT emit any
// *_cost attributes — costs are computed server-side from
// gen_ai.request.model + token counts.
package otel

import (
	"encoding/base64"
	"errors"
	"os"
)

// Config holds configuration for the OTel observability provider.
type Config struct {
	// Endpoint is the full OTLP/HTTP traces endpoint URL.
	// Example: "https://cloud.langfuse.com/api/public/otel/v1/traces".
	// Required when Enabled is true.
	Endpoint string

	// Headers are sent with every OTLP/HTTP export request.
	// Typically includes an Authorization header for the OTel collector.
	// Example: {"Authorization": "Basic <base64(public:secret)>"}.
	Headers map[string]string

	// SampleRate controls span head sampling (0.0–1.0).
	// 1.0 = 100%, 0.0 = drop everything. Defaults to 1.0 when unset.
	SampleRate float64

	// Environment identifies the deployment environment (e.g. "production").
	// Emitted as the deployment.environment.name resource attribute.
	Environment string

	// Version identifies the application version (e.g. git commit).
	// Emitted as the service.version resource attribute.
	Version string

	// Tags are global tags applied as resource attributes (eai.tag.<i>).
	Tags []string

	// Enabled controls whether observability is active.
	// When false, all Record* methods are no-ops and InitTracerProvider returns
	// a no-op shutdown without touching the global tracer provider.
	Enabled bool
}

// Validate checks the configuration and applies defaults in-place.
// It returns an error only when Enabled is true and Endpoint is empty.
//
// Defaults applied:
//   - SampleRate: 1.0 when unset (zero value).
//   - Enabled:    a zero-value Config is treated as enabled by callers that
//     explicitly opt in; Validate does not flip Enabled itself.
func (c *Config) Validate() error {
	if c.SampleRate == 0 {
		c.SampleRate = 1.0
	}
	if c.SampleRate < 0.0 || c.SampleRate > 1.0 {
		return errors.New("otel: SampleRate must be between 0.0 and 1.0")
	}
	if c.Enabled && c.Endpoint == "" {
		return errors.New("otel: Endpoint is required when Enabled is true")
	}
	return nil
}

// LangfuseAuthHeaders reads LANGFUSE_PUBLIC_KEY and LANGFUSE_SECRET_KEY from
// the environment and returns the OTLP/HTTP headers needed to authenticate
// against Langfuse's /api/public/otel endpoint:
//
//	{"Authorization": "Basic <base64(public:secret)>"}
//
// Returns an empty (non-nil) map if either environment variable is missing.
func LangfuseAuthHeaders() map[string]string {
	pub := os.Getenv("LANGFUSE_PUBLIC_KEY")
	sec := os.Getenv("LANGFUSE_SECRET_KEY")
	if pub == "" || sec == "" {
		return map[string]string{}
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(pub + ":" + sec))
	return map[string]string{
		"Authorization": "Basic " + encoded,
	}
}
