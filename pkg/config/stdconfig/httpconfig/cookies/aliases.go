package cookies

// LegacyAliases returns the env-var → koanf-path alias map for cookies config.
func LegacyAliases() map[string]string {
	return map[string]string{
		"SID_COOKIE_KEY":         "http.cookies.sid",
		"OAUTH_STATE_COOKIE_KEY": "http.cookies.oauthstate",
	}
}
