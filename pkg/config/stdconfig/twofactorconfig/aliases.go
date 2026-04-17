package twofactorconfig

// LegacyAliases returns the env-var → koanf-path alias map for twofactorconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"ENABLE_2FA":          "twofactor.enabled",
		"TOTP_ISSUER":         "twofactor.totpissuer",
		"TOTP_ENCRYPTION_KEY": "twofactor.encryptionkey",
		"OTP_CODE_LENGTH":     "twofactor.otp.codelength",
		"OTP_TTL_SECONDS":     "twofactor.otp.ttlseconds",
		"OTP_MAX_ATTEMPTS":    "twofactor.otp.maxattempts",
		"OTP_ENABLE_EMAIL":    "twofactor.otp.enableemail",
		"OTP_ENABLE_SMS":      "twofactor.otp.enablesms",
	}
}
