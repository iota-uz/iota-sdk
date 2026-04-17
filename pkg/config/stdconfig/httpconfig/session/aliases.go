package session

// LegacyAliases returns the env-var → koanf-path alias map for session config.
func LegacyAliases() map[string]string {
	return map[string]string{
		"SESSION_DURATION": "http.session.duration",
	}
}
