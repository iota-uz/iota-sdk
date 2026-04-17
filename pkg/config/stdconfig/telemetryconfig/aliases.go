package telemetryconfig

// LegacyAliases returns the env-var → koanf-path alias map for telemetryconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"LOG_LEVEL":         "telemetry.loglevel",
		"LOKI_URL":          "telemetry.loki.url",
		"LOKI_APP_NAME":     "telemetry.loki.appname",
		"LOG_PATH":          "telemetry.loki.logpath",
		"OTEL_TEMPO_URL":    "telemetry.otel.tempourl",
		"OTEL_SERVICE_NAME": "telemetry.otel.servicename",
	}
}
