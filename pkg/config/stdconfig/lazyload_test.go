package stdconfig_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/googleoauthconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/meiliconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/oidcconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/paymentsconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/redisconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twilioconfig"
)

// TestLazyLoading_EmptySource_AllOptionalFeaturesDisabled verifies that a
// Source with no optional-feature credentials yields Disabled state for every
// opt-in stdconfig. This is the core contract: operators opt in implicitly by
// supplying fields; the framework stays out of the way when they don't.
func TestLazyLoading_EmptySource_AllOptionalFeaturesDisabled(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(nil))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	reg := config.NewRegistry(src)
	reg.SetStrict(config.StrictLax)

	if _, err := stdconfig.RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if err := reg.Seal(); err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Every optional-feature config must resolve to StateDisabled.
	optionals := map[string]reflect.Type{
		"bichat":      reflect.TypeOf(bichatconfig.Config{}),
		"oidc":        reflect.TypeOf(oidcconfig.Config{}),
		"smtp":        reflect.TypeOf(smtpconfig.Config{}),
		"twilio":      reflect.TypeOf(twilioconfig.Config{}),
		"meili":       reflect.TypeOf(meiliconfig.Config{}),
		"googleoauth": reflect.TypeOf(googleoauthconfig.Config{}),
		"redis":       reflect.TypeOf(redisconfig.Config{}),
		"payments":    reflect.TypeOf(paymentsconfig.Config{}),
	}

	for name, typ := range optionals {
		state, ok := reg.State(typ)
		if !ok {
			t.Errorf("%s: state not registered", name)
			continue
		}
		if state != config.StateDisabled {
			t.Errorf("%s: got state=%s, want disabled", name, state)
		}
	}
}

// TestLazyLoading_PartialConfig_StrictFailsBoot verifies the typo-detection
// property: setting an OPENAI fields without APIKEY trips strict-mode error
// that names the missing canonical field via DisabledReason.
func TestLazyLoading_PartialConfig_StrictFailsBoot(t *testing.T) {
	t.Parallel()

	// Operator set BICHAT_OPENAI_MODEL but forgot APIKEY.
	src, err := config.Build(static.New(map[string]any{
		"bichat.openai.model": "gpt-5",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	reg := config.NewRegistry(src)
	reg.SetStrict(config.StrictYes)

	_, err = config.Register[bichatconfig.Config](reg)
	if err == nil {
		t.Fatal("Register in strict mode should error on partial config")
	}
	if !strings.Contains(err.Error(), "partially configured") {
		t.Errorf("error should name partial: %v", err)
	}
	if !strings.Contains(err.Error(), "BICHAT_OPENAI_APIKEY") {
		t.Errorf("error should carry DisabledReason naming canonical key: %v", err)
	}
}

// TestLazyLoading_PartialConfig_LaxDowngrades verifies lax-mode semantics:
// partial is surfaced as StatePartiallyConfigured at Register but
// downgraded to Disabled by Seal so downstream gates skip cleanly.
func TestLazyLoading_PartialConfig_LaxDowngrades(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"oidc.issuerurl": "https://example.com/oidc",
		// missing oidc.cryptokey
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	reg := config.NewRegistry(src)
	reg.SetStrict(config.StrictLax)

	if _, err := config.Register[oidcconfig.Config](reg); err != nil {
		t.Fatalf("Register (lax): %v", err)
	}

	state, _ := config.StateOf[oidcconfig.Config](reg)
	if state != config.StatePartiallyConfigured {
		t.Errorf("pre-Seal: got %s, want partially_configured", state)
	}

	if err := reg.Seal(); err != nil {
		t.Errorf("Seal (lax): unexpected error %v", err)
	}

	state, _ = config.StateOf[oidcconfig.Config](reg)
	if state != config.StateDisabled {
		t.Errorf("post-Seal: got %s, want disabled (lax downgrade)", state)
	}
}

// TestLazyLoading_ConfiguredFeature_IsActive verifies the positive path:
// supplying all required fields yields StateActive and passes Seal.
func TestLazyLoading_ConfiguredFeature_IsActive(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"bichat.openai.apikey": "sk-test",
		"oidc.issuerurl":       "https://example.com/oidc",
		"oidc.cryptokey":       "test-key",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	reg := config.NewRegistry(src)
	reg.SetStrict(config.StrictYes)

	if _, err := config.Register[bichatconfig.Config](reg); err != nil {
		t.Fatalf("bichat Register: %v", err)
	}
	if _, err := config.Register[oidcconfig.Config](reg); err != nil {
		t.Fatalf("oidc Register: %v", err)
	}

	bichatState, _ := config.StateOf[bichatconfig.Config](reg)
	if bichatState != config.StateActive {
		t.Errorf("bichat: got %s, want active", bichatState)
	}
	oidcState, _ := config.StateOf[oidcconfig.Config](reg)
	if oidcState != config.StateActive {
		t.Errorf("oidc: got %s, want active", oidcState)
	}

	if err := reg.Seal(); err != nil {
		t.Errorf("Seal: %v", err)
	}
}

