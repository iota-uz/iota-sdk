package meiliconfig

// LegacyAliases returns the env-var → koanf-path alias map for meiliconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"MEILI_URL":     "meili.url",
		"MEILI_API_KEY": "meili.apikey",
	}
}
