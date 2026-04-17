package httpconfig

// LegacyAliases returns the env-var → koanf-path alias map for httpconfig.
// These map legacy bare env var names (PORT, DOMAIN, etc.) to their canonical
// koanf paths so existing deployments continue to work without renaming.
func LegacyAliases() map[string]string {
	return map[string]string{
		"PORT":                   "http.port",
		"DOMAIN":                 "http.domain",
		"ORIGIN":                 "http.origin",
		"ALLOWED_ORIGINS":        "http.allowedorigins",
		"GO_APP_ENV":             "http.environment",
		"REQUEST_ID_HEADER":      "http.headers.requestid",
		"REAL_IP_HEADER":         "http.headers.realip",
		"SID_COOKIE_KEY":         "http.cookies.sid",
		"OAUTH_STATE_COOKIE_KEY": "http.cookies.oauthstate",
		"SESSION_DURATION":       "http.session.duration",
		"PAGE_SIZE":              "http.pagination.pagesize",
		"MAX_PAGE_SIZE":          "http.pagination.maxpagesize",
	}
}
