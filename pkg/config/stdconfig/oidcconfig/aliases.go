package oidcconfig

// LegacyAliases returns the env-var → koanf-path alias map for oidcconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"OIDC_ISSUER_URL":             "oidc.issuerurl",
		"OIDC_CRYPTO_KEY":             "oidc.cryptokey",
		"OIDC_ACCESS_TOKEN_LIFETIME":  "oidc.accesstokenlifetime",
		"OIDC_REFRESH_TOKEN_LIFETIME": "oidc.refreshtokenlifetime",
		"OIDC_ID_TOKEN_LIFETIME":      "oidc.idtokenlifetime",
	}
}
