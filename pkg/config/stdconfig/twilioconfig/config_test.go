package twilioconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twilioconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func TestConfig_StaticRoundTrip(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"twilio.webhookurl":  "https://example.com/webhook",
		"twilio.accountsid":  "ACtest",
		"twilio.authtoken":   "token123",
		"twilio.phonenumber": "+15550001234",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var cfg twilioconfig.Config
	if err := src.Unmarshal("twilio", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.AccountSID != "ACtest" {
		t.Errorf("AccountSID: want ACtest, got %s", cfg.AccountSID)
	}
	if cfg.AuthToken != "token123" {
		t.Errorf("AuthToken: want token123, got %s", cfg.AuthToken)
	}
	if cfg.PhoneNumber != "+15550001234" {
		t.Errorf("PhoneNumber: want +15550001234, got %s", cfg.PhoneNumber)
	}
}

func TestIsConfigured(t *testing.T) {
	t.Parallel()

	full := &twilioconfig.Config{AccountSID: "AC1", AuthToken: "tok"}
	if !full.IsConfigured() {
		t.Error("IsConfigured: should be true when both SID and token set")
	}

	noSID := &twilioconfig.Config{AuthToken: "tok"}
	if noSID.IsConfigured() {
		t.Error("IsConfigured: should be false when AccountSID missing")
	}

	noToken := &twilioconfig.Config{AccountSID: "AC1"}
	if noToken.IsConfigured() {
		t.Error("IsConfigured: should be false when AuthToken missing")
	}

	empty := &twilioconfig.Config{}
	if empty.IsConfigured() {
		t.Error("IsConfigured: should be false when both fields empty")
	}
}

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{}
	legacy.Twilio.WebhookURL = "https://example.com/tw"
	legacy.Twilio.AccountSID = "AClegacy"
	legacy.Twilio.AuthToken = "legacytoken"
	legacy.Twilio.PhoneNumber = "+1555000"

	got := twilioconfig.FromLegacy(legacy)
	if got.AccountSID != "AClegacy" {
		t.Errorf("AccountSID mismatch")
	}
	if got.WebhookURL != "https://example.com/tw" {
		t.Errorf("WebhookURL mismatch")
	}
	if !got.IsConfigured() {
		t.Error("FromLegacy result should be IsConfigured")
	}
}
