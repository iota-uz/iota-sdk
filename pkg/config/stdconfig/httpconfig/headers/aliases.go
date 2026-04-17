package headers

// LegacyAliases returns the env-var → koanf-path alias map for headers config.
func LegacyAliases() map[string]string {
	return map[string]string{
		"REQUEST_ID_HEADER": "http.headers.requestid",
		"REAL_IP_HEADER":    "http.headers.realip",
	}
}
