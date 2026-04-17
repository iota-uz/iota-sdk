package paymentsconfig

// LegacyAliases returns the env-var → koanf-path alias map for paymentsconfig.
func LegacyAliases() map[string]string {
	return map[string]string{
		"CLICK_URL":              "payments.click.url",
		"CLICK_MERCHANT_ID":      "payments.click.merchantid",
		"CLICK_MERCHANT_USER_ID": "payments.click.merchantuserid",
		"CLICK_SERVICE_ID":       "payments.click.serviceid",
		"CLICK_SECRET_KEY":       "payments.click.secretkey",
		"PAYME_URL":              "payments.payme.url",
		"PAYME_MERCHANT_ID":      "payments.payme.merchantid",
		"PAYME_USER":             "payments.payme.user",
		"PAYME_SECRET_KEY":       "payments.payme.secretkey",
		"OCTO_SHOP_ID":           "payments.octo.shopid",
		"OCTO_SECRET":            "payments.octo.secret",
		"OCTO_SECRET_HASH":       "payments.octo.secrethash",
		"OCTO_NOTIFY_URL":        "payments.octo.notifyurl",
		"STRIPE_SECRET_KEY":      "payments.stripe.secretkey",
		"STRIPE_SIGNING_SECRET":  "payments.stripe.signingsecret",
	}
}
