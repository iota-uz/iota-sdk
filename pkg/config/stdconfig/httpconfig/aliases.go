package httpconfig

// LegacyAliases returns the env-var → koanf-path alias map for httpconfig.
// These map legacy bare env var names to their canonical koanf paths.
// Sub-package aliases (headers, cookies, session, pagination) are managed in
// their respective packages and appended via stdconfig.AllLegacyAliases.
func LegacyAliases() map[string]string {
	return map[string]string{
		"PORT":            "http.port",
		"DOMAIN":          "http.domain",
		"ORIGIN":          "http.origin",
		"ALLOWED_ORIGINS": "http.allowedorigins",
	}
}
