package paymentsconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/paymentsconfig"
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

func TestDefaults_ClickURL(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{
		"payments.click.merchantid": int64(1),
	}))
	cfg, err := config.Register[paymentsconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.Click.URL != "https://my.click.uz" {
		t.Errorf("Click.URL default: got %q, want %q", cfg.Click.URL, "https://my.click.uz")
	}
}

func TestDefaults_PaymeURLAndUser(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[paymentsconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.Payme.URL != "https://checkout.test.paycom.uz" {
		t.Errorf("Payme.URL default: got %q", cfg.Payme.URL)
	}
	if cfg.Payme.User != "Paycom" {
		t.Errorf("Payme.User default: got %q", cfg.Payme.User)
	}
}
