package twilioconfig

// LegacyAliases returns the env-var → koanf-path alias map for twilioconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"TWILIO_WEBHOOK_URL":  "twilio.webhookurl",
		"TWILIO_ACCOUNT_SID":  "twilio.accountsid",
		"TWILIO_AUTH_TOKEN":   "twilio.authtoken",
		"TWILIO_PHONE_NUMBER": "twilio.phonenumber",
	}
}
