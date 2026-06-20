package stdconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig"
)

func TestRegisterAll_AllNonNil(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	r := config.NewRegistry(src)
	b, err := stdconfig.RegisterAll(r)
	if err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	if b == nil {
		t.Fatal("Bundle is nil")
	}

	checks := []struct {
		name string
		ptr  any
	}{
		{"App", b.App},
		{"Bichat", b.Bichat},
		{"DB", b.DB},
		{"GoogleOAuth", b.GoogleOAuth},
		{"HTTP", b.HTTP},
		{"Meili", b.Meili},
		{"OIDC", b.OIDC},
		{"Payments", b.Payments},
		{"RateLimit", b.RateLimit},
		{"Redis", b.Redis},
		{"SMTP", b.SMTP},
		{"Telemetry", b.Telemetry},
		{"Twilio", b.Twilio},
		{"TwoFactor", b.TwoFactor},
		{"Uploads", b.Uploads},
	}

	for _, c := range checks {
		if c.ptr == nil {
			t.Errorf("Bundle.%s is nil after RegisterAll", c.name)
		}
	}
}
