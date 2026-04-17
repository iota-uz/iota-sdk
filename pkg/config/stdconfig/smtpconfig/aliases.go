package smtpconfig

// LegacyAliases returns the env-var → koanf-path alias map for smtpconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"SMTP_HOST":     "smtp.host",
		"SMTP_PORT":     "smtp.port",
		"SMTP_USERNAME": "smtp.username",
		"SMTP_PASSWORD": "smtp.password",
		"SMTP_FROM":     "smtp.from",
	}
}
