package paymentsconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/paymentsconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	values := map[string]any{
		"click.url":            "https://my.click.uz",
		"click.merchantid":     int64(111),
		"click.merchantuserid": int64(222),
		"click.serviceid":      int64(333),
		"click.secretkey":      "click-secret",

		"payme.url":        "https://checkout.test.paycom.uz",
		"payme.merchantid": "payme-merchant-id",
		"payme.user":       "Paycom",
		"payme.secretkey":  "payme-secret",

		"octo.shopid":     int32(42),
		"octo.secret":     "octo-secret",
		"octo.secrethash": "octo-hash",
		"octo.notifyurl":  "https://example.com/octo/notify",

		"stripe.secretkey":     "sk_test_123",
		"stripe.signingsecret": "whsec_abc",
	}

	src := buildSource(t, values)

	var cfg paymentsconfig.Config
	if err := src.Unmarshal("", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Click
	if cfg.Click.URL != "https://my.click.uz" {
		t.Errorf("Click.URL: got %q", cfg.Click.URL)
	}
	if cfg.Click.MerchantID != 111 {
		t.Errorf("Click.MerchantID: got %d", cfg.Click.MerchantID)
	}
	if cfg.Click.MerchantUserID != 222 {
		t.Errorf("Click.MerchantUserID: got %d", cfg.Click.MerchantUserID)
	}
	if cfg.Click.ServiceID != 333 {
		t.Errorf("Click.ServiceID: got %d", cfg.Click.ServiceID)
	}
	if cfg.Click.SecretKey != "click-secret" {
		t.Errorf("Click.SecretKey: got %q", cfg.Click.SecretKey)
	}

	// Payme
	if cfg.Payme.URL != "https://checkout.test.paycom.uz" {
		t.Errorf("Payme.URL: got %q", cfg.Payme.URL)
	}
	if cfg.Payme.MerchantID != "payme-merchant-id" {
		t.Errorf("Payme.MerchantID: got %q", cfg.Payme.MerchantID)
	}
	if cfg.Payme.User != "Paycom" {
		t.Errorf("Payme.User: got %q", cfg.Payme.User)
	}
	if cfg.Payme.SecretKey != "payme-secret" {
		t.Errorf("Payme.SecretKey: got %q", cfg.Payme.SecretKey)
	}

	// Octo
	if cfg.Octo.ShopID != 42 {
		t.Errorf("Octo.ShopID: got %d", cfg.Octo.ShopID)
	}
	if cfg.Octo.Secret != "octo-secret" {
		t.Errorf("Octo.Secret: got %q", cfg.Octo.Secret)
	}
	if cfg.Octo.SecretHash != "octo-hash" {
		t.Errorf("Octo.SecretHash: got %q", cfg.Octo.SecretHash)
	}
	if cfg.Octo.NotifyURL != "https://example.com/octo/notify" {
		t.Errorf("Octo.NotifyURL: got %q", cfg.Octo.NotifyURL)
	}

	// Stripe
	if cfg.Stripe.SecretKey != "sk_test_123" {
		t.Errorf("Stripe.SecretKey: got %q", cfg.Stripe.SecretKey)
	}
	if cfg.Stripe.SigningSecret != "whsec_abc" {
		t.Errorf("Stripe.SigningSecret: got %q", cfg.Stripe.SigningSecret)
	}
}

