package telemetryconfig_test

import (
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/telemetryconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"telemetry.loglevel":         "info",
		"telemetry.loki.url":         "http://loki:3100",
		"telemetry.loki.appname":     "myapp",
		"telemetry.loki.logpath":     "./logs/app.log",
		"telemetry.otel.tempourl":    "http://tempo:4317",
		"telemetry.otel.servicename": "myservice",
	})

	var cfg telemetryconfig.Config
	if err := src.Unmarshal("telemetry", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.Loki.URL != "http://loki:3100" {
		t.Errorf("Loki.URL: got %q, want %q", cfg.Loki.URL, "http://loki:3100")
	}
	if cfg.Loki.AppName != "myapp" {
		t.Errorf("Loki.AppName: got %q, want %q", cfg.Loki.AppName, "myapp")
	}
	if cfg.Loki.LogPath != "./logs/app.log" {
		t.Errorf("Loki.LogPath: got %q, want %q", cfg.Loki.LogPath, "./logs/app.log")
	}
	if cfg.OTEL.TempoURL != "http://tempo:4317" {
		t.Errorf("OTEL.TempoURL: got %q, want %q", cfg.OTEL.TempoURL, "http://tempo:4317")
	}
	if cfg.OTEL.ServiceName != "myservice" {
		t.Errorf("OTEL.ServiceName: got %q, want %q", cfg.OTEL.ServiceName, "myservice")
	}
}

func TestLogrusLogLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected logrus.Level
	}{
		{"silent", logrus.PanicLevel},
		{"error", logrus.ErrorLevel},
		{"warn", logrus.WarnLevel},
		{"info", logrus.InfoLevel},
		{"debug", logrus.DebugLevel},
		{"unknown", logrus.ErrorLevel},
		{"", logrus.ErrorLevel},
		{"WARN", logrus.ErrorLevel}, // case-sensitive, defaults to error
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			cfg := telemetryconfig.Config{LogLevel: tc.input}
			if got := cfg.LogrusLogLevel(); got != tc.expected {
				t.Errorf("LogrusLogLevel(%q): got %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestOTELIsConfigured(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cfg      telemetryconfig.OTELConfig
		expected bool
	}{
		{
			name:     "both set",
			cfg:      telemetryconfig.OTELConfig{TempoURL: "http://tempo:4317", ServiceName: "svc"},
			expected: true,
		},
		{
			name:     "missing ServiceName",
			cfg:      telemetryconfig.OTELConfig{TempoURL: "http://tempo:4317"},
			expected: false,
		},
		{
			name:     "missing TempoURL",
			cfg:      telemetryconfig.OTELConfig{ServiceName: "svc"},
			expected: false,
		},
		{
			name:     "both empty",
			cfg:      telemetryconfig.OTELConfig{},
			expected: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.cfg.IsConfigured(); got != tc.expected {
				t.Errorf("IsConfigured: got %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{
		LogLevel: "debug",
		Loki: configuration.LokiOptions{
			URL:     "http://loki.legacy:3100",
			AppName: "legacy-app",
			LogPath: "./legacy/app.log",
		},
		OpenTelemetry: configuration.OpenTelemetryOptions{
			TempoURL:    "http://tempo.legacy:4317",
			ServiceName: "legacy-service",
		},
	}

	got := telemetryconfig.FromLegacy(legacy)

	if got.LogLevel != legacy.LogLevel {
		t.Errorf("LogLevel: got %q, want %q", got.LogLevel, legacy.LogLevel)
	}
	if got.Loki.URL != legacy.Loki.URL {
		t.Errorf("Loki.URL: got %q, want %q", got.Loki.URL, legacy.Loki.URL)
	}
	if got.Loki.AppName != legacy.Loki.AppName {
		t.Errorf("Loki.AppName: got %q, want %q", got.Loki.AppName, legacy.Loki.AppName)
	}
	if got.Loki.LogPath != legacy.Loki.LogPath {
		t.Errorf("Loki.LogPath: got %q, want %q", got.Loki.LogPath, legacy.Loki.LogPath)
	}
	if got.OTEL.TempoURL != legacy.OpenTelemetry.TempoURL {
		t.Errorf("OTEL.TempoURL: got %q, want %q", got.OTEL.TempoURL, legacy.OpenTelemetry.TempoURL)
	}
	if got.OTEL.ServiceName != legacy.OpenTelemetry.ServiceName {
		t.Errorf("OTEL.ServiceName: got %q, want %q", got.OTEL.ServiceName, legacy.OpenTelemetry.ServiceName)
	}
}
