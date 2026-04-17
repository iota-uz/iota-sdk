package appconfig

// LegacyAliases returns the env-var → koanf-path alias map for appconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"ENABLE_TEST_ENDPOINTS": "app.enabletestendpoints",
		"TELEGRAM_BOT_TOKEN":    "app.telegrambottoken",
	}
}
