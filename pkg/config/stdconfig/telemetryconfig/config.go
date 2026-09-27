// Package telemetryconfig provides typed configuration for logging and observability.
// It merges the legacy LokiOptions, OpenTelemetryOptions, and the top-level LogLevel field
// into a single cohesive Config.
// Intended to be registered via config.Register[telemetryconfig.Config].
package telemetryconfig

import "github.com/sirupsen/logrus"

// LokiConfig groups Loki log-shipping settings under the "loki" sub-key.
type LokiConfig struct {
	// URL is the Loki push endpoint. Empty means Loki shipping is disabled.
	URL string `koanf:"url"`
	// AppName is the app label sent with every log line. Defaults to "sdk".
	AppName string `koanf:"appname" default:"sdk"`
	// LogPath is the local log file path. Defaults to "./logs/app.log".
	LogPath string `koanf:"logpath" default:"./logs/app.log"`
}

// IsConfigured returns true when the Loki push URL is set. Used by
// composition.IfConfigured to gate Loki hook installation inside an
// already-active telemetry path.
func (l LokiConfig) IsConfigured() bool { return l.URL != "" }

// DisabledReason explains why the Loki hook is off when IsConfigured returns false.
func (l LokiConfig) DisabledReason() string { return "TELEMETRY_LOKI_URL not set" }

// OTELConfig groups OpenTelemetry (Tempo) settings under the "otel" sub-key.
type OTELConfig struct {
	// TempoURL is the OTLP gRPC endpoint for traces. Empty means tracing is disabled.
	TempoURL string `koanf:"tempourl"`
	// ServiceName identifies this service in traces and metrics.
	ServiceName string `koanf:"servicename"`
}

// IsConfigured returns true when both TempoURL and ServiceName are set.
// OpenTelemetry enablement is implicit — no explicit flag needed.
func (o *OTELConfig) IsConfigured() bool {
	return o.TempoURL != "" && o.ServiceName != ""
}

// Config holds all telemetry settings (logging level, Loki, OpenTelemetry).
// Env prefix: "telemetry" (e.g. LOG_LEVEL → telemetry.loglevel).
type Config struct {
	// LogLevel controls the minimum log severity. Defaults to "error".
	// Valid values: "silent", "error", "warn", "info", "debug".
	LogLevel string     `koanf:"loglevel" default:"error"`
	Loki     LokiConfig `koanf:"loki"`
	OTEL     OTELConfig `koanf:"otel"`
}

// LogrusLogLevel converts the LogLevel string to a logrus.Level.
// Unknown values default to ErrorLevel, matching legacy behaviour.
func (c *Config) LogrusLogLevel() logrus.Level {
	switch c.LogLevel {
	case "silent":
		return logrus.PanicLevel
	case "error":
		return logrus.ErrorLevel
	case "warn":
		return logrus.WarnLevel
	case "info":
		return logrus.InfoLevel
	case "debug":
		return logrus.DebugLevel
	default:
		return logrus.ErrorLevel
	}
}

// ConfigPrefix returns the koanf prefix for telemetryconfig ("telemetry").
func (Config) ConfigPrefix() string { return "telemetry" }
