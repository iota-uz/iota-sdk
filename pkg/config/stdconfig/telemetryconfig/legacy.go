package telemetryconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// Merges LokiOptions, OpenTelemetryOptions, and the top-level LogLevel field.
// Pure field-for-field mapping — no validation, no derivation.
func FromLegacy(c *configuration.Configuration) Config {
	return Config{
		LogLevel: c.LogLevel,
		Loki: LokiConfig{
			URL:     c.Loki.URL,
			AppName: c.Loki.AppName,
			LogPath: c.Loki.LogPath,
		},
		OTEL: OTELConfig{
			TempoURL:    c.OpenTelemetry.TempoURL,
			ServiceName: c.OpenTelemetry.ServiceName,
		},
	}
}