func TestSetDefaults_ClickURL(t *testing.T) {
	t.Parallel()

	// Source with no click.url key — SetDefaults should fill it in.
	src := buildSource(t, map[string]any{
		"click.merchantid": int64(1),
	})

	var cfg paymentsconfig.Config
	if err := src.Unmarshal("", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	cfg.SetDefaults()

	if cfg.Click.URL != "https://my.click.uz" {
		t.Errorf("Click.URL default: got %q, want %q", cfg.Click.URL, "https://my.click.uz")
	}
}

func TestSetDefaults_PaymeURLAndUser(t *testing.T) {
	t.Parallel()

	var cfg paymentsconfig.Config
	cfg.SetDefaults()

	if cfg.Payme.URL != "https://checkout.test.paycom.uz" {
		t.Errorf("Payme.URL default: got %q", cfg.Payme.URL)
	}
	if cfg.Payme.User != "Paycom" {
		t.Errorf("Payme.User default: got %q", cfg.Payme.User)
	}
}

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{
		Click: configuration.ClickOptions{
			URL:            "https://my.click.uz",
			MerchantID:     999,
			MerchantUserID: 888,
			ServiceID:      777,
			SecretKey:      "click-sk",
		},
		Payme: configuration.PaymeOptions{
			URL:        "https://checkout.test.paycom.uz",
			MerchantID: "pm-id",
			User:       "Paycom",
			SecretKey:  "payme-sk",
		},
		Octo: configuration.OctoOptions{
			OctoShopID:     55,
			OctoSecret:     "octo-s",
			OctoSecretHash: "octo-h",
			NotifyURL:      "https://example.com/notify",
		},
		Stripe: configuration.StripeOptions{
			SecretKey:     "sk_live_x",
			SigningSecret: "whsec_y",
		},
	}

	got := paymentsconfig.FromLegacy(legacy)

	// Click
	if got.Click.URL != legacy.Click.URL {
		t.Errorf("Click.URL: got %q, want %q", got.Click.URL, legacy.Click.URL)
	}
	if got.Click.MerchantID != legacy.Click.MerchantID {
		t.Errorf("Click.MerchantID: got %d, want %d", got.Click.MerchantID, legacy.Click.MerchantID)
	}
	if got.Click.MerchantUserID != legacy.Click.MerchantUserID {
		t.Errorf("Click.MerchantUserID: got %d, want %d", got.Click.MerchantUserID, legacy.Click.MerchantUserID)
	}
	if got.Click.ServiceID != legacy.Click.ServiceID {
		t.Errorf("Click.ServiceID: got %d, want %d", got.Click.ServiceID, legacy.Click.ServiceID)
	}
	if got.Click.SecretKey != legacy.Click.SecretKey {
		t.Errorf("Click.SecretKey: got %q, want %q", got.Click.SecretKey, legacy.Click.SecretKey)
	}

	// Payme
	if got.Payme.URL != legacy.Payme.URL {
		t.Errorf("Payme.URL: got %q, want %q", got.Payme.URL, legacy.Payme.URL)
	}
	if got.Payme.MerchantID != legacy.Payme.MerchantID {
		t.Errorf("Payme.MerchantID: got %q, want %q", got.Payme.MerchantID, legacy.Payme.MerchantID)
	}
	if got.Payme.User != legacy.Payme.User {
		t.Errorf("Payme.User: got %q, want %q", got.Payme.User, legacy.Payme.User)
	}
	if got.Payme.SecretKey != legacy.Payme.SecretKey {
		t.Errorf("Payme.SecretKey: got %q, want %q", got.Payme.SecretKey, legacy.Payme.SecretKey)
	}

	// Octo
	if got.Octo.ShopID != legacy.Octo.OctoShopID {
		t.Errorf("Octo.ShopID: got %d, want %d", got.Octo.ShopID, legacy.Octo.OctoShopID)
	}
	if got.Octo.Secret != legacy.Octo.OctoSecret {
		t.Errorf("Octo.Secret: got %q, want %q", got.Octo.Secret, legacy.Octo.OctoSecret)
	}
	if got.Octo.SecretHash != legacy.Octo.OctoSecretHash {
		t.Errorf("Octo.SecretHash: got %q, want %q", got.Octo.SecretHash, legacy.Octo.OctoSecretHash)
	}
	if got.Octo.NotifyURL != legacy.Octo.NotifyURL {
		t.Errorf("Octo.NotifyURL: got %q, want %q", got.Octo.NotifyURL, legacy.Octo.NotifyURL)
	}

	// Stripe
	if got.Stripe.SecretKey != legacy.Stripe.SecretKey {
		t.Errorf("Stripe.SecretKey: got %q, want %q", got.Stripe.SecretKey, legacy.Stripe.SecretKey)
	}
	if got.Stripe.SigningSecret != legacy.Stripe.SigningSecret {
		t.Errorf("Stripe.SigningSecret: got %q, want %q", got.Stripe.SigningSecret, legacy.Stripe.SigningSecret)
	}
}

func TestFromLegacy_DefaultsApplied(t *testing.T) {
	t.Parallel()

	// Empty legacy — FromLegacy should still apply defaults.
	got := paymentsconfig.FromLegacy(&configuration.Configuration{})

	if got.Click.URL != "https://my.click.uz" {
		t.Errorf("Click.URL default via FromLegacy: got %q", got.Click.URL)
	}
	if got.Payme.URL != "https://checkout.test.paycom.uz" {
		t.Errorf("Payme.URL default via FromLegacy: got %q", got.Payme.URL)
	}
	if got.Payme.User != "Paycom" {
		t.Errorf("Payme.User default via FromLegacy: got %q", got.Payme.User)
	}
}
