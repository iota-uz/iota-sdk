package googleoauthconfig

// LegacyAliases returns the env-var → koanf-path alias map for googleoauthconfig.
// Note: the canonical koanf prefix is "googleoauth" but legacy aliases map to
// the "google.*" path that pre-dates the prefix rename. Existing deployments
// using GOOGLE_REDIRECT_URL etc. continue to work.
func LegacyAliases() map[string]string {
	return map[string]string{
		"GOOGLE_REDIRECT_URL":  "google.redirecturl",
		"GOOGLE_CLIENT_ID":     "google.clientid",
		"GOOGLE_CLIENT_SECRET": "google.clientsecret",
	}
}
